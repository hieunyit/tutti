// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hibiken/asynq"

	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/service/worker/syncer"
)

func terminalTaskError(ctx context.Context, err error) bool {
	if errors.Is(err, asynq.RevokeTask) {
		return false
	}
	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	return errors.Is(err, asynq.SkipRetry) || (isTaskFailure(err) && retried >= maxRetry)
}

func recordFailedDeleteTask(ctx context.Context, record syncer.DeleteAuditRecorder, task *asynq.Task, err error) error {
	if record == nil || task.Type() != tasks.TypeObjectSync || !terminalTaskError(ctx, err) {
		return nil
	}
	var p tasks.ObjectSyncPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}
	if !p.Deleted {
		return nil
	}
	return syncer.RecordDeleteAudit(ctx, record, p, "failed", map[string]any{"error": err.Error()})
}
