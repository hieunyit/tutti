package handler

import (
	"context"
	"errors"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/testutil"
	"github.com/hieunyit/tutti/service/worker/copy"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

type versionWorkCopy struct {
	copy.CopySvc
	mu          sync.Mutex
	versions    []entity.ObjectVersionInfo
	copied      map[string][]string
	failVersion string
}

func (c *versionWorkCopy) GetVersionInfo(context.Context, string, copy.File) ([]entity.ObjectVersionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]entity.ObjectVersionInfo(nil), c.versions...), nil
}
func (c *versionWorkCopy) GetLastMigratedVersionInfo(_ context.Context, _ string, to copy.File) (entity.ObjectVersionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.copied[to.Storage]
	if len(v) == 0 {
		return entity.ObjectVersionInfo{}, dom.ErrNotFound
	}
	return entity.ObjectVersionInfo{Version: v[len(v)-1], Size: 4}, nil
}
func (c *versionWorkCopy) CopyObject(_ context.Context, _ string, from, to copy.File) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failVersion == from.Version {
		c.failVersion = ""
		return errors.New("injected destination outage")
	}
	c.copied[to.Storage] = append(c.copied[to.Storage], from.Version)
	return nil
}
func versionWorkSetup(t *testing.T) (*VersionedMigrationSvc, *versionWorkCopy, *store.ObjectVersionInfoStore, entity.VersionedObjectID, entity.BucketReplicationPolicy) {
	rc := testutil.SetupRedis(t)
	st := store.NewObjectVersionInfoStore(rc)
	c := &versionWorkCopy{versions: []entity.ObjectVersionInfo{{Version: "v2", Size: 4}, {Version: "v1", Size: 4}}, copied: map[string][]string{}}
	id := entity.NewVersionedObjectID("source", "bucket", "key")
	repl := entity.NewBucketReplicationPolicy("u", "source", "bucket", "dest-a", "bucket")
	return NewVersionedMigrationSvc(c, st, store.NewObjectLocker(rc, 0)), c, st, id, repl
}
func TestVersionWork_DestinationsOwnIndependentHistoryAndCleanup(t *testing.T) {
	s, c, st, id, a := versionWorkSetup(t)
	b := entity.NewBucketReplicationPolicy("u", "source", "bucket", "dest-b", "bucket")
	require.NoError(t, s.ListVersions(t.Context(), id, a))
	require.NoError(t, s.ListVersions(t.Context(), id, b))
	require.NoError(t, s.MigrateVersions(t.Context(), a, "key"))
	pending, err := st.ForReplication(b, 0).GetAll(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	require.NoError(t, s.MigrateVersions(t.Context(), b, "key"))
	require.Equal(t, []string{"v1", "v2"}, c.copied["dest-a"])
	require.Equal(t, []string{"v1", "v2"}, c.copied["dest-b"])
	for _, repl := range []entity.BucketReplicationPolicy{a, b} {
		remaining, err := st.ForReplication(repl, 0).GetAll(t.Context(), id)
		require.NoError(t, err)
		require.Empty(t, remaining)
	}
}
func TestVersionWork_RelistAfterNewVersionPreservesChronology(t *testing.T) {
	s, c, st, id, a := versionWorkSetup(t)
	require.NoError(t, s.ListVersions(t.Context(), id, a))
	c.versions = []entity.ObjectVersionInfo{{Version: "v3", Size: 4}, {Version: "v2", Size: 4}, {Version: "v1", Size: 4}}
	require.NoError(t, s.ListVersions(t.Context(), id, a))
	require.NoError(t, s.ListVersions(t.Context(), id, a))
	versions, err := st.ForReplication(a, 0).GetAll(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.NoError(t, s.MigrateVersions(t.Context(), a, "key"))
	require.Equal(t, []string{"v1", "v2", "v3"}, c.copied[a.ToStorage])
}
func TestVersionWork_ConcurrentListingsDoNotDuplicateSnapshot(t *testing.T) {
	s, _, st, id, a := versionWorkSetup(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Go(func() { errs <- s.ListVersions(t.Context(), id, a) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	versions, err := st.ForReplication(a, 0).GetAll(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, []entity.ObjectVersionInfo{{Version: "v1", Size: 4}, {Version: "v2", Size: 4}}, versions)
}
func TestVersionWork_GenerationCleanupDoesNotEraseNextRun(t *testing.T) {
	s, c, st, id, a := versionWorkSetup(t)
	require.NoError(t, s.ListVersions(t.Context(), id, a, 1))
	c.versions = []entity.ObjectVersionInfo{{Version: "v3", Size: 4}, {Version: "v2", Size: 4}, {Version: "v1", Size: 4}}
	require.NoError(t, s.ListVersions(t.Context(), id, a, 2))
	require.NoError(t, s.MigrateVersions(t.Context(), a, "key", 1))
	next, err := st.ForReplication(a, 2).GetAll(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, next, 3)
	require.NoError(t, s.MigrateVersions(t.Context(), a, "key", 2))
	require.Equal(t, []string{"v1", "v2", "v3"}, c.copied[a.ToStorage])
}
func TestVersionWork_FailedCopyResumesWithoutLosingSnapshot(t *testing.T) {
	s, c, st, id, a := versionWorkSetup(t)
	require.NoError(t, s.ListVersions(t.Context(), id, a))
	c.failVersion = "v2"
	require.Error(t, s.MigrateVersions(t.Context(), a, "key"))
	pending, err := st.ForReplication(a, 0).GetAll(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	require.NoError(t, s.MigrateVersions(t.Context(), a, "key"))
	require.Equal(t, []string{"v1", "v2"}, c.copied[a.ToStorage])
}
func TestVersionWork_LegacyPendingCopyRebuildsScopedSnapshot(t *testing.T) {
	s, c, st, id, a := versionWorkSetup(t)
	require.NoError(t, st.AddLeft(t.Context(), id, entity.ObjectVersionInfo{Version: "untrusted-legacy"}))
	require.NoError(t, s.MigrateVersions(t.Context(), a, "key"))
	require.Equal(t, []string{"v1", "v2"}, c.copied[a.ToStorage])
	legacy, err := st.GetAll(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, legacy, 1, "legacy work may still belong to another old worker")
}
