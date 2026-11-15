package copy

import "sync/atomic"

// orphanSweepEnabled switches the orphan multipart cleanup (sweep at the start of a multipart copy, abort after
// a failed legacy copy). On by default; SetOrphanSweep(false) (YAML `copy.orphanUploadCleanup: false`) rolls it back.
var orphanSweepEnabled atomic.Bool

func init() { orphanSweepEnabled.Store(true) }

// SetOrphanSweep enables or disables the orphan multipart upload cleanup.
func SetOrphanSweep(on bool) { orphanSweepEnabled.Store(on) }
