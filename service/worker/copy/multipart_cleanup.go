package copy

import (
	"context"
	"errors"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"
)

// Orphaned multipart uploads: a copy that fails or is cancelled while parts are uploaded leaves an incomplete
// upload on the destination (it keeps using quota until aborted). minio-go aborts with the copy context, which
// is already cancelled on shutdown, task timeout or retry, so the abort fails. The helpers here abort with a
// fresh context (like MinIO's bucket replication) and sweep leftovers of earlier attempts.

const (
	abortTimeout = time.Minute
	abortTries   = 3
	// staleUploadAge: an incomplete upload of the key older than this is considered a leftover of a dead attempt.
	staleUploadAge = 5 * time.Minute
)

var abortRetryDelay = 500 * time.Millisecond

type keyLockedKey struct{}

// WithKeyLocked tells CopyObject that the caller holds the distributed destination-key lock. Only then may the
// service abort incomplete uploads of the key it did not start itself (leftovers of failed attempts): the lock
// guarantees that no other worker is uploading the same key. Without it only the upload of this copy is aborted.
func WithKeyLocked(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyLockedKey{}, true)
}

func keyLocked(ctx context.Context) bool {
	v, _ := ctx.Value(keyLockedKey{}).(bool)
	return v
}

type multipartAPI interface {
	AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error
	ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (minio.ListMultipartUploadsResult, error)
}

// abortUpload aborts an upload with a context that is independent of the copy context: 3 tries, 1 minute each.
// An upload that does not exist any more counts as aborted.
func abortUpload(core multipartAPI, bucket, key, uploadID string) error {
	var err error
	for i := 0; i < abortTries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), abortTimeout)
		err = core.AbortMultipartUpload(ctx, bucket, key, uploadID)
		cancel()
		if err == nil || isNoSuchUpload(err) {
			return nil
		}
		time.Sleep(abortRetryDelay)
	}
	return err
}

func isNoSuchUpload(err error) bool {
	var er minio.ErrorResponse
	return errors.As(err, &er) && er.Code == "NoSuchUpload"
}

// sweepStaleUploads aborts incomplete uploads of exactly this key that were initiated at least olderThan ago
// (0 = all) and that keep() does not claim (an upload a resumable copy intends to reuse). The caller must hold
// the destination-key lock. It returns the number of aborted uploads; listing and abort errors are returned
// after all candidates were tried, and callers treat them as non-fatal.
func sweepStaleUploads(ctx context.Context, core multipartAPI, bucket, key string, olderThan time.Duration, keep func(uploadID string) bool, now time.Time) (int, error) {
	var errs []error
	aborted := 0
	keyMarker, idMarker := "", ""
	for {
		res, err := core.ListMultipartUploads(ctx, bucket, key, keyMarker, idMarker, "", 1000)
		if err != nil {
			errs = append(errs, err)
			break
		}
		for _, u := range res.Uploads {
			if u.Key != key {
				continue // the prefix also matches longer keys
			}
			if keep != nil && keep(u.UploadID) {
				continue
			}
			if olderThan > 0 && !u.Initiated.IsZero() && now.Sub(u.Initiated) < olderThan {
				continue
			}
			if err := abortUpload(core, bucket, key, u.UploadID); err != nil {
				errs = append(errs, err)
				continue
			}
			aborted++
		}
		if !res.IsTruncated {
			break
		}
		keyMarker, idMarker = res.NextKeyMarker, res.NextUploadIDMarker
	}
	return aborted, errors.Join(errs...)
}

// sweepBeforeUpload is run at the start of a multipart copy of a key (destination-key lock held).
func (r *S3CopySvc) sweepBeforeUpload(ctx context.Context, core multipartAPI, bucket, key string, keep func(string) bool) {
	if !keyLocked(ctx) || !orphanSweepEnabled.Load() {
		return
	}
	n, err := sweepStaleUploads(ctx, core, bucket, key, staleUploadAge, keep, time.Now())
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("key", key).Msg("unable to sweep stale multipart uploads")
	}
	if n > 0 {
		zerolog.Ctx(ctx).Info().Int("aborted", n).Str("key", key).Msg("aborted stale multipart uploads of the destination key")
	}
}

// cleanupAfterFailure aborts what a failed multipart copy left behind when the engine could not do it itself
// (legacy engine: minio-go aborts with the cancelled copy context). It needs the destination-key lock.
func (r *S3CopySvc) cleanupAfterFailure(ctx context.Context, core multipartAPI, bucket, key string, keep func(string) bool) {
	if !keyLocked(ctx) || !orphanSweepEnabled.Load() {
		return
	}
	// ctx may be cancelled: list with a fresh one
	lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), abortTimeout)
	defer cancel()
	n, err := sweepStaleUploads(lctx, core, bucket, key, 0, keep, time.Now())
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("key", key).Msg("unable to abort incomplete multipart uploads after a failed copy")
	}
	if n > 0 {
		zerolog.Ctx(ctx).Info().Int("aborted", n).Str("key", key).Msg("aborted incomplete multipart uploads after a failed copy")
	}
}
