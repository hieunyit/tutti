package syncer

import (
	"context"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	minio "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRegression_DeleteHistoricalVersionMustCheckThatVersion(t *testing.T) {
	var deletes atomic.Int32
	var versionHeads atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		if r.Method == "HEAD" {
			if r.URL.Query().Get("versionId") == "old-version" {
				versionHeads.Add(1)
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", "4")
			w.Header().Set("Last-Modified", "Wed, 11 Nov 2026 00:00:00 GMT")
			w.Header().Set("ETag", `"etag"`)
			return
		}
		if r.Method == "DELETE" {
			deletes.Add(1)
			w.WriteHeader(204)
			return
		}
		w.WriteHeader(403)
	}))
	defer backend.Close()
	s, _, _, _, p := setup(t)
	p.Object.Bucket = "bucket"
	p.Object.Version = "old-version"
	p.Deleted = true
	p.SetReplicationID(entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: "u", FromStorage: "main", FromBucket: "bucket", ToStorage: "follower", ToBucket: "bucket"}), 0)

	var destinationDeleted atomic.Bool
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		if r.Method == "DELETE" {
			destinationDeleted.Store(true)
			deletes.Add(1)
			w.WriteHeader(204)
			return
		}
		if r.Method == "HEAD" {
			if destinationDeleted.Load() {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", "4")
			w.Header().Set("ETag", `"old-content"`)
			w.Header().Set("Last-Modified", "Wed, 11 Nov 2026 00:00:00 GMT")
			return
		}
		w.WriteHeader(403)
	}))
	defer dst.Close()
	c := comprehensiveDeleteClient(t, backend.URL)
	destination := comprehensiveDeleteClient(t, dst.URL)
	s.clients = comprehensiveDeleteRegistry{source: c, dest: destination}
	_, err := destination.S3().StatObject(t.Context(), "bucket", p.Object.Name, minio.StatObjectOptions{VersionID: p.Object.Version})
	require.NoError(t, err, "old replica version must exist before the task")
	require.NoError(t, s.Sync(t.Context(), p))
	_, err = destination.S3().StatObject(t.Context(), "bucket", p.Object.Name, minio.StatObjectOptions{VersionID: p.Object.Version})
	if err == nil {
		t.Errorf("Sync returned success but deleted historical version remains readable on replica; source version HEADs=%d destination DELETEs=%d", versionHeads.Load(), deletes.Load())
	}

}

func comprehensiveDeleteClient(t *testing.T, url string) s3client.Client {
	c, err := s3client.NewClient(t.Context(), reviewMetrics{}, s3.StorageAddress{Address: url, DefaultRegion: "us-east-1"}, s3.CredentialsV4{AccessKeyID: "access", SecretAccessKey: "secret"}, "storage", "u")
	require.NoError(t, err)
	return c
}

type comprehensiveDeleteRegistry struct {
	objstore.Clients
	source, dest s3client.Client
}

func (c comprehensiveDeleteRegistry) AsS3(_ context.Context, storage, user string) (s3client.Client, error) {
	if storage == "main" {
		return c.source, nil
	}
	return c.dest, nil
}
