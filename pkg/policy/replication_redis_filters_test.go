package policy

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type unavailablePolicyStore struct{ controlplane.Store }

func (unavailablePolicyStore) Repos() controlplane.Repos {
	return controlplane.Repos{Replications: unavailableReplications{}}
}

type unavailableReplications struct{ controlplane.Replications }

func (unavailableReplications) GetStatus(context.Context, controlplane.ReplicationID) (controlplane.ReplicationStatus, error) {
	return controlplane.ReplicationStatus{}, context.DeadlineExceeded
}

func TestRedisFallbackPreservesProjectedRestrictions(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = client.Close() })
	q := &tasks.QueueServiceMock{}
	tasks.Reset(q)
	svc := NewService(client, q, "main").WithControlPlane(unavailablePolicyStore{})
	bucket := entity.BucketReplicationPolicy{User: "u", FromStorage: "main", ToStorage: "dst", FromBucket: "src", ToBucket: "dst"}
	id := entity.UniversalFromBucketReplication(bucket)
	q.InitReplicationDone(id)
	q.EventReplicationInProgress(id)
	require.NoError(t, svc.bucketReplicationStatusStore.AddOp(ctx, bucket, entity.ReplicationStatus{DeletePropagation: "sync"}).Get())
	require.NoError(t, client.HSet(ctx, "p:repl_status_bucket:"+id.AsString(), map[string]any{
		"delete_include_prefixes": `["logs/"]`, "delete_exclude_prefixes": `["logs/protected/"]`,
		"include_patterns": `["logs/*"]`, "exclude_patterns": `["secret/*"]`,
	}).Err())
	got, err := svc.GetReplicationPolicyInfoExtended(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "sync", got.ReplicationStatus.DeletePropagation)
	require.Equal(t, []string{"logs/"}, got.ReplicationStatus.DeleteIncludePrefixes)
	require.Equal(t, []string{"logs/protected/"}, got.ReplicationStatus.DeleteExcludePrefixes)
	require.Equal(t, []string{"logs/*"}, got.ReplicationStatus.IncludePatterns)
	require.Equal(t, []string{"secret/*"}, got.ReplicationStatus.ExcludePatterns)
}

func TestRedisReplicationRestrictions(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = client.Close() })
	q := &tasks.QueueServiceMock{}
	tasks.Reset(q)
	svc := NewService(client, q, "main")
	user := entity.UserReplicationPolicy{User: "u", FromStorage: "main", ToStorage: "dst"}
	bucket := entity.BucketReplicationPolicy{User: "u", FromStorage: "main", ToStorage: "dst", FromBucket: "src", ToBucket: "dst"}
	cases := []struct {
		name string
		id   entity.UniversalReplicationID
		add  func(entity.ReplicationStatus) error
		key  string
	}{
		{"user", entity.UniversalFromUserReplication(user), func(st entity.ReplicationStatus) error {
			return svc.userReplicationStatusStore.AddOp(ctx, user, st).Get()
		}, "p:repl_status_user:"},
		{"bucket", entity.UniversalFromBucketReplication(bucket), func(st entity.ReplicationStatus) error {
			return svc.bucketReplicationStatusStore.AddOp(ctx, bucket, st).Get()
		}, "p:repl_status_bucket:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q.InitReplicationDone(tc.id)
			q.EventReplicationInProgress(tc.id)
			require.ErrorIs(t, svc.UpdateReplicationFilters(ctx, tc.id, nil, nil), dom.ErrNotFound)
			st := entity.ReplicationStatus{DeleteIncludePrefixes: []string{"a/"}, DeleteExcludePrefixes: []string{"b/"}, IncludePatterns: []string{"*.jpg"}, ExcludePatterns: []string{"secret/*"}}
			require.NoError(t, tc.add(st))
			got, err := svc.GetReplicationPolicyInfoExtended(ctx, tc.id)
			require.NoError(t, err)
			require.Equal(t, st.DeleteIncludePrefixes, got.ReplicationStatus.DeleteIncludePrefixes)
			require.Equal(t, st.DeleteExcludePrefixes, got.ReplicationStatus.DeleteExcludePrefixes)
			require.Equal(t, st.IncludePatterns, got.ReplicationStatus.IncludePatterns)
			require.Equal(t, st.ExcludePatterns, got.ReplicationStatus.ExcludePatterns)
			if tc.name == "user" {
				list, err := svc.userReplicationStatusStore.List(ctx)
				require.NoError(t, err)
				require.Equal(t, st.DeleteExcludePrefixes, list[user].DeleteExcludePrefixes)
				require.Equal(t, st.ExcludePatterns, list[user].ExcludePatterns)
			} else {
				list, err := svc.bucketReplicationStatusStore.List(ctx, "u")
				require.NoError(t, err)
				require.Equal(t, st.DeleteExcludePrefixes, list[bucket].DeleteExcludePrefixes)
				require.Equal(t, st.ExcludePatterns, list[bucket].ExcludePatterns)
			}
			require.NoError(t, svc.UpdateDeletePropagation(ctx, tc.id, "sync", nil, []string{"new/"}, time.Minute))
			require.NoError(t, svc.UpdateReplicationFilters(ctx, tc.id, []string{"*.png"}, nil))
			got, err = svc.GetReplicationPolicyInfoExtended(ctx, tc.id)
			require.NoError(t, err)
			require.Empty(t, got.ReplicationStatus.DeleteIncludePrefixes)
			require.Equal(t, []string{"new/"}, got.ReplicationStatus.DeleteExcludePrefixes)
			require.Equal(t, int64(60), got.ReplicationStatus.DeleteDelaySeconds)
			require.Equal(t, []string{"*.png"}, got.ReplicationStatus.IncludePatterns)
			require.Empty(t, got.ReplicationStatus.ExcludePatterns)
			require.NoError(t, svc.UpdateReplicationFilters(ctx, tc.id, nil, nil))
			require.NoError(t, svc.UpdateDeletePropagation(ctx, tc.id, "ignore", nil, nil, 0))
			got, err = svc.GetReplicationPolicyInfoExtended(ctx, tc.id)
			require.NoError(t, err)
			require.Empty(t, got.ReplicationStatus.IncludePatterns)
			require.Empty(t, got.ReplicationStatus.ExcludePatterns)
			require.Empty(t, got.ReplicationStatus.DeleteIncludePrefixes)
			require.Empty(t, got.ReplicationStatus.DeleteExcludePrefixes)
			require.Equal(t, "ignore", got.ReplicationStatus.DeletePropagation)
			require.Zero(t, got.ReplicationStatus.DeleteDelaySeconds)
			require.NoError(t, client.HSet(ctx, tc.key+tc.id.AsString(), "delete_exclude_prefixes", "invalid").Err())
			_, err = svc.GetReplicationPolicyInfoExtended(ctx, tc.id)
			require.Error(t, err)
		})
	}
}
