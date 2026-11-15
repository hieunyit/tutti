package copy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/systemsettings"
)

// The pipeline copy engine: one sequential source stream is cut into parts, each part is read into a pooled
// buffer and uploaded by one of K uploader goroutines, so reading part N+1 overlaps uploading part N and the
// source GET is never stalled behind an upload (legacy PutObject reads a part, then uploads it, then reads the
// next). Memory per object is bounded by (K+1) part buffers taken from one process-wide size-class pool.

const (
	// targetMaxParts keeps the part count far below the 10000 parts limit of S3.
	targetMaxParts = 9000
	mib            = 1 << 20
	// maxSourceReopens bounds consecutive failed attempts to continue a broken source stream.
	maxSourceReopens = 3
	// minStreamProgress: a broken stream that delivered at least this much resets the reopen budget.
	minStreamProgress = 256 << 10
)

// pipelinePartSize is max(configured part size, ceil(size/9000) rounded up to a MiB).
func pipelinePartSize(size int64, configured uint64) int64 {
	ps := int64(configured)
	if ps <= 0 {
		ps = int64(systemsettings.DefaultMultipartPartSizeMiB) * mib
	}
	need := (size + targetMaxParts - 1) / targetMaxParts
	need = (need + mib - 1) / mib * mib
	return max(ps, need)
}

// legacyPartSize returns the part size to pass to minio-go so that the 10000 parts limit cannot be hit:
// the configured one, raised when size/9000 requires it.
func legacyPartSize(size int64, configured uint64) uint64 {
	need := (size + targetMaxParts - 1) / targetMaxParts
	need = (need + mib - 1) / mib * mib
	if configured == 0 && need <= int64(systemsettings.DefaultMultipartPartSizeMiB)*mib {
		return 0 // minio-go's own policy is enough
	}
	return uint64(max(int64(configured), need))
}

// pipelineBuffer is the memory reserved against the buffer budget for one pipeline copy.
func pipelineBuffer(size, partSize int64, threads int) int64 {
	return min(int64(threads+1)*partSize, size)
}

// ---- buffer pool ----

type partPool struct {
	mu          sync.Mutex
	classes     map[int64]*sync.Pool
	outstanding atomic.Int64 // bytes of buffers handed out and not yet returned
	peak        atomic.Int64
	allocated   atomic.Int64 // bytes of buffers ever allocated (not reused from the pool)
}

var partBuffers = &partPool{classes: map[int64]*sync.Pool{}}

func (p *partPool) class(capacity int64) *sync.Pool {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.classes[capacity]
	if c == nil {
		c = &sync.Pool{}
		p.classes[capacity] = c
	}
	return c
}

// get returns a buffer of length n taken from the size class of capacity (>= n).
func (p *partPool) get(capacity, n int64) []byte {
	var b []byte
	if v, ok := p.class(capacity).Get().(*[]byte); ok {
		b = *v
	} else {
		b = make([]byte, capacity)
		p.allocated.Add(capacity)
	}
	cur := p.outstanding.Add(capacity)
	for {
		pk := p.peak.Load()
		if cur <= pk || p.peak.CompareAndSwap(pk, cur) {
			break
		}
	}
	return b[:n]
}

func (p *partPool) put(b []byte) {
	capacity := int64(cap(b))
	b = b[:capacity]
	p.outstanding.Add(-capacity)
	p.class(capacity).Put(&b)
}

// ---- source stream ----

// sourceStream reads the source object sequentially from a start offset and transparently continues a broken
// stream with a ranged GET at the current offset (If-Match keeps the revision).
type sourceStream struct {
	ctx     context.Context
	open    func(ctx context.Context, start int64) (io.ReadCloser, error)
	size    int64
	pos     int64
	rc      io.ReadCloser
	opened  int64 // bytes read from the current stream
	reopens int   // consecutive streams that broke before making real progress
	err     error // last non-EOF error, to tell a missing source from a destination failure
}

