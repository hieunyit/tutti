package api

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	pg "github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/replscope"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

// slCreds: storages with a connection account serve the empty user; no S3 users are configured.
type slCreds struct {
	objstore.CredsService
	conn map[string]bool
}

func (c slCreds) HasUser(storage, user string) error {
	if user == "" {
		if c.conn[storage] {
			return nil
		}
		return fmt.Errorf("%w: storage %q has no connection account", dom.ErrFailedPrecondition, storage)
	}
	return dom.ErrNotFound
}

func (c slCreds) ValidateReplicationID(id entity.UniversalReplicationID) error {
	for _, st := range []string{id.FromStorage(), id.ToStorage()} {
		if err := c.HasUser(st, id.User()); err != nil {
			return err
		}
	}
	return id.Validate()
}

func (c slCreds) Storages() map[string]dom.StorageType {
	return map[string]dom.StorageType{"main": dom.S3, "a": dom.S3, "b": dom.S3, "c": dom.S3}
}

// slQueue is a goroutine-safe queue stub that counts enqueued tasks.
type slQueue struct {
	tasks.QueueService
	mu       sync.Mutex
	enqueued int
}

func (q *slQueue) EnqueueTask(context.Context, any, ...asynq.Option) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enqueued++
	return nil
}
func (q *slQueue) Stop(context.Context, ...string) error  { return nil }
func (q *slQueue) Purge(context.Context, ...string) error { return nil }
func (q *slQueue) count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enqueued
}

type slCommon struct {
	objstore.Common
	buckets  []string
	onExists func(ctx context.Context, bucket string)
}

func (c slCommon) ListBuckets(context.Context) ([]string, error) { return c.buckets, nil }

type slClients struct {
	objstore.Clients
	buckets  []string
	onExists func(ctx context.Context, bucket string)
}

func (c slClients) AsCommon(_ context.Context, _, _ string) (objstore.Common, error) {
	return slCommon{buckets: c.buckets, onExists: c.onExists}, nil
}

type slEnv struct {
	rc     *redis.Client
	policy policy.Service
	queue  *slQueue
	client pb.PolicyClient
}

func newSLEnv(t *testing.T, conn map[string]bool, buckets ...string) *slEnv {
	t.Helper()
	return newSLEnvHook(t, conn, nil, buckets...)
}

func newSLEnvHook(t *testing.T, conn map[string]bool, onExists func(context.Context, string), buckets ...string) *slEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	q := &slQueue{}
	policySvc := policy.NewService(rc, q, "main")
	h := PolicyHandlers(slCreds{conn: conn}, slClients{buckets: buckets, onExists: onExists}, q, policySvc, meta.NewVersionService(rc),
		store.NewMigrationObjectListStateStore(rc), store.NewMigrationBucketListStateStore(rc), nil,
		store.NewReplicationStatusLocker(rc, time.Second), store.NewUserLocker(rc, time.Second), nil,
		WithPolicyScopeLocker(replscope.NewRedisLocker(rc)))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrorInterceptor()))
	pb.RegisterPolicyServer(srv, h)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn2, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn2.Close() })
	return &slEnv{rc: rc, policy: policySvc, queue: q, client: pb.NewPolicyClient(conn2)}
}

func (e *slEnv) add(t *testing.T, from, to string, buckets ...string) error {
	t.Helper()
	return e.addCtx(t, from, to, buckets...)
}

func (e *slEnv) addCtx(t *testing.T, from, to string, buckets ...string) error {
	id := &pb.ReplicationID{FromStorage: from, ToStorage: to}
	if len(buckets) > 0 {
		id.FromBucket, id.ToBucket = &buckets[0], &buckets[0]
	}
	_, err := e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: id})
	return err
}

func (e *slEnv) owners(t *testing.T) []replscope.Owner {
	o, err := e.policy.ListScopeOwners(t.Context())
	require.NoError(t, err)
	return o
}

var allConn = map[string]bool{"main": true, "a": true, "b": true, "c": true}

