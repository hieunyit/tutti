// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package syncer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type deleteAuditCapture struct {
	mu      sync.Mutex
	entries []controlplane.DeleteAuditEntry
}

func (c *deleteAuditCapture) Record(ctx context.Context, entry controlplane.DeleteAuditEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entry)
	return nil
}

func makeTestClient(t *testing.T, sourceStatus int, deletes *atomic.Int32) s3client.Client {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			if deletes != nil {
				deletes.Add(1)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		if r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "versioning") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>Enabled</Status></VersioningConfiguration>`))
			return
		}
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
			return
		}
		if sourceStatus == http.StatusOK {
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.Header().Set("Content-Length", "4")
		}
		w.WriteHeader(sourceStatus)
	}))
	t.Cleanup(backend.Close)

	client, err := s3client.NewClient(t.Context(), reviewMetrics{}, s3.StorageAddress{Address: backend.URL, DefaultRegion: "us-east-1"}, s3.CredentialsV4{AccessKeyID: "access", SecretAccessKey: "secret"}, "storage", "u")
	require.NoError(t, err)

	raw, err := mclient.New(strings.TrimPrefix(backend.URL, "http://"), &mclient.Options{Region: "us-east-1", MaxRetries: 1})
	require.NoError(t, err)
	client.S3().Client = raw
	return client
}

func setupDeleteTask(p *tasks.ObjectSyncPayload) {
	p.Deleted = true
	p.Object.Bucket = "bucket"
	p.SetReplicationID(entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "u", FromStorage: "main", FromBucket: "bucket", ToStorage: "follower", ToBucket: "bucket",
	}), 0)
}

func TestDeletePropagation_IgnoreMode(t *testing.T) {
	s, copySvc, _, _, p := setup(t)
	setupDeleteTask(&p)
	var deletes atomic.Int32
	client := makeTestClient(t, http.StatusNotFound, &deletes)
	s.clients = reviewClients{client: client}

	audit := &deleteAuditCapture{}
	s.SetDeleteAuditRecorder(audit.Record)
	s.SetPolicyResolver(func(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{
			DeletePropagation: "ignore",
		}, nil
	})

	err := s.Sync(context.Background(), p)
	require.NoError(t, err)

	require.Equal(t, 0, len(copySvc.calls))
	require.EqualValues(t, 0, deletes.Load())

	require.Len(t, audit.entries, 1)
	require.Equal(t, "skipped", audit.entries[0].Action)
}

func TestDeletePropagation_SyncMode(t *testing.T) {
	s, _, _, _, p := setup(t)
	setupDeleteTask(&p)
	var deletes atomic.Int32
	client := makeTestClient(t, http.StatusNotFound, &deletes)
	s.clients = reviewClients{client: client}

	audit := &deleteAuditCapture{}
	s.SetDeleteAuditRecorder(audit.Record)
	s.SetPolicyResolver(func(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{
			DeletePropagation: "sync",
		}, nil
	})

	err := s.Sync(context.Background(), p)
	require.NoError(t, err)

	require.EqualValues(t, 1, deletes.Load())
	require.Len(t, audit.entries, 1)
	require.Equal(t, "deleted", audit.entries[0].Action)
}

func TestDeletePropagation_MarkerOnlyMode(t *testing.T) {
	s, _, _, _, p := setup(t)
	setupDeleteTask(&p)
	var deletes atomic.Int32
	client := makeTestClient(t, http.StatusNotFound, &deletes)
	s.clients = reviewClients{client: client}

	audit := &deleteAuditCapture{}
	s.SetDeleteAuditRecorder(audit.Record)
	s.SetPolicyResolver(func(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{
			DeletePropagation: "marker-only",
		}, nil
	})

	err := s.Sync(context.Background(), p)
	require.NoError(t, err)

	require.EqualValues(t, 1, deletes.Load())
	require.Len(t, audit.entries, 1)
	require.Equal(t, "marker_created", audit.entries[0].Action)
}

func TestMarkerOnlyRequiresConfirmedVersioning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		xml    string
	}{
		{"unversioned", http.StatusOK, "<VersioningConfiguration/>"},
		{"suspended", http.StatusOK, "<VersioningConfiguration><Status>Suspended</Status></VersioningConfiguration>"},
		{"forbidden", http.StatusForbidden, "<Error><Code>AccessDenied</Code></Error>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, _, p := setup(t)
			setupDeleteTask(&p)
			var deletes atomic.Int32
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodDelete:
					deletes.Add(1)
					w.WriteHeader(http.StatusNoContent)
				case http.MethodHead:
					w.WriteHeader(http.StatusNotFound)
				default:
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.xml))
				}
			}))
			t.Cleanup(backend.Close)
			s.clients = reviewClients{client: comprehensiveDeleteClient(t, backend.URL)}
			s.SetPolicyResolver(func(context.Context, entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
				return entity.ReplicationStatus{DeletePropagation: "marker-only"}, nil
			})
			require.ErrorContains(t, s.Sync(t.Context(), p), "versioning")
			require.Zero(t, deletes.Load())
		})
	}
}

func TestMarkerOnlyRecognizesAlreadyReplicatedMarker(t *testing.T) {
	s, _, _, _, p := setup(t)
	setupDeleteTask(&p)
	var deletes atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			deletes.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodHead:
			if deletes.Load() != 0 {
				w.Header().Set("x-amz-delete-marker", "true")
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte("<VersioningConfiguration><Status>Enabled</Status></VersioningConfiguration>"))
		}
	}))
	t.Cleanup(destination.Close)
	s.clients = comprehensiveDeleteRegistry{
		source: makeTestClient(t, http.StatusNotFound, nil),
		dest:   comprehensiveDeleteClient(t, destination.URL),
	}
	s.SetPolicyResolver(func(context.Context, entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{DeletePropagation: "marker-only"}, nil
	})
	require.NoError(t, s.Sync(t.Context(), p))
	require.NoError(t, s.Sync(t.Context(), p))
	require.EqualValues(t, 1, deletes.Load())
}

func TestDeletePropagation_PrefixFilter(t *testing.T) {
	s, _, _, _, p := setup(t)
	setupDeleteTask(&p)
	var deletes atomic.Int32
	client := makeTestClient(t, http.StatusNotFound, &deletes)
	s.clients = reviewClients{client: client}

	audit := &deleteAuditCapture{}
	s.SetDeleteAuditRecorder(audit.Record)
	s.SetPolicyResolver(func(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{
			DeletePropagation:     "sync",
			DeleteIncludePrefixes: []string{"data/", "uploads/"},
			DeleteExcludePrefixes: []string{"data/temp/"},
		}, nil
	})

	// 1. Not in include prefixes: skipped
	p.Object.Name = "logs/app.log"
	err := s.Sync(context.Background(), p)
	require.NoError(t, err)
	require.EqualValues(t, 0, deletes.Load())
	require.Len(t, audit.entries, 1)
	require.Equal(t, "skipped", audit.entries[0].Action)

	// 2. In exclude prefix: skipped
	p.Object.Name = "data/temp/scratch.txt"
	err = s.Sync(context.Background(), p)
	require.NoError(t, err)
	require.EqualValues(t, 0, deletes.Load())
	require.Len(t, audit.entries, 2)
	require.Equal(t, "skipped", audit.entries[1].Action)

	// 3. Matches include and not excluded: deleted
	p.Object.Name = "data/users/1.json"
	err = s.Sync(context.Background(), p)
	require.NoError(t, err)
	require.EqualValues(t, 1, deletes.Load())
	require.Len(t, audit.entries, 3)
	require.Equal(t, "deleted", audit.entries[2].Action)
}

func TestPatternFiltering_SyncSkipsFilteredObjects(t *testing.T) {
	s, copySvc, _, verSvc, p := setup(t)
	s.SetPolicyResolver(func(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{
			IncludePatterns: []string{"*.jpg", "docs/*"},
			ExcludePatterns: []string{"*.tmp", "docs/drafts/*"},
		}, nil
	})

	// Set version from=2, to=1 to trigger copy attempt
	_ = verSvc.UpdateIfGreater(context.Background(), p.ID, p.Object, meta.Destination{Storage: "dest"}, 2)

	// Excluded by pattern (*.tmp)
	p.Object.Name = "photo.tmp"
	err := s.Sync(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, 0, len(copySvc.calls))

	// Excluded by exclude pattern (docs/drafts/*)
	p.Object.Name = "docs/drafts/proposal.pdf"
	err = s.Sync(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, 0, len(copySvc.calls))

	// Not included (not matching *.jpg or docs/*)
	p.Object.Name = "videos/clip.mp4"
	err = s.Sync(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, 0, len(copySvc.calls))
}

func TestPatternFiltering_RecordsSkippedDeleteEvent(t *testing.T) {
	s, _, _, _, p := setup(t)
	setupDeleteTask(&p)
	p.Object.Name = "logs/object"
	p.DeleteEventID = 45
	p.DeleteEventRetryCount = 3
	s.SetPolicyResolver(func(context.Context, entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{IncludePatterns: []string{"docs/*"}}, nil
	})
	audit := &deleteAuditCapture{}
	s.SetDeleteAuditRecorder(audit.Record)
	require.NoError(t, s.Sync(t.Context(), p))
	require.Len(t, audit.entries, 1)
	require.Equal(t, "skipped", audit.entries[0].Action)
	require.Equal(t, p.DeleteEventID, audit.entries[0].DeleteEventID)
	require.Equal(t, p.DeleteEventRetryCount, audit.entries[0].DeleteEventRetryCount)
}