func (s *sourceStream) Read(p []byte) (int, error) {
	for {
		if s.pos >= s.size {
			return 0, io.EOF
		}
		if s.rc == nil {
			rc, err := s.open(s.ctx, s.pos)
			if err != nil {
				s.err = err
				return 0, err
			}
			s.rc = rc
			s.opened = 0
		}
		n, err := s.rc.Read(p)
		s.pos += int64(n)
		s.opened += int64(n)
		if err == nil {
			return n, nil
		}
		if errors.Is(err, io.EOF) && s.pos >= s.size {
			return n, io.EOF
		}
		// the connection broke (or the body ended early): continue at the current offset
		_ = s.rc.Close()
		s.rc = nil
		if s.opened >= minStreamProgress {
			s.reopens = 0 // the stream made real progress before it broke
		}
		if s.ctx.Err() != nil {
			s.err = s.ctx.Err()
			return n, s.ctx.Err()
		}
		if s.reopens >= maxSourceReopens {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			s.err = err
			return n, err
		}
		s.reopens++
		if n > 0 {
			return n, nil
		}
	}
}

func (s *sourceStream) Close() error {
	if s.rc != nil {
		err := s.rc.Close()
		s.rc = nil
		return err
	}
	return nil
}

// ---- engine ----

type pipelineParams struct {
	from, to                *s3client.S3
	fromBucket, fromName    string
	fromVersion, sourceETag string
	toBucket, toName        string
	size                    int64
	partSize                int64
	threads                 int
	putOpts                 minio.PutObjectOptions // metadata, tags, internal query params
	limiter                 *rate.Limiter
	progress                func(done int64)
	disableSha256           bool
	// resume (see resume.go): continue an existing upload; parts[0..startPart-2] are already uploaded
	resume *resumeState
	// rec persists the upload state for resume (nil: resume off)
	rec *resumeRecorder
}

// resumeState describes an existing upload to continue (see resume.go).
type resumeState struct {
	uploadID  string
	startPart int                  // first part still to upload (1-based)
	parts     []minio.CompletePart // contiguous completed prefix 1..startPart-1
}

type partJob struct {
	num int
	buf []byte
}

// pipelineCopy uploads the object. Any error aborts the upload with a fresh context (see abortUpload),
// except that with resume enabled a failed upload is left for the next attempt (see resume.go).
func (r *S3CopySvc) pipelineCopy(ctx context.Context, p pipelineParams) (info minio.UploadInfo, err error) {
	core := p.to.Core()
	nParts := int((p.size + p.partSize - 1) / p.partSize)
	startPart := 1
	var prior []minio.CompletePart
	uploadID := ""
	if p.resume != nil {
		uploadID, startPart, prior = p.resume.uploadID, p.resume.startPart, p.resume.parts
	}
	defer func() { p.to.CountPut(ctx, p.toBucket, p.size) }()

	if uploadID == "" {
		uploadID, err = core.NewMultipartUpload(ctx, p.toBucket, p.toName, p.putOpts)
		if err != nil {
			return info, fmt.Errorf("unable to start multipart upload: %w", err)
		}
		if p.rec != nil {
			p.rec.init(ctx, uploadID, p.partSize)
		}
	}

	var doneBytes atomic.Int64
	if startPart > 1 {
		doneBytes.Store(min(int64(startPart-1)*p.partSize, p.size))
	}
	report := func() {
		if p.progress != nil {
			p.progress(min(doneBytes.Load(), p.size))
		}
	}
	report()

	workCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	startOffset := int64(startPart-1) * p.partSize
	src := &sourceStream{ctx: workCtx, size: p.size, pos: startOffset, open: func(c context.Context, start int64) (io.ReadCloser, error) {
		opts := minio.GetObjectOptions{VersionID: p.fromVersion}
		if start > 0 {
			if err := opts.SetRange(start, p.size-1); err != nil {
				return nil, err
			}
		}
		if p.sourceETag != "" {
			if err := opts.SetMatchETag(p.sourceETag); err != nil {
				return nil, err
			}
		}
		p.from.CountGetRequest(c)
		rc, _, _, err := p.from.Core().GetObject(c, p.fromBucket, p.fromName, opts)
		return rc, err
	}}
	defer src.Close()
	reader := systemsettings.NewThrottledReader(workCtx, src, p.limiter)

	var (
		mu       sync.Mutex
		firstErr error
		parts    = append([]minio.CompletePart(nil), prior...)
	)
	setErr := func(e error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = e
			cancel(e)
		}
		mu.Unlock()
	}

	tokens := make(chan struct{}, p.threads+1) // at most threads+1 buffers exist: threads uploading, one filling
	jobs := make(chan partJob)
	partOpts := minio.PutObjectPartOptions{DisableContentSha256: p.disableSha256}
	var wg sync.WaitGroup
	for i := 0; i < p.threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				n := len(job.buf)
				if workCtx.Err() == nil {
					part, perr := core.PutObjectPart(workCtx, p.toBucket, p.toName, uploadID, job.num, bytes.NewReader(job.buf), int64(n), partOpts)
					if perr != nil {
						setErr(fmt.Errorf("upload part %d: %w", job.num, perr))
					} else {
						cp := minio.CompletePart{PartNumber: job.num, ETag: part.ETag}
						mu.Lock()
						parts = append(parts, cp)
						mu.Unlock()
						doneBytes.Add(int64(n))
						report()
						if p.rec != nil {
							p.rec.part(workCtx, cp, int64(n))
						}
					}
				}
				partBuffers.put(job.buf)
				<-tokens
			}
		}()
	}

	// reader: fill buffers sequentially
