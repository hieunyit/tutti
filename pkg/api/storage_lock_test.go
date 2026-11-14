// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

func TestStorageLockerSerializesAndOrders(t *testing.T) {
	rc := testutil.SetupRedis(t)
	l := NewStorageLocker(rc)
	var inside atomic.Int32
	var maxInside atomic.Int32
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			// opposite name order must not deadlock
			names := []string{"a", "b"}
			if i%2 == 1 {
				names = []string{"b", "a"}
			}
			done <- l.With(t.Context(), names, func(context.Context) error {
				n := inside.Add(1)
				if n > maxInside.Load() {
					maxInside.Store(n)
				}
				time.Sleep(20 * time.Millisecond)
				inside.Add(-1)
				return nil
			})
		}()
	}
	for i := 0; i < 4; i++ {
		require.NoError(t, <-done)
	}
	require.EqualValues(t, 1, maxInside.Load())
	// nil locker runs fn directly
	var nilLocker *StorageLocker
	require.NoError(t, nilLocker.With(t.Context(), []string{"x"}, func(context.Context) error { return nil }))
}

type lockTestEnv struct {
	rc     redis.UniversalClient
	creds  RuntimeStorages
	res    *resilienceHandlers
	policy pb.PolicyServer
	pol    policy.Service
}

func newLockTestEnv(t *testing.T, refs StorageReferences) *lockTestEnv {
	t.Helper()
	rc := testutil.SetupRedis(t)
	cfg := managedConf()
	cs, err := objstore.NewCredsSvc(t.Context(), cfg, rc, objstore.WithManagedStorages(rc))
	require.NoError(t, err)
	var creds RuntimeStorages = cs
	_, err = creds.CreateManaged(t.Context(), objstore.ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: "http://localhost:9001", Provider: s3.ProviderMinIO}})
	require.NoError(t, err)
	require.NoError(t, creds.SetS3Credentials(t.Context(), "archive", "u", s3.CredentialsV4{AccessKeyID: "k", SecretAccessKey: "s"}))
	require.NoError(t, creds.SetS3Credentials(t.Context(), "main", "u", s3.CredentialsV4{AccessKeyID: "k", SecretAccessKey: "s"}))
	pol := policy.NewService(rc, &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}, "main")
	locker := NewStorageLocker(rc)
	if refs == nil {
		refs = func(ctx context.Context, name string) ([]string, error) {
			routes, err := pol.GetUserRoutings(ctx)
			if err != nil {
				return nil, err
			}
			var out []string
			for u, to := range routes {
				if to == name {
					out = append(out, "user routing "+u)
				}
			}
			return out, nil
		}
	}
	res := ResilienceHandlers(rc, nil, WithRuntimeStorages(creds, refs), WithStorageLocker(locker)).(*resilienceHandlers)
	ph := PolicyHandlers(creds, nil, nil, pol, nil, nil, nil, nil, nil, store.NewUserLocker(rc, time.Second), nil, WithPolicyStorageLocker(locker))
	return &lockTestEnv{rc: rc, creds: creds, res: res, policy: ph, pol: pol}
}

func managedConf() *objstore.Config {
	return &objstore.Config{Main: "main", Storages: map[string]objstore.Storage{"main": {CommonConfig: objstore.CommonConfig{Type: dom.S3}, S3: &s3.Storage{StorageAddress: s3.StorageAddress{Address: "http://localhost:9000", Provider: s3.ProviderMinIO}}}}, DynamicCredentials: objstore.DynamicCredentialsConfig{Enabled: true, MasterPassword: "lock-test-password", PollInterval: time.Hour}}
}

