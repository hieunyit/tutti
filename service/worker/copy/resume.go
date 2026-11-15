package copy

import (
	"context"
	"crypto/sha1" //nolint:gosec // key derivation only
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"
)

// Resumable multipart uploads (pipeline engine, OFF by default, see docs/tutti/configuration.md).
//
// While a large object is uploaded the worker records {upload id, source ETag/size/mtime, part size, completed
// parts} in Redis. When the copy is retried (worker restart, task timeout, transient error) and the source is
// unchanged, the destination's ListParts is asked which parts really exist (the source of truth), the source
// is read with a ranged GET from the first missing part and only the missing parts are uploaded.
// Anything unexpected (source changed, different part size, unknown upload, listing errors, duplicate or
// mismatching parts) aborts the old upload and restarts from zero.

const (
	// ResumeRecordTTL is the Redis expiry of a record; every part write refreshes it.
	ResumeRecordTTL = 7 * 24 * time.Hour
	// resumeWriteInterval throttles Redis writes while parts complete.
	resumeWriteInterval = 2 * time.Second
	listPartsPage       = 1000
)

// ResumeStore is the persistence of resume records (implemented on Redis in pkg/store). Get returns (nil, nil)
// for a missing key.
type ResumeStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type resumeCtxKey struct{}

type resumeScope struct {
	store ResumeStore
	key   string
}

// WithResume enables resumable multipart for the copy in ctx: the record lives under key in store. Callers
// must only do this while holding the destination-key lock and when worker.resumeMultipart is on.
func WithResume(ctx context.Context, store ResumeStore, key string) context.Context {
	return context.WithValue(ctx, resumeCtxKey{}, &resumeScope{store: store, key: key})
}

func resumeFrom(ctx context.Context) *resumeScope {
	s, _ := ctx.Value(resumeCtxKey{}).(*resumeScope)
	return s
}

// ResumeKey builds the Redis key p:mpu:{id:gen}:<sha1(key NUL version)>. The hash tag keeps all records of a
// replication generation in one cluster slot and makes them easy to find.
func ResumeKey(replicationID string, generation int64, key, version string) string {
	sum := sha1.Sum([]byte(key + "\x00" + version)) //nolint:gosec
	return fmt.Sprintf("p:mpu:{%s:%d}:%s", replicationID, generation, hex.EncodeToString(sum[:]))
}

type resumePart struct {
	N    int    `json:"n"`
	ETag string `json:"e"`
	Size int64  `json:"s"`
}

// resumeRecord is the persisted state.
type resumeRecord struct {
	UploadID     string       `json:"uploadId"`
	Bucket       string       `json:"bucket"`
	Key          string       `json:"key"`
	SourceETag   string       `json:"srcEtag"`
	Size         int64        `json:"size"`
	LastModified int64        `json:"lastModifiedNs"`
	PartSize     int64        `json:"partSize"`
	Parts        []resumePart `json:"parts"`
}

// resumeRecorder persists the record, throttled.
type resumeRecorder struct {
	scope *resumeScope
	mu    sync.Mutex
	rec   resumeRecord
	dirty bool
	last  time.Time
	now   func() time.Time
}

func (r *resumeRecorder) write(ctx context.Context) {
	b, err := json.Marshal(r.rec)
	if err != nil {
		return
	}
	// the context of the copy may already be cancelled: the record must survive exactly then
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := r.scope.store.Set(wctx, r.scope.key, b, ResumeRecordTTL); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to persist multipart resume record")
		return
	}
	r.dirty = false
	r.last = r.now()
}

func (r *resumeRecorder) init(ctx context.Context, uploadID string, partSize int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.UploadID, r.rec.PartSize = uploadID, partSize
	r.write(ctx)
}

func (r *resumeRecorder) part(ctx context.Context, p minio.CompletePart, size int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.Parts = append(r.rec.Parts, resumePart{N: p.PartNumber, ETag: p.ETag, Size: size})
	r.dirty = true
	if r.now().Sub(r.last) >= resumeWriteInterval {
		r.write(ctx)
	}
}

func (r *resumeRecorder) flush(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dirty {
		r.write(ctx)
	}
}

func (r *resumeRecorder) delete(ctx context.Context) {
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := r.scope.store.Delete(dctx, r.scope.key); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to delete multipart resume record")
	}
}

// partsAPI is the part of minio.Core used to inspect an existing upload.
type partsAPI interface {
	multipartAPI
	ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker, maxParts int) (minio.ListObjectPartsResult, error)
}

// resumePlan is the outcome of looking at a previous attempt.
type resumePlan struct {
	state *resumeState // nil: start from zero
	rec   *resumeRecorder
	// keepID is the upload the orphan sweep must not touch
	keepID string
}

