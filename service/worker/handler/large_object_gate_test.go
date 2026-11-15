package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// The gate runs before anything else: with no free stream slot a large copy returns a rate-limit style error
// (retried later, not counted as a failure) and touches no dependency (the zero svc would panic otherwise).
func TestMigrationObjCopyLargeObjectGate(t *testing.T) {
	t.Cleanup(func() { systemsettings.ApplySettings(controlplane.SystemSettings{}) })
	systemsettings.ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{LargeObjectStreams: 1}})
	id := entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("u", "from", "to"))
	payload := tasks.MigrateObjCopyPayload{}
	payload.SetReplicationID(id, 1)
	payload.Bucket = "b"
	payload.Obj = tasks.ObjPayload{Name: "k", Size: systemsettings.LargeObjectThreshold}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	hold, ok := systemsettings.TryAcquireLargeObject("to", systemsettings.LargeObjectThreshold)
	require.True(t, ok)
	defer hold()

	s := &svc{}
	err = s.handleMigrationObjCopy(context.Background(), asynq.NewTask(tasks.TypeMigrateObjCopy, raw))
	var rl *dom.ErrRateLimitExceeded
	require.ErrorAs(t, err, &rl)
	require.Greater(t, rl.RetryIn.Seconds(), 0.0)
}
