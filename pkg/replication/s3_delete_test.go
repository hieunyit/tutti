package replication

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
)

type delayedDeletePolicies struct {
	policy.Service
	delays map[entity.UniversalReplicationID]int64
}

func (p delayedDeletePolicies) GetReplicationPolicyInfoExtended(_ context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatusExtended, error) {
	return entity.ReplicationStatusExtended{ReplicationStatus: &entity.ReplicationStatus{
		DeletePropagation: "sync", DeleteDelaySeconds: p.delays[id],
	}}, nil
}

func (delayedDeletePolicies) ControlPlane() controlplane.Store { return nil }

type deleteTaskCapture struct {
	tasks.QueueService
	payloads []tasks.ObjectSyncPayload
	runAt    []time.Time
}

func (q *deleteTaskCapture) EnqueueTask(_ context.Context, task any, opts ...asynq.Option) error {
	q.payloads = append(q.payloads, *task.(*tasks.ObjectSyncPayload))
	for _, opt := range opts {
		if opt.Type() == asynq.ProcessAtOpt {
			q.runAt = append(q.runAt, opt.Value().(time.Time))
		}
	}
	return nil
}

func TestReplicatePersistsEachDeleteDeadlineBeforeScheduling(t *testing.T) {
	first := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "u", FromStorage: "main", FromBucket: "bucket", ToStorage: "first", ToBucket: "bucket",
	})
	second := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "u", FromStorage: "main", FromBucket: "bucket", ToStorage: "second", ToBucket: "bucket",
	})
	ctx := xctx.SetReplications(t.Context(), []entity.UniversalReplicationID{first, second})
	versions := meta.NewVersionService(testutil.SetupRedis(t))
	queue := &deleteTaskCapture{}
	policies := delayedDeletePolicies{delays: map[entity.UniversalReplicationID]int64{first: 3600, second: 7200}}
	service := NewS3(queue, versions, policies)
	obj := dom.Object{Bucket: "bucket", Name: "object"}
	require.NoError(t, service.Replicate(ctx, "main", &tasks.ObjectSyncPayload{Object: obj, Deleted: true}))
	require.Len(t, queue.payloads, 2)
	require.Len(t, queue.runAt, 2)
	for i, id := range []entity.UniversalReplicationID{first, second} {
		require.Equal(t, id, queue.payloads[i].ID)
		deadline, err := versions.GetOrSetDeleteDeadline(ctx, id, obj, time.Time{})
		require.NoError(t, err)
		require.WithinDuration(t, time.Now().Add(time.Duration(policies.delays[id])*time.Second), deadline, time.Second)
		require.False(t, queue.runAt[i].Before(deadline))
		require.WithinDuration(t, deadline, queue.runAt[i], time.Second)
	}
}
