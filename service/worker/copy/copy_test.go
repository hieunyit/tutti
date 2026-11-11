package copy

import (
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

func TestSameMetadata(t *testing.T) {
	r := require.New(t)

	baseA := minio.ObjectInfo{
		ETag:         "etag-123",
		Size:         1024,
		ContentType:  "application/json",
		UserMetadata: map[string]string{"env": "prod", "author": "alice"},
		UserTags:     map[string]string{"dept": "eng"},
	}

	// 1. Identical metadata matches
	baseB := minio.ObjectInfo{
		ETag:         "etag-123",
		Size:         1024,
		ContentType:  "application/json",
		UserMetadata: map[string]string{"env": "prod", "author": "alice"},
		UserTags:     map[string]string{"dept": "eng"},
	}
	r.True(sameMetadata(baseA, baseB, true))
	r.True(sameMetadata(baseA, baseB, false))

	// 2. Different Content-Type must not match
	diffContentType := baseB
	diffContentType.ContentType = "text/plain"
	r.False(sameMetadata(baseA, diffContentType, false), "different Content-Type must fail match")

	// 3. Different UserMetadata must not match
	diffMeta := baseB
	diffMeta.UserMetadata = map[string]string{"env": "staging", "author": "alice"}
	r.False(sameMetadata(baseA, diffMeta, false), "different UserMetadata must fail match")

	// 4. Missing UserMetadata key must not match
	missingMeta := baseB
	missingMeta.UserMetadata = map[string]string{"env": "prod"}
	r.False(sameMetadata(baseA, missingMeta, false), "missing UserMetadata key must fail match")

	// 5. Different UserTags must not match if checkTags is true
	diffTags := baseB
	diffTags.UserTags = map[string]string{"dept": "finance"}
	r.False(sameMetadata(baseA, diffTags, true), "different UserTags must fail match when checkTags=true")
	r.True(sameMetadata(baseA, diffTags, false), "different UserTags ignored when checkTags=false")
}
