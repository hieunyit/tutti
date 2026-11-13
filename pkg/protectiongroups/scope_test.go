// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/replscope"
)

// An infrastructure (storage-level) group replicates with the storage connection accounts: its replications have
// an empty user.
func TestGroupReplicationsEmptyUser(t *testing.T) {
	all := Group{ID: "g", Members: []string{"a", "b", "c"}}
	ids := all.Replications("a")
	require.Len(t, ids, 2)
	for _, id := range ids {
		require.True(t, id.IsStorageLevel())
		require.Equal(t, "", id.User())
		require.Equal(t, "a", id.FromStorage())
		_, ok := id.AsUserID()
		require.True(t, ok)
	}
	buckets := Group{ID: "g", Members: []string{"a", "b"}, Buckets: []string{"x", "y"}}
	ids = buckets.Replications("b")
	require.Len(t, ids, 2)
	for _, id := range ids {
		require.Equal(t, "", id.User())
		_, ok := id.AsBucketID()
		require.True(t, ok)
	}
	// a user group keeps its user
	user := Group{ID: "g", User: "alice", Members: []string{"a", "b"}}
	require.Equal(t, "alice", user.Replications("a")[0].User())
}

func TestGroupScope(t *testing.T) {
	g := Group{ID: "g", Members: []string{"a", "b", "c"}, Buckets: []string{"x"}}
	require.Equal(t, replscope.GroupScope([]string{"a", "b", "c"}, []string{"x"}), g.Scope())
	require.Equal(t, "", g.Scope().User, "group without user is storage-level")
	gu := Group{ID: "gu", User: "alice", Members: []string{"a", "b"}}
	require.Equal(t, "alice", gu.Scope().User)
	require.Equal(t, replscope.GroupScope([]string{"a", "b"}, nil).WithUser("alice"), gu.Scope())
	owners := GroupOwners([]Group{g})
	require.Equal(t, []replscope.Owner{{Kind: "group", ID: "g", Scope: g.Scope()}}, owners)
}

func scopeService(t *testing.T, extra ...replscope.Owner) *Service {
	svc, _, _, _, _ := newService(t)
	svc.ScopeLocker = replscope.NewRedisLocker(svc.Store.Redis)
	svc.ScopeOwners = func(ctx context.Context) ([]replscope.Owner, error) {
		groups, err := svc.Store.List(ctx)
		if err != nil {
			return nil, err
		}
		return append(GroupOwners(groups), extra...), nil
	}
	return svc
}

// A group creation that overlaps another group is refused with AlreadyExists and stores nothing.
func TestGroupCreateRejectedOnOverlapLeavesNothing(t *testing.T) {
	svc := scopeService(t)
	ctx := t.Context()
	_, err := svc.Create(ctx, Group{ID: "g1", Members: []string{"a", "b"}, Copies: 2}, false)
	require.NoError(t, err)

	_, err = svc.Create(ctx, Group{ID: "g2", Members: []string{"a", "c"}, Buckets: []string{"x"}, Copies: 2}, false)
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
	require.Contains(t, err.Error(), `group "g1"`)

	groups, err := svc.Store.List(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "g1", groups[0].ID)
	index, err := svc.Store.Redis.HGetAll(ctx, ScopeIndexKey).Result()
	require.NoError(t, err)
	for field := range index {
		require.NotContains(t, field, "g2")
	}
	// the scope lock is released
	unlock, err := svc.ScopeLocker.Lock(ctx)
	require.NoError(t, err)
	unlock()
}

// A group creation that overlaps a standalone replication is refused; a disjoint one is allowed.
func TestGroupCreateRejectedOnReplicationOverlap(t *testing.T) {
	repl := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{FromStorage: "z", ToStorage: "c", FromBucket: "x", ToBucket: "x"})
	svc := scopeService(t, replscope.Owner{Kind: "replication", ID: repl.AsString(), Scope: replscope.ReplicationScope("z", "c", []string{"x"})})
	ctx := t.Context()

	_, err := svc.Create(ctx, Group{ID: "g2", Members: []string{"a", "c"}, Buckets: []string{"x"}, Copies: 2}, false)
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
	require.Contains(t, err.Error(), "replication")
	_, err = svc.Store.Get(ctx, "g2")
	require.ErrorIs(t, err, dom.ErrNotFound)

	// different bucket: allowed
	_, err = svc.Create(ctx, Group{ID: "g3", Members: []string{"a", "c"}, Buckets: []string{"y"}, Copies: 2}, false)
	require.NoError(t, err)
}

// The replications a group itself manages (adopted on creation) are not conflicts of that group.
func TestGroupCreateIgnoresOwnReplications(t *testing.T) {
	own := entity.UniversalFromUserReplication(entity.UserReplicationPolicy{FromStorage: "a", ToStorage: "b"})
	svc := scopeService(t, replscope.Owner{Kind: "replication", ID: own.AsString(), Scope: replscope.ReplicationScope("a", "b", nil)})
	_, err := svc.Create(t.Context(), Group{ID: "g", Members: []string{"a", "b"}, Copies: 2}, true)
	require.NoError(t, err)
}

// A replication of the group's own topology added between Backend.Validate and the scope lock must not be
// silently taken over by a non-adopting create.
func TestGroupCreateNonAdoptRejectsReplicationAddedBeforeLock(t *testing.T) {
	own := entity.UniversalFromUserReplication(entity.UserReplicationPolicy{FromStorage: "a", ToStorage: "b"})
	var racing bool
	svc := scopeService(t)
	groupOwners := svc.ScopeOwners
	svc.ScopeOwners = func(ctx context.Context) ([]replscope.Owner, error) {
		o, err := groupOwners(ctx)
		if racing {
			o = append(o, replscope.Owner{Kind: "replication", ID: own.AsString(), Scope: replscope.ReplicationScope("a", "b", nil)})
		}
		return o, err
	}
	racing = true // the replication appears after Validate passed (Validate is a no-op in the fake backend)
	ctx := t.Context()
	_, err := svc.Create(ctx, Group{ID: "g", Members: []string{"a", "b"}, Copies: 2}, false)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "unmanaged replication")
	_, err = svc.Store.Get(ctx, "g")
	require.ErrorIs(t, err, dom.ErrNotFound)
	unlock, err := svc.ScopeLocker.Lock(ctx)
	require.NoError(t, err)
	unlock()
}
