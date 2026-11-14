// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRuntimeStorageLifecycle(t *testing.T) {
	var reject atomic.Bool
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if reject.Load() {
			w.WriteHeader(403)
			_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
			return
		}
		_, _ = w.Write([]byte(`<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Owner><ID>u</ID></Owner><Buckets/></ListAllMyBucketsResult>`))
	}))
	defer backend.Close()
	rc := testutil.SetupRedis(t)
	cfg := &objstore.Config{Main: "main", Storages: map[string]objstore.Storage{"main": {CommonConfig: objstore.CommonConfig{Type: dom.S3}, S3: &s3.Storage{StorageAddress: s3.StorageAddress{Address: backend.URL, Provider: s3.ProviderMinIO}}}}, DynamicCredentials: objstore.DynamicCredentialsConfig{Enabled: true, MasterPassword: "runtime-test-password", PollInterval: time.Hour}}
	creds, err := objstore.NewCredsSvc(t.Context(), cfg, rc, objstore.WithManagedStorages(rc))
	require.NoError(t, err)
	var references []string
	h := ResilienceHandlers(rc, nil, WithRuntimeStorages(creds, func(context.Context, string) ([]string, error) { return references, nil }))
	def := &pb.RuntimeStorage{Name: "archive", Address: backend.URL, Provider: "Minio", DefaultRegion: "us-east-1"}
	cr := &pb.RuntimeStorageCredential{User: "u", AccessKeyId: "key", SecretAccessKey: "secret"}
	reject.Store(true)
	_, err = h.CreateRuntimeStorage(t.Context(), &pb.CreateRuntimeStorageRequest{Storage: def, Credentials: []*pb.RuntimeStorageCredential{cr}})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	_, exists := creds.GetManaged("archive")
	require.False(t, exists, "failed verification must not save")
	reject.Store(false)
	created, err := h.CreateRuntimeStorage(t.Context(), &pb.CreateRuntimeStorageRequest{Storage: def, Credentials: []*pb.RuntimeStorageCredential{cr}})
	require.NoError(t, err)
	require.Equal(t, "managed", created.Source)
	require.Len(t, created.Users, 1)
	listed, err := h.ListRuntimeStorages(t.Context(), &pb.ListRuntimeStoragesRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Storages, 2)
	references = []string{"bucket routing user/bucket"}
	_, err = h.DeleteRuntimeStorage(t.Context(), &pb.RuntimeStorageName{Name: "archive"})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	_, err = h.UpdateRuntimeStorage(t.Context(), &pb.UpdateRuntimeStorageRequest{Storage: def})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	_, err = h.UpdateRuntimeStorage(t.Context(), &pb.UpdateRuntimeStorageRequest{Storage: def, Confirm: true, Force: true})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	_, err = h.UpdateRuntimeStorage(t.Context(), &pb.UpdateRuntimeStorageRequest{Storage: def, Confirm: true})
	require.NoError(t, err)
	_, err = h.DeleteRuntimeStorage(t.Context(), &pb.RuntimeStorageName{Name: "main"})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	_, err = h.UpdateRuntimeStorage(t.Context(), &pb.UpdateRuntimeStorageRequest{Storage: &pb.RuntimeStorage{Name: "main", Address: backend.URL, Provider: "Minio"}})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	references = nil
	_, err = h.DeleteRuntimeStorage(t.Context(), &pb.RuntimeStorageName{Name: "archive"})
	require.NoError(t, err)
	_, err = creds.GetS3Credentials("archive", "u")
	require.Error(t, err)
}
func TestStorageReferencesFindOrphanedScopesAndDiffs(t *testing.T) {
	rc := testutil.SetupRedis(t)
	cfg := &objstore.Config{Main: "main", Storages: map[string]objstore.Storage{"main": {CommonConfig: objstore.CommonConfig{Type: dom.S3}, S3: &s3.Storage{}}}}
	creds, err := objstore.NewCredsSvc(t.Context(), cfg, rc)
	require.NoError(t, err)
	pol := policy.NewService(rc, &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}, "main")
	require.NoError(t, rc.HSet(t.Context(), "p:route_bucket:orphan", "bucket", "archive").Err())
	get := RuntimeStorageReferences(pol, creds, rc, rc)
	refs, err := get(t.Context(), "archive")
	require.NoError(t, err)
	require.Contains(t, refs, "bucket routing orphan/bucket")
	require.NoError(t, failover.NewStore(rc, nil).SetFailoverPolicy(t.Context(), failover.Policy{Source: "archive", Targets: []string{"main"}}))
	_, err = store.NewDiffIDStore(rc).AddOp(t.Context(), struct{}{}, entity.DiffID{Locations: []entity.DiffLocation{{Storage: "archive", Bucket: "bucket"}, {Storage: "main", Bucket: "bucket"}}}).Get()
	require.NoError(t, err)
	refs, err = get(t.Context(), "archive")
	require.NoError(t, err)
	require.Len(t, refs, 3)
	// Reference protection must fail closed when Redis is unavailable.
	require.NoError(t, rc.(*redis.Client).Close())
	_, err = get(t.Context(), "archive")
	require.Error(t, err)
}
