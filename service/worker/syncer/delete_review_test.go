package syncer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

type reviewMetrics struct{}

func (reviewMetrics) Count(xctx.Flow, string, string)         {}
func (reviewMetrics) Upload(xctx.Flow, string, string, int)   {}
func (reviewMetrics) Download(xctx.Flow, string, string, int) {}

type reviewClients struct {
	objstore.Clients
	client s3client.Client
}

func (c reviewClients) AsS3(context.Context, string, string) (s3client.Client, error) {
	return c.client, nil
}

func TestReviewDeletePreservesReplicaWhenSourceAccessDenied(t *testing.T) {
	reviewDeleteWithSourceStatus(t, http.StatusForbidden, false)
}

func TestReviewDeleteRemovesReplicaOnlyWhenSourceMissing(t *testing.T) {
	reviewDeleteWithSourceStatus(t, http.StatusNotFound, true)
}

func TestReviewDeletePreservesReplicaWhenSourceUnavailable(t *testing.T) {
	reviewDeleteWithSourceStatus(t, http.StatusServiceUnavailable, false)
}

func reviewDeleteWithSourceStatus(t *testing.T, sourceStatus int, shouldDelete bool) {
	t.Helper()
	var deletes atomic.Int32
	var sourceHeads atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			sourceHeads.Add(1)
		}
		if r.Method == http.MethodDelete {
			deletes.Add(1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		// Startup's bucket-versioning probe must establish a reachable backend.
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
			return
		}
		w.WriteHeader(sourceStatus)
	}))
	defer backend.Close()
	client, err := s3client.NewClient(t.Context(), reviewMetrics{}, s3.StorageAddress{Address: backend.URL, DefaultRegion: "us-east-1"}, s3.CredentialsV4{AccessKeyID: "access", SecretAccessKey: "secret"}, "storage", "u")
	require.NoError(t, err)
	// Bound MinIO's retries so the 503 regression case stays fast and deterministic.
	raw, err := mclient.New(strings.TrimPrefix(backend.URL, "http://"), &mclient.Options{Region: "us-east-1", MaxRetries: 1})
	require.NoError(t, err)
	client.S3().Client = raw
	s, _, _, _, p := setup(t)
	p.Object.Bucket = "bucket"
	p.SetReplicationID(entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: "u", FromStorage: "main", FromBucket: "bucket", ToStorage: "follower", ToBucket: "bucket"}), 0)
	s.clients = reviewClients{client: client}
	err = s.Sync(t.Context(), p) // empty version metadata selects the real delete path
	require.Positive(t, sourceHeads.Load(), "test must reach backend source existence check")
	t.Logf("Sync error=%v; backend DELETE requests=%d", err, deletes.Load())
	if shouldDelete {
		require.NoError(t, err)
		require.EqualValues(t, 1, deletes.Load())
		return
	}
	if err == nil {
		t.Error("source existence check failure must be returned for retry")
	}
	if deletes.Load() != 0 {
		t.Error("replica must not be deleted when source existence is unknown")
	}
}
