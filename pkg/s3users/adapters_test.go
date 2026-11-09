package s3users

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/failover"
)

type fakePolicies struct {
	user    map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended
	bucket  map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended
	routing map[string]string
	bRoute  map[string]string
	err     error
}

func (f fakePolicies) ListUserReplicationsInfo(context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return f.user, f.err
}
func (f fakePolicies) ListBucketReplicationsInfo(context.Context, string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return f.bucket, f.err
}
func (f fakePolicies) GetUserRoutings(context.Context) (map[string]string, error) {
	return f.routing, f.err
}
func (f fakePolicies) GetBucketRoutings(context.Context, string) (map[string]string, error) {
	return f.bRoute, f.err
}

func TestPolicyReferences(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	events := failover.NewStore(rc, nil)

	// the real policy service reports "not found" when nothing exists: that means no references
	notFound := PolicyReferences(fakePolicies{err: dom.ErrNotFound}, events)
	r, err := notFound(ctx, "u")
	require.NoError(t, err)
	require.Empty(t, r)

	none := PolicyReferences(fakePolicies{}, events)
	r, err = none(ctx, "u")
	require.NoError(t, err)
	require.Empty(t, r)

	require.NoError(t, events.SaveFailoverEvent(ctx, failover.Event{User: "u", From: "a", To: "b", Time: time.Now()}))
	p := fakePolicies{
		user:    map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended{{User: "u", FromStorage: "a", ToStorage: "b"}: {}, {User: "other", FromStorage: "a", ToStorage: "b"}: {}},
		bucket:  map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{{User: "u", FromStorage: "a", FromBucket: "x", ToStorage: "b", ToBucket: "x"}: {}},
		routing: map[string]string{"u": "b"},
		bRoute:  map[string]string{"x": "b"},
	}
	r, err = PolicyReferences(p, events)(ctx, "u")
	require.NoError(t, err)
	require.Len(t, r, 4)
	require.Contains(t, r[0], "2 replication(s)")
	r, err = PolicyReferences(p, events)(ctx, "nobody")
	require.NoError(t, err)
	require.Len(t, r, 2, "routings returned by the fake are user-agnostic, only the real user filter applies to replications and events")
}
