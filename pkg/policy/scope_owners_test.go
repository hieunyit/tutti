package policy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/replscope"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func newScopeTestService(t *testing.T) *policySvc {
	c := testutil.SetupRedis(t)
	qs := &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	return NewService(c, qs, "main")
}

func TestListScopeOwnersIncludesGroupsAndPolicies(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := t.Context()
	r := require.New(t)

	storageLevel := entity.UserReplicationPolicy{FromStorage: "a", ToStorage: "b"}
	userBucket := entity.BucketReplicationPolicy{User: "alice", FromStorage: "main", ToStorage: "c", FromBucket: "x", ToBucket: "x"}
	storageBucket := entity.BucketReplicationPolicy{FromStorage: "d", ToStorage: "e", FromBucket: "y", ToBucket: "z"}
	userLevel := entity.UserReplicationPolicy{User: "bob", FromStorage: "main", ToStorage: "f"}
	for _, p := range []entity.UserReplicationPolicy{storageLevel, userLevel} {
		_, err := svc.AddUserReplicationPolicy(ctx, p, entity.ReplicationOptions{})
		r.NoError(err)
	}
	for _, p := range []entity.BucketReplicationPolicy{userBucket, storageBucket} {
		_, err := svc.AddBucketReplicationPolicy(ctx, p, entity.ReplicationOptions{})
		r.NoError(err)
	}
	// a group and the replication it manages: the replication is represented by the group, not listed twice
	g := protectiongroups.Group{ID: "g1", Members: []string{"p", "q"}, Primary: "p", Copies: 2}
	r.NoError(protectiongroups.NewStore(svc.client).Save(ctx, &g))
	managed := g.Replications("p")[0]
	u, _ := managed.AsUserID()
	_, err := svc.AddUserReplicationPolicy(ctx, u, entity.ReplicationOptions{})
	r.NoError(err)

	owners, err := svc.ListScopeOwners(ctx)
	r.NoError(err)
	byID := map[string]replscope.Owner{}
	for _, o := range owners {
		byID[o.ID] = o
	}
	r.Len(owners, 5, "%+v", owners)
	r.Equal(replscope.Owner{Kind: "group", ID: "g1", Scope: replscope.GroupScope([]string{"p", "q"}, nil)}, byID["g1"])
	id := idStr(entity.UniversalFromUserReplication(storageLevel))
	r.Equal(replscope.Owner{Kind: "replication", ID: id, Scope: replscope.ReplicationScope("a", "b", nil)}, byID[id])
	id = idStr(entity.UniversalFromUserReplication(userLevel))
	r.Equal(replscope.ReplicationScope("main", "f", nil), byID[id].Scope)
	id = idStr(entity.UniversalFromBucketReplication(userBucket))
	r.Equal(replscope.ReplicationScope("main", "c", []string{"x"}), byID[id].Scope)
	id = idStr(entity.UniversalFromBucketReplication(storageBucket))
	r.Equal(replscope.ReplicationScope("d", "e", []string{"y", "z"}), byID[id].Scope)
	_, ok := byID[managed.AsString()]
	r.False(ok)

	// a standalone replication next to the group is allowed, one into a member is not
	r.Nil(replscope.Check(replscope.ReplicationScope("p", "w", nil), owners, ""))
	r.NotNil(replscope.Check(replscope.ReplicationScope("w", "q", nil), owners, ""))
}

// The per-user exclusivity (a user-level replication blocks bucket replications of the same user and vice versa)
// is about one tenant: storage-level replications (empty user) of unrelated storage pairs must not block each other.
func TestStorageLevelNotBlockedByUserExclusivity(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := t.Context()
	r := require.New(t)

	_, err := svc.AddUserReplicationPolicy(ctx, entity.UserReplicationPolicy{FromStorage: "a", ToStorage: "b"}, entity.ReplicationOptions{})
	r.NoError(err)
	_, err = svc.AddBucketReplicationPolicy(ctx, entity.BucketReplicationPolicy{FromStorage: "c", ToStorage: "d", FromBucket: "x", ToBucket: "x"}, entity.ReplicationOptions{})
	r.NoError(err)
	_, err = svc.AddUserReplicationPolicy(ctx, entity.UserReplicationPolicy{FromStorage: "e", ToStorage: "f"}, entity.ReplicationOptions{})
	r.NoError(err)

	// storage-level replications do not set routing for the empty user
	_, err = svc.userRoutingStore.GetOp(ctx, "").Get()
	r.Error(err)

	// real users keep the exclusivity
	_, err = svc.AddUserReplicationPolicy(ctx, entity.UserReplicationPolicy{User: "alice", FromStorage: "main", ToStorage: "b"}, entity.ReplicationOptions{})
	r.NoError(err)
	_, err = svc.AddBucketReplicationPolicy(ctx, entity.BucketReplicationPolicy{User: "alice", FromStorage: "main", ToStorage: "b", FromBucket: "x", ToBucket: "x"}, entity.ReplicationOptions{})
	r.Error(err)
}

func idStr(id entity.UniversalReplicationID) string { return id.AsString() }
