package policy

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
)

type fakeLister struct {
	user   map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended
	bucket map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended
}

func (f *fakeLister) ListUserReplicationsInfo(context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return f.user, nil
}
func (f *fakeLister) ListBucketReplicationsInfo(_ context.Context, u string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error) {
	out := map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{}
	for p, s := range f.bucket {
		if p.User == u {
			out[p] = s
		}
	}
	return out, nil
}

func lagStatus(n int) entity.ReplicationStatusExtended {
	return entity.ReplicationStatusExtended{EventMigration: entity.QueueStats{Unprocessed: n}}
}

func TestReplicationUnprocessedMetricMoves(t *testing.T) {
	l := &fakeLister{
		user: map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended{
			{User: "u1", FromStorage: "a", ToStorage: "b"}: lagStatus(5)},
		bucket: map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{
			{User: "u2", FromStorage: "a", ToStorage: "b", FromBucket: "x", ToBucket: "x"}: lagStatus(3),
			{User: "u2", FromStorage: "a", ToStorage: "b", FromBucket: "y", ToBucket: "y"}: lagStatus(4)},
	}
	users := func() []string { return []string{"u1", "u2", "u2"} }
	require.NoError(t, CollectReplicationMetrics(t.Context(), l, users))
	require.Equal(t, 5.0, testutil.ToFloat64(replicationUnprocessed.WithLabelValues("u1", "a", "b")))
	require.Equal(t, 7.0, testutil.ToFloat64(replicationUnprocessed.WithLabelValues("u2", "a", "b")))

	l.user[entity.UserReplicationPolicy{User: "u1", FromStorage: "a", ToStorage: "b"}] = lagStatus(0)
	delete(l.bucket, entity.BucketReplicationPolicy{User: "u2", FromStorage: "a", ToStorage: "b", FromBucket: "x", ToBucket: "x"})
	require.NoError(t, CollectReplicationMetrics(t.Context(), l, users))
	require.Equal(t, 0.0, testutil.ToFloat64(replicationUnprocessed.WithLabelValues("u1", "a", "b")))
	require.Equal(t, 4.0, testutil.ToFloat64(replicationUnprocessed.WithLabelValues("u2", "a", "b")))
	// removed replications leave no stale series
	delete(l.user, entity.UserReplicationPolicy{User: "u1", FromStorage: "a", ToStorage: "b"})
	require.NoError(t, CollectReplicationMetrics(t.Context(), l, users))
	require.Equal(t, 1, testutil.CollectAndCount(replicationUnprocessed))
}
