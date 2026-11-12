package tasks

import (
	"context"
	"errors"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func atomicAdmissionSetup(t *testing.T, high int) (*miniredis.Miniredis, *redis.Client, *asynq.Client, *asynq.Inspector, entity.UniversalReplicationID, string) {
	var mr *miniredis.Miniredis
	addr := os.Getenv("TUTTI_TEST_REDIS_ADDR")
	if addr == "" {
		mr = miniredis.RunT(t)
		addr = mr.Addr()
	}
	rc := redis.NewClient(&redis.Options{Addr: addr})
	client := asynq.NewClientFromRedisClient(rc)
	insp := asynq.NewInspectorFromRedisClient(rc)
	t.Cleanup(func() { client.Close(); insp.Close(); rc.Close() })
	id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: "u", FromStorage: "source", FromBucket: "bucket", ToStorage: "dest", ToBucket: "bucket-" + uuid.NewString()})
	q := InitMigrationCopyQueue(id)
	require.NoError(t, rc.Set(t.Context(), "asynq:{"+q+"}:high_watermark", high, 0).Err())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		keys, err := rc.Keys(ctx, "asynq:{"+q+"}:*").Result()
		if err == nil && len(keys) > 0 {
			_ = rc.Del(ctx, keys...).Err()
		}
		_ = rc.SRem(ctx, "asynq:queues", q).Err()
	})
	return mr, rc, client, insp, id, q
}
func atomicAdmissionPayload(id entity.UniversalReplicationID, key string) MigrateObjCopyPayload {
	p := MigrateObjCopyPayload{Bucket: "bucket", Obj: ObjPayload{Name: key}}
	p.SetReplicationID(id, 1)
	return p
}

// Delay the native broker RPC, not a Go-side check whose result can go stale.
type pauseBeforeCommit struct {
	redis.UniversalClient
	once   sync.Once
	before func()
}

func (c *pauseBeforeCommit) wait(keys []string) {
	if len(keys) > 0 && strings.Contains(keys[0], ":t:") {
		c.once.Do(c.before)
	}
}
func (c *pauseBeforeCommit) EvalSha(ctx context.Context, sha string, keys []string, args ...interface{}) *redis.Cmd {
	c.wait(keys)
	return c.UniversalClient.EvalSha(ctx, sha, keys, args...)
}
func (c *pauseBeforeCommit) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	c.wait(keys)
	return c.UniversalClient.Eval(ctx, script, keys, args...)
}
func TestAtomicAdmission_DelayedCommitCannotOverrunWatermark(t *testing.T) {
	mr, rc, client, insp, id, queue := atomicAdmissionSetup(t, 1)
	b := NewQueueService(client, insp)
	wrapped := &pauseBeforeCommit{UniversalClient: rc}
	wrapped.before = func() {
		if mr != nil {
			mr.FastForward(10 * time.Second)
		}
		require.NoError(t, b.EnqueueTask(t.Context(), atomicAdmissionPayload(id, "b")))
	}
	aClient := asynq.NewClientFromRedisClient(wrapped)
	defer aClient.Close()
	a := NewQueueService(aClient, insp)
	err := a.EnqueueTask(t.Context(), atomicAdmissionPayload(id, "a"))
	var retry *dom.ErrRateLimitExceeded
	require.ErrorAs(t, err, &retry)
	st, err := a.Stats(t.Context(), queue)
	require.NoError(t, err)
	require.Equal(t, 1, st.Unprocessed)
	_, err = insp.GetTaskInfo(queue, migrateObjCopy.taskID(atomicAdmissionPayload(id, "a")))
	require.Error(t, err, "rejected task must not be silently created")
	require.NoError(t, b.EnqueueTask(t.Context(), atomicAdmissionPayload(id, "b")), "duplicate accepted task must remain idempotent at full capacity")
}
func TestAtomicAdmission_AllTaskStatesRejectWithoutUniqueLockLeak(t *testing.T) {
	cases := []struct {
		name string
		opts []asynq.Option
	}{
		{"pending", nil}, {"unique", []asynq.Option{asynq.Unique(time.Minute)}}, {"scheduled", []asynq.Option{asynq.ProcessIn(time.Hour)}}, {"scheduled-unique", []asynq.Option{asynq.ProcessIn(time.Hour), asynq.Unique(time.Minute)}}, {"group", []asynq.Option{asynq.Group("g")}}, {"group-unique", []asynq.Option{asynq.Group("g"), asynq.Unique(time.Minute)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, rc, client, insp, _, queue := atomicAdmissionSetup(t, 1)
			_, err := client.EnqueueContext(t.Context(), asynq.NewTask("first", nil), asynq.Queue(queue))
			require.NoError(t, err)
			opts := append([]asynq.Option{asynq.Queue(queue)}, tc.opts...)
			_, err = client.EnqueueContext(t.Context(), asynq.NewTask("rejected", nil), opts...)
			require.ErrorIs(t, err, asynq.ErrQueueFull)
			st, err := NewQueueService(client, insp).Stats(t.Context(), queue)
			require.NoError(t, err)
			require.Equal(t, 1, st.Unprocessed)
			keys, err := rc.Keys(t.Context(), "asynq:{"+queue+"}:unique:*").Result()
			require.NoError(t, err)
			require.Empty(t, keys)
		})
	}
}
func TestAtomicAdmission_CountsExistingScheduledRetryActiveAndGrouped(t *testing.T) {
	for _, state := range []string{"active", "scheduled", "retry", "aggregating"} {
		t.Run(state, func(t *testing.T) {
			_, rc, client, insp, id, queue := atomicAdmissionSetup(t, 1)
			prefix := "asynq:{" + queue + "}:"
			switch state {
			case "active":
				require.NoError(t, rc.LPush(t.Context(), prefix+"active", "existing").Err())
			case "scheduled", "retry":
				require.NoError(t, rc.ZAdd(t.Context(), prefix+state, redis.Z{Score: 1, Member: "existing"}).Err())
			case "aggregating":
				require.NoError(t, rc.SAdd(t.Context(), prefix+"groups", "g").Err())
				require.NoError(t, rc.ZAdd(t.Context(), prefix+"g:g", redis.Z{Score: 1, Member: "existing"}).Err())
			}
			err := NewQueueService(client, insp).EnqueueTask(t.Context(), atomicAdmissionPayload(id, "new"))
			var retry *dom.ErrRateLimitExceeded
			require.ErrorAs(t, err, &retry)
		})
	}
}
func TestAtomicAdmission_ConcurrentClientsRespectTotalBudget(t *testing.T) {
	_, rc, client, insp, id, queue := atomicAdmissionSetup(t, 3)
	other := asynq.NewClientFromRedisClient(rc)
	defer other.Close()
	clients := []*queueService{NewQueueService(client, insp), NewQueueService(other, insp)}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Go(func() { errs <- clients[i%2].EnqueueTask(t.Context(), atomicAdmissionPayload(id, string(rune('a'+i)))) })
	}
	wg.Wait()
	close(errs)
	accepted := 0
	for err := range errs {
		if err == nil {
			accepted++
		} else {
			var retry *dom.ErrRateLimitExceeded
			if !errors.As(err, &retry) {
				t.Fatalf("unexpected error: %v", err)
			}
		}
	}
	require.Equal(t, 3, accepted)
	st, err := clients[0].Stats(t.Context(), queue)
	require.NoError(t, err)
	require.Equal(t, 3, st.Unprocessed)
}