func TestAddReplicationEmptyUserOK(t *testing.T) {
	e := newSLEnv(t, allConn)
	require.NoError(t, e.add(t, "main", "a"))
	require.Equal(t, 1, e.queue.count())
	owners := e.owners(t)
	require.Len(t, owners, 1)
	res, err := e.client.ListReplications(t.Context(), &pb.ListReplicationsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Replications, 1)
	require.Equal(t, "", res.Replications[0].Id.User)
	got, err := e.client.GetReplication(t.Context(), res.Replications[0].Id)
	require.NoError(t, err)
	require.Equal(t, "", got.Id.User)
	// bucket-level storage replication lists too
	require.NoError(t, e.add(t, "main", "b", "photos"))
	res, err = e.client.ListReplications(t.Context(), &pb.ListReplicationsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Replications, 2)
	for _, r := range res.Replications {
		require.Equal(t, "", r.Id.User)
	}
}

func TestAddReplicationEmptyUserNoConnectionAccountNamesStorage(t *testing.T) {
	e := newSLEnv(t, map[string]bool{"main": true})
	err := e.add(t, "main", "a")
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, reason(err), `"a"`)
	require.Contains(t, reason(err), "connection account")
	require.Equal(t, 0, e.queue.count())
	require.Empty(t, e.owners(t))
}

func TestAddReplicationConflictSameDestination(t *testing.T) {
	e := newSLEnv(t, allConn)
	require.NoError(t, e.add(t, "main", "a"))
	err := e.add(t, "b", "a")
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Contains(t, reason(err), "is already covered by replication")
	require.Equal(t, 1, e.queue.count())
	require.Len(t, e.owners(t), 1)
	// bucket-level over the covered edge is refused as well
	err = e.add(t, "c", "a", "photos")
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Contains(t, reason(err), `"photos"`)
	require.Len(t, e.owners(t), 1)
	// the scope lock is released after a conflict
	unlock, lerr := replscope.NewRedisLocker(e.rc).Lock(t.Context())
	require.NoError(t, lerr)
	unlock()
}

func TestAddReplicationAllowedFanOut(t *testing.T) {
	e := newSLEnv(t, allConn)
	require.NoError(t, e.add(t, "main", "a", "photos"))
	require.NoError(t, e.add(t, "main", "b", "photos"))
	require.Len(t, e.owners(t), 2)
}

func TestAddReplicationReverseRejected(t *testing.T) {
	e := newSLEnv(t, allConn)
	require.NoError(t, e.add(t, "main", "a"))
	err := e.add(t, "a", "main")
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Len(t, e.owners(t), 1)
}