// planResume loads the record of a previous attempt and checks it against the source (srcETag, size, mtime) and
// the destination (ListParts). Whatever it cannot trust is aborted (fresh context) and the copy restarts at 0.
func planResume(ctx context.Context, rs *resumeScope, core partsAPI, bucket, key, srcETag string, size int64, mtime time.Time, partSize int64) *resumePlan {
	rec := &resumeRecorder{scope: rs, now: time.Now, rec: resumeRecord{Bucket: bucket, Key: key, SourceETag: srcETag, Size: size, LastModified: mtime.UnixNano(), PartSize: partSize}}
	plan := &resumePlan{rec: rec}
	logger := zerolog.Ctx(ctx)

	raw, err := rs.store.Get(ctx, rs.key)
	if err != nil {
		logger.Warn().Err(err).Msg("multipart resume: unable to read the record, starting from zero")
		return plan
	}
	if raw == nil {
		return plan
	}
	var old resumeRecord
	reject := func(reason string) *resumePlan {
		logger.Info().Str("reason", reason).Str("upload_id", old.UploadID).Msg("multipart resume: discarding the previous upload")
		if old.UploadID != "" {
			if aerr := abortUpload(core, bucket, key, old.UploadID); aerr != nil {
				logger.Warn().Err(aerr).Msg("multipart resume: unable to abort the previous upload")
			}
		}
		rec.delete(ctx)
		return plan
	}
	if err := json.Unmarshal(raw, &old); err != nil || old.UploadID == "" {
		return reject("unreadable record")
	}
	plan.keepID = old.UploadID
	switch {
	case old.Bucket != bucket || old.Key != key:
		return reject("different destination")
	case old.SourceETag != srcETag || old.Size != size || old.LastModified != mtime.UnixNano():
		return reject("source changed")
	case old.PartSize != partSize:
		return reject("part size changed")
	}

	listed, err := listAllParts(ctx, core, bucket, key, old.UploadID)
	if err != nil {
		if isNoSuchUpload(err) {
			logger.Info().Str("upload_id", old.UploadID).Msg("multipart resume: the destination lost the upload, starting from zero")
			plan.keepID = ""
			rec.delete(ctx)
			return plan
		}
		return reject("unable to list parts: " + err.Error())
	}
	byNum := map[int]minio.ObjectPart{}
	for _, p := range listed {
		if _, dup := byNum[p.PartNumber]; dup {
			return reject("duplicate part numbers in the destination listing")
		}
		byNum[p.PartNumber] = p
	}
	recorded := map[int]resumePart{}
	for _, p := range old.Parts {
		recorded[p.N] = p
	}
	nParts := int((size + partSize - 1) / partSize)
	var prefix []minio.CompletePart
	for n := 1; n <= nParts; n++ {
		p, ok := byNum[n]
		if !ok {
			break // parts the destination does not list are uploaded again
		}
		want := min(partSize, size-int64(n-1)*partSize)
		if p.Size != want {
			return reject(fmt.Sprintf("part %d has size %d, expected %d", n, p.Size, want))
		}
		etag := strings.Trim(p.ETag, `"`) // listings quote the ETag, PutObjectPart results do not
		if r, ok := recorded[n]; ok && strings.Trim(r.ETag, `"`) != etag {
			return reject(fmt.Sprintf("part %d ETag differs from the record", n))
		}
		prefix = append(prefix, minio.CompletePart{PartNumber: n, ETag: etag})
	}
	for _, p := range prefix {
		rec.rec.Parts = append(rec.rec.Parts, resumePart{N: p.PartNumber, ETag: p.ETag, Size: min(partSize, size-int64(p.PartNumber-1)*partSize)})
	}
	rec.rec.UploadID = old.UploadID
	plan.state = &resumeState{uploadID: old.UploadID, startPart: len(prefix) + 1, parts: prefix}
	logger.Info().Str("upload_id", old.UploadID).Int("parts_done", len(prefix)).Int("parts_total", nParts).Msg("multipart resume: continuing the previous upload")
	return plan
}

func listAllParts(ctx context.Context, core partsAPI, bucket, key, uploadID string) ([]minio.ObjectPart, error) {
	var all []minio.ObjectPart
	marker := 0
	for {
		res, err := core.ListObjectParts(ctx, bucket, key, uploadID, marker, listPartsPage)
		if err != nil {
			return nil, err
		}
		all = append(all, res.ObjectParts...)
		if !res.IsTruncated {
			return all, nil
		}
		marker = res.NextPartNumberMarker
		if marker == 0 {
			return nil, errors.New("truncated parts listing without a next marker")
		}
	}
}

// keepUploadOnError decides whether a failed upload may be continued by the next attempt: transient trouble
// (cancellation, network, 5xx, throttling) yes; the source disappearing or changing, or client errors, no.
func keepUploadOnError(err error) bool {
	if err == nil {
		return false
	}
	var nf *sourceNotFoundError
	if errors.As(err, &nf) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var er minio.ErrorResponse
	if errors.As(err, &er) {
		if er.Code == "NoSuchUpload" || er.Code == "PreconditionFailed" || er.StatusCode == http.StatusPreconditionFailed {
			return false
		}
		return er.StatusCode >= 500 || er.StatusCode == http.StatusTooManyRequests || er.StatusCode == http.StatusRequestTimeout
	}
	return true // network and I/O errors
}
