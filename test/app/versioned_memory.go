package app

import (
	"slices"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// s3mem v1.2.0 emits historical versions before the current version. Correct
// that test-backend ordering, including cursors, before exercising production
// code which expects the S3 newest-first contract. The fixture already stores
// its whole dataset in RAM; it is not used to certify storage-scale memory use.
type versionedMemory struct{ *s3mem.Backend }

func newVersionedMemory() *versionedMemory { return &versionedMemory{s3mem.New()} }
func versionKey(v gofakes3.VersionItem) string {
	switch x := v.(type) {
	case *gofakes3.Version:
		return x.Key
	case *gofakes3.DeleteMarker:
		return x.Key
	default:
		panic("unexpected memory version item")
	}
}
func (b *versionedMemory) ListBucketVersions(bucket string, prefix *gofakes3.Prefix, page *gofakes3.ListBucketVersionsPage) (*gofakes3.ListBucketVersionsResult, error) {
	if page == nil {
		page = &gofakes3.ListBucketVersionsPage{}
	}
	all, err := b.Backend.ListBucketVersions(bucket, prefix, &gofakes3.ListBucketVersionsPage{})
	if err != nil {
		return nil, err
	}
	for start := 0; start < len(all.Versions); {
		end := start + 1
		for end < len(all.Versions) && versionKey(all.Versions[end]) == versionKey(all.Versions[start]) {
			end++
		}
		slices.Reverse(all.Versions[start:end])
		start = end
	}
	start := 0
	if page.KeyMarker != "" {
		start = len(all.Versions)
		for i, item := range all.Versions {
			key := versionKey(item)
			if key > page.KeyMarker {
				start = i
				break
			}
			if key == page.KeyMarker && page.VersionIDMarker != "" && item.GetVersionID() == page.VersionIDMarker {
				start = i + 1
				break
			}
		}
	}
	end := len(all.Versions)
	if page.MaxKeys > 0 && int64(end-start) > page.MaxKeys {
		end = start + int(page.MaxKeys)
	}
	result := gofakes3.NewListBucketVersionsResult(bucket, prefix, page)
	result.CommonPrefixes = all.CommonPrefixes
	result.Versions = all.Versions[start:end]
	result.IsTruncated = end < len(all.Versions)
	if result.IsTruncated && len(result.Versions) > 0 {
		last := result.Versions[len(result.Versions)-1]
		result.NextKeyMarker = versionKey(last)
		result.NextVersionIDMarker = last.GetVersionID()
	}
	return result, nil
}
