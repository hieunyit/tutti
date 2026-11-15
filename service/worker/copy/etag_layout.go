package copy

import (
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"

	"github.com/hieunyit/tutti/pkg/s3"
)

const (
	minPartSize         = 5 * mib
	maxInferredPartSize = 128 * mib // larger parts would blow the per-copy buffer budget: fall back
	maxPartCount        = 10000
)

// preferredPartMiB lists common uploader part sizes (MiB) in the order they win when several whole-MiB part
// sizes produce the same part count: 15 (s3cmd), 16 (minio-go, mc, Tutti), 8 (aws cli, boto3), 5 (rclone, min),
// then 10, 20, 25, 32, 50, 64, 100, 128.
var preferredPartMiB = []int64{15, 16, 8, 5, 10, 20, 25, 32, 50, 64, 100, 128}

// multipartETagParts returns N of a multipart ETag "<32 hex>-N" (quotes and a W/ prefix are ignored).
func multipartETagParts(etag string) (int, bool) {
	etag = strings.TrimPrefix(strings.TrimSpace(etag), "W/")
	etag = strings.Trim(etag, `"`)
	i := strings.IndexByte(etag, '-')
	if i != 32 {
		return 0, false
	}
	for _, c := range etag[:32] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return 0, false
		}
	}
	n, err := strconv.Atoi(etag[33:])
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// trimETag strips quotes from an ETag.
func trimETag(etag string) string { return strings.Trim(strings.TrimSpace(etag), `"`) }

// inferPartSize returns the part size p with ceil(size/p) == n, i.e. p in [ceil(size/n), (size-1)/(n-1)], that
// reproduces the multipart ETag of an object of the given size with n parts.
//
// Rule: only whole-MiB sizes are considered (then whole-MB sizes, 1e6). Among several candidates the first of
// preferredPartMiB wins, otherwise the smallest candidate. Nothing is inferred (ok=false) for n < 2, n > 10000,
// size <= 0, a range without a candidate of at least 5 MiB, or a candidate above 128 MiB. A wrong guess is
// harmless: the destination ETag then differs and the source-ETag marker decides.
func inferPartSize(size int64, n int) (int64, bool) {
	if size <= 0 || n < 2 || n > maxPartCount {
		return 0, false
	}
	lo := (size + int64(n) - 1) / int64(n)
	hi := (size - 1) / int64(n-1)
	lo = max(lo, minPartSize)
	hi = min(hi, maxInferredPartSize)
	if lo > hi {
		return 0, false
	}
	for _, m := range preferredPartMiB {
		if p := m * mib; p >= lo && p <= hi {
			return p, true
		}
	}
	if p := (lo + mib - 1) / mib * mib; p <= hi {
		return p, true
	}
	const mb = 1000 * 1000
	if p := (lo + mb - 1) / mb * mb; p <= hi {
		return p, true
	}
	return 0, false
}

// etagPreservingPartSize is the part size for uploading an object so that its destination ETag equals the source
// ETag (a multipart source ETag only).
func etagPreservingPartSize(srcETag string, size int64) (int64, bool) {
	n, ok := multipartETagParts(srcETag)
	if !ok {
		return 0, false
	}
	return inferPartSize(size, n)
}

// userMetaWithoutMarker is the user metadata without Tutti's internal source-ETag key.
func userMetaWithoutMarker(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if strings.EqualFold(k, s3.TuttiSrcETagMeta) || strings.EqualFold(k, "x-amz-meta-"+s3.TuttiSrcETagMeta) {
			continue
		}
		out[k] = v
	}
	return out
}

// sourceETagMarker returns the recorded source ETag of a destination object.
func sourceETagMarker(info minio.ObjectInfo) string {
	for k, v := range info.UserMetadata {
		if strings.EqualFold(k, s3.TuttiSrcETagMeta) {
			return trimETag(v)
		}
	}
	return ""
}

// alreadyUpToDate reports whether the destination object needs no copy: equal size and metadata (the internal
// marker excluded), and either equal ETags, or the recorded source ETag equals the current source ETag, or (only
// with trustSameSizeMultipart) both are multipart ETags and the destination is not older than the source.
func alreadyUpToDate(from, to minio.ObjectInfo, checkTags bool) bool {
	if from.Size != to.Size || !sameMetadata(from, to, checkTags) {
		return false
	}
	if from.ETag == to.ETag {
		return true
	}
	if recordSourceETag.Load() {
		if m := sourceETagMarker(to); m != "" && m == trimETag(from.ETag) {
			return true
		}
	}
	if trustSameSizeMultipart.Load() {
		_, fm := multipartETagParts(from.ETag)
		_, tm := multipartETagParts(to.ETag)
		if fm && tm && !to.LastModified.Before(from.LastModified) {
			return true
		}
	}
	return false
}
