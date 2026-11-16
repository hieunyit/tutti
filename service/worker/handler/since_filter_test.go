// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// stepClock is a gofakes3 time source whose "now" is set by the test, so objects get chosen LastModified values.
type stepClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *stepClock) Now() time.Time                  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *stepClock) Since(t time.Time) time.Duration { return c.Now().Sub(t) }
func (c *stepClock) set(t time.Time)                 { c.mu.Lock(); c.t = t; c.mu.Unlock() }

var sinceBase = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// sinceEnv has 6 objects: old-1..old-3 modified at base-10d/-5d/-1d and new-1..new-3 at base, base+1h, base+2h.
func sinceEnv(t *testing.T) (*svc, *mockQueueRecorder, *miniredis.Miniredis, string, entity.UniversalReplicationID) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	clock := &stepClock{}
	backend := s3mem.New(s3mem.WithTimeSource(clock))
	bucket := "bkt"
	require.NoError(t, backend.CreateBucket(bucket))
	put := func(key string, at time.Time, size int) {
		clock.set(at)
		_, err := backend.PutObject(bucket, key, nil, strings.NewReader(strings.Repeat("x", size)), int64(size), nil)
		require.NoError(t, err)
	}
	put("old-1", sinceBase.Add(-10*24*time.Hour), 10)
	put("old-2", sinceBase.Add(-5*24*time.Hour), 20)
	put("old-3", sinceBase.Add(-24*time.Hour), 30)
	put("new-1", sinceBase, 100)
	put("new-2", sinceBase.Add(time.Hour), 200)
	put("new-3", sinceBase.Add(2*time.Hour), 300)
	ts := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(ts.Close)
	client, err := s3client.NewClient(t.Context(), metrics.NewS3Service(false),
		s3.StorageAddress{Address: ts.URL, HttpTimeout: time.Minute},
		s3.CredentialsV4{AccessKeyID: "test", SecretAccessKey: "test"}, "from-storage", "test-user")
	require.NoError(t, err)
	rec := &mockQueueRecorder{}
	retry := time.Second
	s := &svc{
		conf:           &Config{CustomErrRetryInterval: &retry},
		clients:        singleS3Clients{client: client},
		listStateStore: store.NewMigrationObjectListStateStore(rc),
		queueSvc:       rec,
		limit:          ratelimit.New(rc, nil),
	}
	id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "test-user", FromStorage: "from-storage", ToStorage: "to-storage", FromBucket: bucket, ToBucket: bucket,
	})
	ResetQueueAdmissionState()
	t.Cleanup(ResetQueueAdmissionState)
	return s, rec, mr, bucket, id
}

func runList(t *testing.T, s *svc, bucket string, id entity.UniversalReplicationID, versioned bool) {
	t.Helper()
	payload := tasks.MigrateBucketListObjectsPayload{Bucket: bucket, Iteration: 1, Versioned: versioned}
	payload.SetReplicationID(id, 1)
	task, err := tasks.EncodeAny(t.Context(), payload)
	require.NoError(t, err)
	require.NoError(t, s.HandleMigrationBucketListObj(t.Context(), task))
}

func enqueuedKeys(rec *mockQueueRecorder) (copies, lists []string) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, task := range rec.tasks {
		switch p := task.(type) {
		case tasks.MigrateObjCopyPayload:
			copies = append(copies, p.Obj.Name)
		case tasks.ListObjectVersionsPayload:
			lists = append(lists, p.Prefix)
		}
	}
	return
}

