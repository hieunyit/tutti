// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
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
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/replscope"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

// rsQueue is a queue stub whose queue stats the test controls.
type rsQueue struct {
	slQueue
	smu         sync.Mutex
	unprocessed map[string]int
	paused      map[string]bool
	enqueued    []any
}

func (q *rsQueue) EnqueueTask(_ context.Context, task any, _ ...asynq.Option) error {
	q.smu.Lock()
	defer q.smu.Unlock()
	q.enqueued = append(q.enqueued, task)
	return nil
}

func (q *rsQueue) Stats(_ context.Context, queue string) (*tasks.QueueStats, error) {
	q.smu.Lock()
	defer q.smu.Unlock()
	return &tasks.QueueStats{Unprocessed: q.unprocessed[queue], Paused: q.paused[queue]}, nil
}

func (q *rsQueue) set(queue string, unprocessed int, paused bool) {
	q.smu.Lock()
	defer q.smu.Unlock()
	q.unprocessed[queue] = unprocessed
	q.paused[queue] = paused
}

func (q *rsQueue) sent() []any {
	q.smu.Lock()
	defer q.smu.Unlock()
	return append([]any(nil), q.enqueued...)
}

type rsEnv struct {
	rc     *redis.Client
	queue  *rsQueue
	client pb.PolicyClient
}

func newRescanEnv(t *testing.T) *rsEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	q := &rsQueue{slQueue: slQueue{rc: rc}, unprocessed: map[string]int{}, paused: map[string]bool{}}
	policySvc := policy.NewService(rc, q, "main")
	h := PolicyHandlers(slCreds{conn: allConn}, slClients{buckets: []string{"bkt"}}, q, policySvc, meta.NewVersionService(rc),
		store.NewMigrationObjectListStateStore(rc), store.NewMigrationBucketListStateStore(rc), nil,
		store.NewReplicationStatusLocker(rc, time.Second), store.NewUserLocker(rc, time.Second), nil,
		WithPolicyScopeLocker(replscope.NewRedisLocker(rc)))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrorInterceptor()))
	pb.RegisterPolicyServer(srv, h)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &rsEnv{rc: rc, queue: q, client: pb.NewPolicyClient(conn)}
}

func rsID(to string) *pb.ReplicationID {
	return &pb.ReplicationID{FromStorage: "main", ToStorage: to, FromBucket: ptr("bkt"), ToBucket: ptr("bkt")}
}

func (e *rsEnv) get(t *testing.T, to string) *pb.Replication {
	t.Helper()
	r, err := e.client.GetReplication(t.Context(), rsID(to))
	require.NoError(t, err)
	return r
}

func TestAddReplicationRescanIntervalValidation(t *testing.T) {
	e := newRescanEnv(t)
	add := func(to string, d *durationpb.Duration) error {
		_, err := e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: rsID(to), Opts: &pb.ReplicationOpts{RescanInterval: d}})
		return err
	}
	for name, d := range map[string]time.Duration{
		"below the minimum": 14*time.Minute + 59*time.Second,
		"above the maximum": 24*time.Hour + time.Second,
		"negative":          -time.Hour,
		"one second":        time.Second,
	} {
		require.Equal(t, codes.InvalidArgument, status.Code(add("a", durationpb.New(d))), name)
	}
	require.Equal(t, codes.InvalidArgument, status.Code(add("a", &durationpb.Duration{Seconds: 1, Nanos: -5})), "inconsistent duration")

	require.NoError(t, add("a", durationpb.New(15*time.Minute)))
	require.NoError(t, add("b", durationpb.New(24*time.Hour)))
	require.NoError(t, add("c", nil))

	require.Equal(t, 15*time.Minute, e.get(t, "a").RescanInterval.AsDuration())
	require.Equal(t, 24*time.Hour, e.get(t, "b").RescanInterval.AsDuration())
	require.Nil(t, e.get(t, "c").RescanInterval, "unset when off")
}

func TestUpdateReplicationRescanInterval(t *testing.T) {
	e := newRescanEnv(t)
	_, err := e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: rsID("a")})
	require.NoError(t, err)
	upd := func(d *durationpb.Duration) error {
		_, err := e.client.UpdateReplicationRescanInterval(t.Context(), &pb.UpdateReplicationRescanIntervalRequest{Id: rsID("a"), RescanInterval: d})
		return err
	}
	require.Equal(t, codes.InvalidArgument, status.Code(upd(durationpb.New(time.Minute))))
	require.Equal(t, codes.InvalidArgument, status.Code(upd(durationpb.New(48*time.Hour))))
	require.Equal(t, codes.InvalidArgument, status.Code(upd(durationpb.New(-time.Hour))))
	require.NoError(t, upd(durationpb.New(time.Hour)))
	require.Equal(t, time.Hour, e.get(t, "a").RescanInterval.AsDuration())
	require.NoError(t, upd(nil), "unset turns it off")
	require.Nil(t, e.get(t, "a").RescanInterval)
	require.NoError(t, upd(durationpb.New(30*time.Minute)))
	require.NoError(t, upd(durationpb.New(0)), "0 turns it off")
	require.Nil(t, e.get(t, "a").RescanInterval)

	_, err = e.client.UpdateReplicationRescanInterval(t.Context(), &pb.UpdateReplicationRescanIntervalRequest{Id: rsID("zzz"), RescanInterval: durationpb.New(time.Hour)})
	require.Equal(t, codes.NotFound, status.Code(err))

	// other attributes are untouched
	_, err = e.client.UpdateReplicationName(t.Context(), &pb.UpdateReplicationNameRequest{Id: rsID("a"), Name: "n"})
	require.NoError(t, err)
	require.NoError(t, upd(durationpb.New(time.Hour)))
	r := e.get(t, "a")
	require.Equal(t, "n", r.Name)
	require.Equal(t, time.Hour, r.RescanInterval.AsDuration())
}

