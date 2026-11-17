package syncer

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestDeleteDeadlineProtectsExplicitAndOlderWrite(t *testing.T) {
	s, _, _, versions, p := setup(t)
	setupDeleteTask(&p)
	p.Deleted = false
	var deletes atomic.Int32
	s.clients = reviewClients{client: makeTestClient(t, http.StatusNotFound, &deletes)}
	s.SetPolicyResolver(func(context.Context, entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{DeletePropagation: "sync", DeleteDelaySeconds: 3600}, nil
	})
	deadline, err := versions.DeleteObjWithDeadline(t.Context(), p.ID, p.Object, time.Now().Add(time.Hour))
	require.NoError(t, err)
	var deferred *DeleteDeferredError
	require.ErrorAs(t, s.Sync(t.Context(), p), &deferred)
	require.Equal(t, deadline, deferred.Deadline)
	p.Deleted = true
	require.ErrorIs(t, s.Sync(t.Context(), p), ErrDeleteDeferred)
	require.Zero(t, deletes.Load())
}

func TestDeleteDeadlineDoesNotDeleteRecreatedSource(t *testing.T) {
	s, _, _, versions, p := setup(t)
	setupDeleteTask(&p)
	p.Deleted = true
	var deletes atomic.Int32
	s.clients = reviewClients{client: makeTestClient(t, http.StatusOK, &deletes)}
	s.SetPolicyResolver(func(context.Context, entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
		return entity.ReplicationStatus{DeletePropagation: "sync", DeleteDelaySeconds: 3600}, nil
	})
	_, err := versions.DeleteObjWithDeadline(t.Context(), p.ID, p.Object, time.Now().Add(-time.Second))
	require.NoError(t, err)
	require.NoError(t, s.Sync(t.Context(), p))
	require.Zero(t, deletes.Load())
}

func TestFutureDeleteGetsNewDeadlineAfterRecreation(t *testing.T) {
	s, _, _, versions, p := setup(t)
	setupDeleteTask(&p)
	var deletes atomic.Int32
	s.clients = reviewClients{client: makeTestClient(t, http.StatusNotFound, &deletes)}
	ctx := t.Context()
	_, err := versions.DeleteObjWithDeadline(ctx, p.ID, p.Object, time.Now().Add(-time.Second))
	require.NoError(t, err)
	require.NoError(t, s.Sync(ctx, p))
	require.EqualValues(t, 1, deletes.Load())
	_, err = versions.IncrementObj(ctx, p.ID, p.Object, meta.Destination{Storage: p.ID.FromStorage(), Bucket: p.Object.Bucket})
	require.NoError(t, err)
	cleared, err := versions.GetOrSetDeleteDeadline(ctx, p.ID, p.Object, time.Time{})
	require.NoError(t, err)
	require.True(t, cleared.IsZero())
	deadline, err := versions.DeleteObjWithDeadline(ctx, p.ID, p.Object, time.Now().Add(time.Hour))
	require.NoError(t, err)
	var deferred *DeleteDeferredError
	require.ErrorAs(t, s.Sync(ctx, p), &deferred)
	require.Equal(t, deadline, deferred.Deadline)
	require.EqualValues(t, 1, deletes.Load())
}

func TestDeleteDeadlinesAreIndependentPerDestination(t *testing.T) {
	s, _, _, versions, p := setup(t)
	setupDeleteTask(&p)
	var deletes atomic.Int32
	s.clients = reviewClients{client: makeTestClient(t, http.StatusNotFound, &deletes)}
	ctx := t.Context()
	other := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: p.ID.User(), FromStorage: p.ID.FromStorage(), FromBucket: p.Object.Bucket,
		ToStorage: "other", ToBucket: p.Object.Bucket,
	})
	_, err := versions.DeleteObjWithDeadline(ctx, p.ID, p.Object, time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = versions.DeleteObjWithDeadline(ctx, other, p.Object, time.Now().Add(-time.Second))
	require.NoError(t, err)
	require.ErrorIs(t, s.Sync(ctx, p), ErrDeleteDeferred)
	p.SetReplicationID(other, 0)
	require.NoError(t, s.Sync(ctx, p))
	require.EqualValues(t, 1, deletes.Load())
}

func TestMissingVersionMetadataStillCopiesExistingObject(t *testing.T) {
	for _, status := range []entity.ReplicationStatus{
		{DeletePropagation: "sync", DeleteDelaySeconds: 3600},
		{DeletePropagation: "ignore", DeleteDelaySeconds: 3600},
		{DeletePropagation: "sync", DeleteExcludePrefixes: []string{"o"}, DeleteDelaySeconds: 3600},
	} {
		s, copier, _, _, p := setup(t)
		setupDeleteTask(&p)
		p.Deleted = false
		var deletes atomic.Int32
		s.clients = reviewClients{client: makeTestClient(t, http.StatusOK, &deletes)}
		s.SetPolicyResolver(func(context.Context, entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
			return status, nil
		})
		require.NoError(t, s.Sync(t.Context(), p))
		require.Len(t, copier.calls, 2)
		require.Zero(t, deletes.Load())
	}
}