func TestListSince(t *testing.T) {
	at := func(d time.Duration) *time.Time { v := sinceBase.Add(d); return &v }
	for _, tc := range []struct {
		name      string
		since     *time.Time
		wantKeys  []string
		wantBytes int64
	}{
		{"no cutoff copies everything", nil, []string{"new-1", "new-2", "new-3", "old-1", "old-2", "old-3"}, 660},
		{"cutoff skips older, exact boundary included", at(0), []string{"new-1", "new-2", "new-3"}, 600},
		{"cutoff between objects", at(30 * time.Minute), []string{"new-2", "new-3"}, 500},
		{"cutoff after the newest object", at(3 * time.Hour), nil, 0},
		{"cutoff far in the past equals full", at(-100 * 24 * time.Hour), []string{"new-1", "new-2", "new-3", "old-1", "old-2", "old-3"}, 660},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _, bucket, id := sinceEnv(t)
			since := tc.since
			s.SetSinceResolver(func(_ context.Context, got entity.UniversalReplicationID) (*time.Time, error) {
				require.Equal(t, id, got)
				return since, nil
			})
			runList(t, s, bucket, id, false)
			copies, _ := enqueuedKeys(rec)
			require.ElementsMatch(t, tc.wantKeys, copies)
			bytes, err := s.initBytes().Get(t.Context(), id.AsString(), 1, time.Now())
			require.NoError(t, err)
			require.Equal(t, tc.wantBytes, bytes.Total, "bytes counter only counts enqueued objects")
		})
	}
}

func TestListSince_VersionedBucketFiltersKeysByLatestVersion(t *testing.T) {
	s, rec, _, bucket, id := sinceEnv(t)
	cut := sinceBase
	s.SetSinceResolver(func(context.Context, entity.UniversalReplicationID) (*time.Time, error) { return &cut, nil })
	runList(t, s, bucket, id, true)
	copies, lists := enqueuedKeys(rec)
	require.Empty(t, copies)
	require.ElementsMatch(t, []string{"new-1", "new-2", "new-3"}, lists)
}

func TestListSince_ResolverFailureRetriesAndMissingReplicationMeansNoCutoff(t *testing.T) {
	s, rec, _, bucket, id := sinceEnv(t)
	s.SetSinceResolver(func(context.Context, entity.UniversalReplicationID) (*time.Time, error) {
		return nil, fmt.Errorf("redis down")
	})
	payload := tasks.MigrateBucketListObjectsPayload{Bucket: bucket, Iteration: 1}
	payload.SetReplicationID(id, 1)
	task, err := tasks.EncodeAny(t.Context(), payload)
	require.NoError(t, err)
	require.Error(t, s.HandleMigrationBucketListObj(t.Context(), task))
	copies, _ := enqueuedKeys(rec)
	require.Empty(t, copies, "an unreadable cutoff must never fall back to copying everything")

	s.SetSinceResolver(func(context.Context, entity.UniversalReplicationID) (*time.Time, error) { return nil, dom.ErrNotFound })
	runList(t, s, bucket, id, false)
	copies, _ = enqueuedKeys(rec)
	require.Len(t, copies, 6)
}

func TestVersionSnapshotSince(t *testing.T) {
	s, c, st, id, a := versionWorkSetup(t)
	c.versions = []entity.ObjectVersionInfo{
		{Version: "v3", Size: 4, LastModified: sinceBase.Add(time.Hour)},
		{Version: "dm", Size: 0, LastModified: sinceBase.Add(-time.Hour), DeleteMarker: true},
		{Version: "v2", Size: 4, LastModified: sinceBase},
		{Version: "v1", Size: 4, LastModified: sinceBase.Add(-time.Hour)},
	}
	cut := sinceBase
	require.NoError(t, s.ListVersionsSince(t.Context(), id, a, &cut))
	got, err := st.ForReplication(a, 0).GetAll(t.Context(), id)
	require.NoError(t, err)
	var names []string
	for _, v := range got {
		names = append(names, v.Version)
	}
	require.Equal(t, []string{"v2", "v3"}, names, "versions and delete markers older than the cutoff are dropped, the boundary is kept")
	require.NoError(t, s.MigrateVersionsSince(t.Context(), a, "key", &cut))
	require.Equal(t, []string{"v2", "v3"}, c.copied[a.ToStorage])
}
