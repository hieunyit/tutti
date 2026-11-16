// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/s3users"
	"github.com/hieunyit/tutti/pkg/store"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/stretchr/testify/require"
)

type referencePolicies struct{ s3users.PolicyService }

type referenceCountingCreds struct {
	objstore.CredsService
	bulkCalls, singleCalls int
}

func (c *referenceCountingCreds) AllOverrideUsers() map[string]struct{} {
	c.bulkCalls++
	return c.CredsService.AllOverrideUsers()
}

func (c *referenceCountingCreds) ListUsers(storage string) []string {
	c.singleCalls++
	return c.CredsService.ListUsers(storage)
}

func (referencePolicies) ListUserReplicationsInfo(context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended{
		{User: "u", FromStorage: "main", ToStorage: "archive"}: {},
	}, nil
}

func (referencePolicies) GetUserRoutings(context.Context) (map[string]string, error) {
	return map[string]string{"u": "archive"}, nil
}

func TestRuntimeStorageReferenceIndexIncludesAllFamilies(t *testing.T) {
	e := newLockTestEnv(t, nil)
	ctx := t.Context()
	group, err := json.Marshal(protectiongroups.Group{ID: "g", Members: []string{"main", "archive", "archive"}})
	require.NoError(t, err)
	require.NoError(t, e.rc.HSet(ctx, protectiongroups.GroupsKey, "g", group).Err())
	require.NoError(t, e.rc.HSet(ctx, "p:route_bucket:orphan", "bucket", "archive").Err())
	id, err := store.BucketReplicationPolicyToStringConverter(entity.BucketReplicationPolicy{
		User: "orphan", FromStorage: "main", ToStorage: "archive", FromBucket: "source", ToBucket: "target",
	})
	require.NoError(t, err)
	require.NoError(t, e.rc.SAdd(ctx, "p:repl_bucket_status_index:orphan", id).Err())
	fs := failover.NewStore(e.rc, nil)
	require.NoError(t, fs.SetFailoverPolicy(ctx, failover.Policy{Source: "main", Targets: []string{"archive"}}))
	// Repeated endpoints still produce only one reference per storage.
	require.NoError(t, fs.SaveFailoverEvent(ctx, failover.Event{User: "u", Bucket: "bucket", From: "archive", To: "archive"}))
	_, err = store.NewDiffIDStore(e.rc).AddOp(ctx, struct{}{}, entity.DiffID{Locations: []entity.DiffLocation{
		{Storage: "archive", Bucket: "first"}, {Storage: "archive", Bucket: "second"}, {Storage: "main", Bucket: "first"},
	}}).Get()
	require.NoError(t, err)
	creds := &referenceCountingCreds{CredsService: e.creds}
	load := RuntimeStorageReferenceIndex(referencePolicies{}, creds, e.rc, e.rc)
	refs, err := load(ctx)
	require.NoError(t, err)
	want := []string{
		"protection group g", "user replication u: main → archive", "user routing u",
		"bucket routing orphan/bucket", "bucket replication orphan/source", "failover policy main",
		"failover event u/bucket", "diff report archive/first",
	}
	sort.Strings(want)
	require.Equal(t, want, refs["archive"])
	require.Equal(t, []string{
		"bucket replication orphan/source", "diff report main/first", "failover policy main",
		"protection group g", "user replication u: main → archive",
	}, refs["main"])
	require.NotContains(t, refs, "unknown")
	require.Equal(t, 1, creds.bulkCalls)
	require.Zero(t, creds.singleCalls, "collect credential users once for the whole index")
}

func TestRuntimeStorageListReferencesDoNotCacheDeleteCheck(t *testing.T) {
	e := newLockTestEnv(t, nil)
	ctx := t.Context()
	e.res.runtimeRefs = RuntimeStorageReferences(e.pol, e.creds, e.rc, e.rc)
	e.res.runtimeRefsBatch = RuntimeStorageReferenceIndex(e.pol, e.creds, e.rc, e.rc)
	_, err := e.res.ListRuntimeStorages(ctx, nil)
	require.NoError(t, err)
	// A reference written after a list must block the next delete request.
	require.NoError(t, e.rc.HSet(ctx, "p:route_bucket:new-user", "new-bucket", "archive").Err())
	_, err = e.res.DeleteRuntimeStorage(ctx, &pb.RuntimeStorageName{Name: "archive"})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "bucket routing new-user/new-bucket")
	_, exists := e.creds.GetManaged("archive")
	require.True(t, exists)

	_, err = e.res.ListRuntimeStorages(ctx, nil)
	require.NoError(t, err)
	// Removing a reference also takes effect immediately, after a populated list.
	require.NoError(t, e.rc.Del(ctx, "p:route_bucket:new-user").Err())
	_, err = e.res.DeleteRuntimeStorage(ctx, &pb.RuntimeStorageName{Name: "archive"})
	require.NoError(t, err)
}