func TestRescanReplicationErrors(t *testing.T) {
	e := newRescanEnv(t)
	_, err := e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: rsID("a"), Opts: &pb.ReplicationOpts{RescanInterval: durationpb.New(time.Hour)}})
	require.NoError(t, err)
	id := entityID(t, rsID("a"))
	rescan := func() error { _, err := e.client.RescanReplication(t.Context(), rsID("a")); return err }
	count := func() int {
		n := 0
		for _, task := range e.queue.sent() {
			if _, ok := task.(*tasks.RescanListObjectsPayload); ok {
				n++
			}
		}
		return n
	}

	// the initial copy is not finished
	e.queue.set(tasks.InitMigrationCopyQueue(id), 5, false)
	err = rescan()
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, reasonOf(err), "initial copy is not finished")
	r := e.get(t, "a")
	require.False(t, r.IsInitDone)
	require.Nil(t, r.NextRescanAt, "no next scan before the initial copy is done")

	// initial copy done, but paused
	e.queue.set(tasks.InitMigrationCopyQueue(id), 0, false)
	e.queue.set(tasks.InitMigrationListQueue(id), 0, true)
	err = rescan()
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, reasonOf(err), "paused")
	require.Nil(t, e.get(t, "a").NextRescanAt, "no next scan while paused")
	require.Zero(t, count())

	// resumed: works, and a second request while it runs is refused
	e.queue.set(tasks.InitMigrationListQueue(id), 0, false)
	require.NotNil(t, e.get(t, "a").NextRescanAt)
	require.NoError(t, rescan())
	require.Equal(t, 1, count())
	err = rescan()
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, reasonOf(err), "already running")
	require.Equal(t, 1, count())
	r = e.get(t, "a")
	require.True(t, r.RescanRunning)
	require.Nil(t, r.NextRescanAt, "no next scan while one runs")
	require.True(t, r.IsInitDone, "a scan never flips the replication back to initial copy")
	require.EqualValues(t, 0, r.InitObjListed)
	require.EqualValues(t, 0, r.InitObjDone)

	// unknown replication
	_, err = e.client.RescanReplication(t.Context(), rsID("zzz"))
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestRescanReportedStateAndDeleteCleanup(t *testing.T) {
	e := newRescanEnv(t)
	_, err := e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: rsID("a"), Opts: &pb.ReplicationOpts{RescanInterval: durationpb.New(time.Hour)}})
	require.NoError(t, err)
	id := entityID(t, rsID("a"))
	rs := store.NewRescanStore(e.rc)
	start := time.Now().Add(-10 * time.Minute).Truncate(time.Millisecond)
	ok, err := rs.Start(t.Context(), id.AsString(), 1, "r1", start, time.Time{})
	require.NoError(t, err)
	require.True(t, ok)
	_, err = rs.Listed(t.Context(), id.AsString(), 1, "r1", 3, true, "p", start)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = rs.ObjectDone(t.Context(), id.AsString(), 1, "r1", i < 2, start.Add(time.Minute))
		require.NoError(t, err)
	}
	r := e.get(t, "a")
	require.False(t, r.RescanRunning)
	require.True(t, start.Equal(r.LastRescanAt.AsTime()))
	require.EqualValues(t, 2, r.LastRescanObjects)
	require.Empty(t, r.LastRescanError)
	require.WithinDuration(t, start.Add(time.Hour), r.NextRescanAt.AsTime(), time.Second)

	keys, _ := e.rc.Keys(t.Context(), "p:repl_rescan:*").Result()
	require.NotEmpty(t, keys)
	_, err = e.client.DeleteReplication(t.Context(), rsID("a"))
	require.NoError(t, err)
	keys, _ = e.rc.Keys(t.Context(), "p:repl_rescan:*").Result()
	require.Empty(t, keys, "deleting the replication removes its scan state")

	// re-created: starts clean
	_, err = e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: rsID("a"), Opts: &pb.ReplicationOpts{RescanInterval: durationpb.New(time.Hour)}})
	require.NoError(t, err)
	r = e.get(t, "a")
	require.Nil(t, r.LastRescanAt)
	require.EqualValues(t, 0, r.LastRescanObjects)
}

func TestEnablingRescanRequestsCatchUpScan(t *testing.T) {
	e := newRescanEnv(t)
	_, err := e.client.AddReplication(t.Context(), &pb.AddReplicationRequest{Id: rsID("a")})
	require.NoError(t, err)
	id := entityID(t, rsID("a"))
	_, err = e.client.UpdateReplicationRescanInterval(t.Context(), &pb.UpdateReplicationRescanIntervalRequest{Id: rsID("a"), RescanInterval: durationpb.New(time.Hour)})
	require.NoError(t, err)
	st, err := store.NewRescanStore(e.rc).Get(t.Context(), id.AsString(), 1, time.Now())
	require.NoError(t, err)
	require.True(t, st.Full, "turning rescan on later scans the whole source once")
}

func entityID(t *testing.T, id *pb.ReplicationID) entity.UniversalReplicationID {
	t.Helper()
	uid, err := pbToReplicationID(id)
	require.NoError(t, err)
	return uid
}

// reasonOf returns the human readable reason the error interceptor attaches as ErrorInfo.
func reasonOf(err error) string {
	for _, d := range status.Convert(err).Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.Reason
		}
	}
	return err.Error()
}