func TestConcurrentAddOnlyOneWins(t *testing.T) {
	e := newSLEnv(t, allConn)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, from := range []string{"main", "b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = e.addCtx(t, from, "a", "photos")
		}()
	}
	wg.Wait()
	ok, exists := 0, 0
	for _, err := range errs {
		switch status.Code(err) {
		case codes.OK:
			ok++
		case codes.AlreadyExists:
			exists++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	require.Equal(t, 1, ok)
	require.Equal(t, 1, exists)
	require.Len(t, e.owners(t), 1)
}

func TestAvailableBucketsMarksCoveredByGroup(t *testing.T) {
	e := newSLEnv(t, allConn, "photos", "docs", "logs")
	// a protection group over {a,b} for bucket photos covers a->b and b->a
	g := pg.Group{ID: "g1", Members: []string{"a", "b"}, Buckets: []string{"photos"}, Copies: 2}
	require.NoError(t, pg.NewStore(e.rc).Save(t.Context(), &g))
	// a user-less bucket replication of docs on main->c
	require.NoError(t, e.add(t, "main", "c", "docs"))

	res, err := e.client.AvailableBuckets(t.Context(), &pb.AvailableBucketsRequest{FromStorage: "main", ToStorage: "b"})
	require.NoError(t, err)
	// main->b shares its destination with the group edge a->b: photos is covered; docs (main->c) is another destination
	require.ElementsMatch(t, []string{"docs", "logs"}, res.Buckets)
	require.Len(t, res.CoveredBuckets, 1)
	require.Equal(t, "photos", res.CoveredBuckets[0].Bucket)

	res, err = e.client.AvailableBuckets(t.Context(), &pb.AvailableBucketsRequest{FromStorage: "c", ToStorage: "main"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"photos", "logs"}, res.Buckets, "c->main reverses main->c only for docs")
	require.Len(t, res.CoveredBuckets, 1)
	require.Equal(t, "docs", res.CoveredBuckets[0].Bucket)

	res, err = e.client.AvailableBuckets(t.Context(), &pb.AvailableBucketsRequest{FromStorage: "a", ToStorage: "b"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"docs", "logs"}, res.Buckets)
	require.Len(t, res.CoveredBuckets, 1)
	require.Equal(t, "photos", res.CoveredBuckets[0].Bucket)
	require.Equal(t, "group", res.CoveredBuckets[0].OwnerKind)
	require.Equal(t, "g1", res.CoveredBuckets[0].OwnerId)

	// same destination as the replication: docs is covered by the replication
	res, err = e.client.AvailableBuckets(t.Context(), &pb.AvailableBucketsRequest{FromStorage: "b", ToStorage: "c"})
	require.NoError(t, err)
	require.NotContains(t, res.Buckets, "docs")
	require.Len(t, res.CoveredBuckets, 1)
	require.Equal(t, "docs", res.CoveredBuckets[0].Bucket)
	require.Equal(t, "replication", res.CoveredBuckets[0].OwnerKind)

	// the same edge keeps its replicated_buckets meaning
	res, err = e.client.AvailableBuckets(t.Context(), &pb.AvailableBucketsRequest{FromStorage: "main", ToStorage: "c", ShowReplicated: true})
	require.NoError(t, err)
	require.Equal(t, []string{"docs"}, res.ReplicatedBuckets)
	require.NotContains(t, res.Buckets, "docs")
	require.Empty(t, res.CoveredBuckets)

	// a storage without connection account is refused naming it
	e2 := newSLEnv(t, map[string]bool{"main": true}, "x")
	_, err = e2.client.AvailableBuckets(t.Context(), &pb.AvailableBucketsRequest{FromStorage: "main", ToStorage: "a"})
	require.Error(t, err)
	require.Contains(t, reason(err), `"a"`)
}

func TestGroupCreateConflictWithReplication(t *testing.T) {
	e := newSLEnv(t, allConn)
	require.NoError(t, e.add(t, "b", "a", "photos"))

	svc := &pg.Service{Store: pg.NewStore(e.rc), Backend: slBackend{}, ScopeOwners: e.policy.ListScopeOwners, ScopeLocker: replscope.NewRedisLocker(e.rc)}
	_, err := svc.Create(t.Context(), pg.Group{ID: "g", Members: []string{"a", "c"}, Buckets: []string{"photos"}, Copies: 2}, false)
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
	require.Contains(t, err.Error(), "replication")
	require.Contains(t, err.Error(), `"photos"`)
	_, err = svc.Store.Get(t.Context(), "g")
	require.ErrorIs(t, err, dom.ErrNotFound)
}

func (c slCreds) MainStorage() string       { return "main" }
func (c slCreds) ListUsers(string) []string { return nil }

func (*slQueue) Stats(context.Context, string) (*tasks.QueueStats, error) {
	return &tasks.QueueStats{}, nil
}
func (*slQueue) IsPaused(context.Context, string) (bool, error) { return false, nil }
func (*slQueue) UnprocessedCount(context.Context, bool, ...string) (int, error) {
	return 0, nil
}

func (c slCommon) BucketExists(ctx context.Context, bucket string) (bool, error) {
	if c.onExists != nil {
		c.onExists(ctx, bucket)
	}
	return true, nil
}

// reason returns the error text the API puts into the ErrorInfo detail.
func reason(err error) string {
	for _, d := range status.Convert(err).Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.Reason
		}
	}
	return status.Convert(err).Message()
}

type slBackend struct{ pg.Backend }

func (slBackend) Validate(context.Context, pg.Group, bool) error { return nil }

// TestScopeLockNotHeldDuringBucketExists proves the network call BucketExists runs before the scope lock: while one
// AddReplication is blocked in it, another AddReplication on a different bucket completes.
func TestScopeLockNotHeldDuringBucketExists(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	e := newSLEnvHook(t, allConn, func(_ context.Context, bucket string) {
		if bucket == "slow" {
			close(entered)
			<-release
		}
	})
	slowDone := make(chan error, 1)
	go func() { slowDone <- e.add(t, "main", "a", "slow") }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("slow AddReplication did not reach BucketExists")
	}
	// the scope lock must be free now
	unlock, err := replscope.NewRedisLocker(e.rc).Lock(t.Context())
	require.NoError(t, err)
	unlock()
	fastDone := make(chan error, 1)
	go func() { fastDone <- e.add(t, "main", "b", "fast") }()
	select {
	case err := <-fastDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("second AddReplication blocked while the first is in BucketExists: scope lock is held")
	}
	close(release)
	require.NoError(t, <-slowDone)
	require.Len(t, e.owners(t), 2)
}