// A routing created while the storage is being deleted waits for the delete and then fails: no dangling reference.
func TestConcurrentRoutingCreateDuringDelete(t *testing.T) {
	inRefs, release := make(chan struct{}), make(chan struct{})
	e := newLockTestEnv(t, func(ctx context.Context, name string) ([]string, error) {
		close(inRefs)
		<-release
		return nil, nil // nothing references the storage at check time
	})
	delDone := make(chan error, 1)
	go func() {
		_, err := e.res.DeleteRuntimeStorage(t.Context(), &pb.RuntimeStorageName{Name: "archive"})
		delDone <- err
	}()
	<-inRefs
	addDone := make(chan error, 1)
	go func() {
		_, err := e.policy.AddRouting(t.Context(), &pb.AddRoutingRequest{User: "u", ToStorage: "archive"})
		addDone <- err
	}()
	select {
	case err := <-addDone:
		t.Fatalf("routing creation must wait for the in-flight delete, returned %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-delDone)
	require.Error(t, <-addDone, "the storage is gone: the routing must not be created")
	routes, err := e.pol.GetUserRoutings(t.Context())
	require.NoError(t, err)
	require.NotContains(t, routes, "u")
}

// A routing created before the delete is seen by the reference check under the lock.
func TestRoutingCreatedBeforeDeleteBlocksDelete(t *testing.T) {
	e := newLockTestEnv(t, nil)
	_, err := e.policy.AddRouting(t.Context(), &pb.AddRoutingRequest{User: "u", ToStorage: "archive"})
	require.NoError(t, err)
	_, err = e.res.DeleteRuntimeStorage(t.Context(), &pb.RuntimeStorageName{Name: "archive"})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.True(t, strings.Contains(err.Error(), "user routing u"))
	_, exists := e.creds.GetManaged("archive")
	require.True(t, exists)
}

// Concurrent creates and deletes never leave a routing pointing at a deleted storage.
func TestConcurrentRoutingAndDeleteNeverDangles(t *testing.T) {
	for i := 0; i < 5; i++ {
		e := newLockTestEnv(t, nil)
		done := make(chan struct{}, 2)
		go func() {
			_, _ = e.policy.AddRouting(t.Context(), &pb.AddRoutingRequest{User: "u", ToStorage: "archive"})
			done <- struct{}{}
		}()
		go func() {
			_, _ = e.res.DeleteRuntimeStorage(t.Context(), &pb.RuntimeStorageName{Name: "archive"})
			done <- struct{}{}
		}()
		<-done
		<-done
		routes, _ := e.pol.GetUserRoutings(t.Context())
		_, exists := e.creds.GetManaged("archive")
		if routes["u"] == "archive" {
			require.True(t, exists, fmt.Sprintf("round %d: routing to a deleted storage", i))
		}
	}
}

// The reference scan reads many users in pipelined batches and returns the same references as before.
func TestStorageReferencesManyUsersPipelined(t *testing.T) {
	rc := testutil.SetupRedis(t)
	creds, err := objstore.NewCredsSvc(t.Context(), managedConf(), rc)
	require.NoError(t, err)
	pol := policy.NewService(rc, &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}, "main")
	const users = 450
	var want []string
	for i := 0; i < users; i++ {
		u := fmt.Sprintf("user%03d", i)
		require.NoError(t, rc.HSet(t.Context(), "p:route_bucket:"+u, "b1", "main", "b2", "archive").Err())
		want = append(want, "bucket routing "+u+"/b2")
		if i%50 == 0 {
			id, err := store.BucketReplicationPolicyToStringConverter(entity.BucketReplicationPolicy{User: u, FromStorage: "main", ToStorage: "archive", FromBucket: "src", ToBucket: "src"})
			require.NoError(t, err)
			require.NoError(t, rc.SAdd(t.Context(), "p:repl_bucket_status_index:"+u, id).Err())
			want = append(want, "bucket replication "+u+"/src")
		}
	}
	var pipelines atomic.Int32
	c := rc.(*redis.Client)
	c.AddHook(pipelineCounter{&pipelines})
	refs, err := RuntimeStorageReferences(pol, creds, rc, rc)(t.Context(), "archive")
	require.NoError(t, err)
	require.ElementsMatch(t, want, refs)
	// 450 users: 3 batches of 200 at most plus the user scan, instead of 2 round trips per user
	require.LessOrEqual(t, int(pipelines.Load()), 5)
	none, err := RuntimeStorageReferences(pol, creds, rc, rc)(t.Context(), "nothing")
	require.NoError(t, err)
	require.Empty(t, none)
}

type pipelineCounter struct{ n *atomic.Int32 }

func (pipelineCounter) DialHook(next redis.DialHook) redis.DialHook { return next }
func (p pipelineCounter) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return next
}
func (p pipelineCounter) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		p.n.Add(1)
		return next(ctx, cmds)
	}
}