func TestAtomicAdmission_ArchivedRedriveMustWaitForCapacity(t *testing.T) {
	_, _, client, insp, _, queue := atomicAdmissionSetup(t, 1)
	archived, err := client.EnqueueContext(t.Context(), asynq.NewTask("archived", nil), asynq.Queue(queue))
	require.NoError(t, err)
	require.NoError(t, insp.ArchiveTask(queue, archived.ID))
	pending, err := client.EnqueueContext(t.Context(), asynq.NewTask("pending", nil), asynq.Queue(queue))
	require.NoError(t, err)
	require.ErrorIs(t, insp.RunTask(queue, archived.ID), asynq.ErrQueueFull)
	st, err := NewQueueService(client, insp).Stats(t.Context(), queue)
	require.NoError(t, err)
	require.Equal(t, 1, st.Unprocessed)
	require.Equal(t, 1, st.Archived)
	require.NoError(t, insp.DeleteTask(queue, pending.ID))
	require.NoError(t, insp.RunTask(queue, archived.ID))
	st, err = NewQueueService(client, insp).Stats(t.Context(), queue)
	require.NoError(t, err)
	require.Equal(t, 1, st.Unprocessed)
	require.Zero(t, st.Archived)
}
func TestAtomicAdmission_ArchiveBatchRedriveKeepsExcessTasksArchived(t *testing.T) {
	_, _, client, insp, _, queue := atomicAdmissionSetup(t, 2)
	for i := 0; i < 3; i++ {
		task, err := client.EnqueueContext(t.Context(), asynq.NewTask("archived", nil), asynq.Queue(queue))
		require.NoError(t, err)
		require.NoError(t, insp.ArchiveTask(queue, task.ID))
	}
	_, err := client.EnqueueContext(t.Context(), asynq.NewTask("pending", nil), asynq.Queue(queue))
	require.NoError(t, err)
	moved, err := insp.RunAllArchivedTasks(queue)
	require.NoError(t, err)
	require.Equal(t, 1, moved)
	st, err := NewQueueService(client, insp).Stats(t.Context(), queue)
	require.NoError(t, err)
	require.Equal(t, 2, st.Unprocessed)
	require.Equal(t, 2, st.Archived)
	moved, err = insp.RunAllArchivedTasks(queue)
	require.NoError(t, err)
	require.Zero(t, moved, "full queue must not remove archived work")
}
