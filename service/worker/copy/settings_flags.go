package copy

import "sync/atomic"

// orphanSweepEnabled switches the orphan multipart cleanup (sweep at the start of a multipart copy, abort after
// a failed legacy copy). On by default; SetOrphanSweep(false) (YAML `copy.orphanUploadCleanup: false`) rolls it back.
var orphanSweepEnabled atomic.Bool

func init() { orphanSweepEnabled.Store(true) }

// SetOrphanSweep enables or disables the orphan multipart upload cleanup.
func SetOrphanSweep(on bool) { orphanSweepEnabled.Store(on) }

// recordSourceETag makes every copy write the source ETag as user metadata on the destination object and lets the
// skip check accept a matching marker when the ETags differ. On by default (YAML `recordSourceETag: false` = off).
var recordSourceETag atomic.Bool

// trustSameSizeMultipart (YAML `trustSameSizeMultipart`, default off) skips an object whose source and destination
// ETags are both multipart ETags (different part layout) when sizes are equal and the destination is not older than
// the source. A heuristic: it cannot prove equal content.
var trustSameSizeMultipart atomic.Bool

func init() { recordSourceETag.Store(true) }

// SetRecordSourceETag enables or disables the source ETag marker.
func SetRecordSourceETag(on bool) { recordSourceETag.Store(on) }

// SetTrustSameSizeMultipart enables or disables the same-size multipart heuristic.
func SetTrustSameSizeMultipart(on bool) { trustSameSizeMultipart.Store(on) }
