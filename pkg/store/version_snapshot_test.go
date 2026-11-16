package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func TestFindVersionAcrossSnapshotPages(t *testing.T) {
	c := testutil.SetupRedis(t)
	s := NewObjectVersionInfoStore(c)
	id := entity.NewVersionedObjectID("source", "bucket", "object")
	versions := make([]entity.ObjectVersionInfo, 2001)
	for i := range versions {
		versions[i] = entity.ObjectVersionInfo{Version: fmt.Sprintf("v%d", i), Size: uint64(i), LastModified: time.Date(2026, 11, 10, 0, 0, i, 0, time.UTC)}
	}
	versions[1500].DeleteMarker = true
	require.NoError(t, s.ReplaceSnapshot(t.Context(), id, versions))
	for _, index := range []int{0, 999, 1000, 1500, 2000} {
		checkpoint := versions[index]
		checkpoint.LastModified = time.Time{}
		got, err := s.FindVersion(t.Context(), id, checkpoint)
		require.NoError(t, err)
		require.EqualValues(t, index, got)
	}
	for _, checkpoint := range []entity.ObjectVersionInfo{
		{Version: "missing"},
		{Version: versions[1500].Version, Size: versions[1500].Size},
		{Version: versions[1000].Version, Size: versions[1000].Size + 1},
	} {
		_, err := s.FindVersion(t.Context(), id, checkpoint)
		require.ErrorIs(t, err, dom.ErrNotFound)
	}
}

func TestFindVersionPropagatesSnapshotErrors(t *testing.T) {
	c := testutil.SetupRedis(t)
	s := NewObjectVersionInfoStore(c)
	id := entity.NewVersionedObjectID("source", "bucket", "object")
	key, err := s.MakeKey(id)
	require.NoError(t, err)
	_, err = s.FindVersion(t.Context(), id, entity.ObjectVersionInfo{Version: "missing"})
	require.ErrorIs(t, err, dom.ErrNotFound)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = s.FindVersion(ctx, id, entity.ObjectVersionInfo{})
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, c.Set(t.Context(), key, "wrong type", 0).Err())
	_, err = s.FindVersion(t.Context(), id, entity.ObjectVersionInfo{})
	require.ErrorContains(t, err, "WRONGTYPE")
	require.NoError(t, c.Del(t.Context(), key).Err())
	require.NoError(t, c.RPush(t.Context(), key, "invalid JSON").Err())
	_, err = s.FindVersion(t.Context(), id, entity.ObjectVersionInfo{})
	require.ErrorContains(t, err, "unable to deserialize")
}