produce:
	for num := startPart; num <= nParts; num++ {
		select {
		case tokens <- struct{}{}:
		case <-workCtx.Done():
			break produce
		}
		n := min(p.partSize, p.size-int64(num-1)*p.partSize)
		buf := partBuffers.get(p.partSize, n)
		if _, rerr := io.ReadFull(reader, buf); rerr != nil {
			partBuffers.put(buf)
			<-tokens
			if errors.Is(rerr, io.EOF) {
				rerr = io.ErrUnexpectedEOF
			}
			setErr(fmt.Errorf("read source part %d: %w", num, rerr))
			break
		}
		select {
		case jobs <- partJob{num: num, buf: buf}:
		case <-workCtx.Done():
			partBuffers.put(buf)
			<-tokens
			break produce
		}
	}
	close(jobs)
	wg.Wait()

	mu.Lock()
	err = firstErr
	mu.Unlock()
	if err == nil {
		if cerr := ctx.Err(); cerr != nil {
			err = cerr
		}
	}
	if err == nil {
		sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
		if len(parts) != nParts {
			err = fmt.Errorf("pipeline copy: %d of %d parts uploaded", len(parts), nParts)
		}
	}
	if err == nil {
		info, err = core.CompleteMultipartUpload(ctx, p.toBucket, p.toName, uploadID, parts, p.putOpts)
		if err != nil {
			err = fmt.Errorf("complete multipart upload: %w", err)
		} else if info.Size > 0 && info.Size != p.size {
			err = fmt.Errorf("destination reports %d bytes, source has %d", info.Size, p.size)
		}
	}
	if err != nil {
		err = r.mapSourceErr(src, err)
		if p.rec != nil && keepUploadOnError(err) {
			// the next attempt continues this upload: persist the final state and leave it on the destination
			p.rec.flush(ctx)
			return info, err
		}
		if aerr := abortUpload(core, p.toBucket, p.toName, uploadID); aerr != nil {
			zerolog.Ctx(ctx).Warn().Err(aerr).Str("upload_id", uploadID).Msg("unable to abort multipart upload")
		}
		if p.rec != nil {
			p.rec.delete(ctx)
		}
		return info, err
	}
	if p.rec != nil {
		p.rec.delete(ctx)
	}
	return info, nil
}

// mapSourceErr leaves the error unchanged; callers use src.err via sourceErr.
func (r *S3CopySvc) mapSourceErr(src *sourceStream, err error) error {
	if src.err != nil && isNotFound(src.err) {
		return &sourceNotFoundError{err: err}
	}
	return err
}

type sourceNotFoundError struct{ err error }

func (e *sourceNotFoundError) Error() string { return e.err.Error() }
func (e *sourceNotFoundError) Unwrap() error { return e.err }
