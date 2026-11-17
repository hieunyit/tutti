// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func TestDeleteEventsCRUD(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Repos().DeleteEvents

	// 1. Create a delete event
	event := controlplane.DeleteEvent{
		ReplicationID:   "user1:src:dst:b1:b2",
		Bucket:          "b1",
		ObjectKey:       "docs/report.pdf",
		DeletionType:    "hard",
		SourceTimestamp: time.Now(),
		Status:          "pending",
	}
	id, err := repo.Create(ctx, event)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// 2. List delete events
	list, total, err := repo.List(ctx, controlplane.DeleteEventFilter{
		ReplicationID: "user1:src:dst:b1:b2",
		Limit:         10,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.Equal(t, "docs/report.pdf", list[0].ObjectKey)
	require.Equal(t, "pending", list[0].Status)

	// 3. Update status to failed
	err = repo.UpdateStatus(ctx, id, "failed", "network timeout")
	require.NoError(t, err)

	list, _, err = repo.List(ctx, controlplane.DeleteEventFilter{
		ReplicationID: "user1:src:dst:b1:b2",
		Status:        "failed",
	})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "failed", list[0].Status)
	require.Equal(t, int32(1), list[0].RetryCount)
	require.Equal(t, "network timeout", list[0].LastError)

	// 4. Retry failed
	claimed, err := repo.EnqueueRetry(ctx, id, 1, func() error { return errors.New("queue unavailable") })
	require.Error(t, err)
	require.False(t, claimed)
	list, _, err = repo.List(ctx, controlplane.DeleteEventFilter{Status: "failed"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	claimed, err = repo.EnqueueRetry(ctx, id, 1, func() error { return nil })
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = repo.EnqueueRetry(ctx, id, 1, func() error { t.Fatal("duplicate enqueue"); return nil })
	require.NoError(t, err)
	require.False(t, claimed)

	list, _, err = repo.List(ctx, controlplane.DeleteEventFilter{
		ReplicationID: "user1:src:dst:b1:b2",
		Status:        "pending",
	})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "pending", list[0].Status)

	// A second event for the same object must be unaffected by the first outcome.
	otherID, err := repo.Create(ctx, event)
	require.NoError(t, err)
	// 5. Record audit log
	err = repo.RecordAudit(ctx, controlplane.DeleteAuditEntry{
		DeleteEventID:         id,
		DeleteEventRetryCount: 1,
		ReplicationID:         "user1:src:dst:b1:b2",
		Bucket:                "b2",
		ObjectKey:             "docs/report.pdf",
		Action:                "deleted",
		DestinationStorage:    "dst",
		Timestamp:             time.Now(),
		Metadata:              map[string]any{"version": "v1"},
	})
	require.NoError(t, err)
	list, _, err = repo.List(ctx, controlplane.DeleteEventFilter{ReplicationID: event.ReplicationID})
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, otherID, list[0].ID)
	require.Equal(t, "pending", list[0].Status)
	require.Equal(t, id, list[1].ID)
	require.Equal(t, "success", list[1].Status)
}

func TestDeleteEventOutcomeIgnoresDuplicateFailure(t *testing.T) {
	repo := newStore(t).Repos().DeleteEvents
	ctx := t.Context()
	event := controlplane.DeleteEvent{ReplicationID: "user:src:dst", Bucket: "bucket", ObjectKey: "key"}
	id, err := repo.Create(ctx, event)
	require.NoError(t, err)
	failed := controlplane.DeleteAuditEntry{
		DeleteEventID: id, ReplicationID: event.ReplicationID, Bucket: event.Bucket, ObjectKey: event.ObjectKey,
		Action: "failed", DestinationStorage: "dst", Metadata: map[string]any{"error": "connection closed"},
	}
	require.NoError(t, repo.RecordAudit(ctx, failed))
	require.NoError(t, repo.RecordAudit(ctx, failed))
	claimed, err := repo.EnqueueRetry(ctx, id, 1, func() error { return nil })
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, repo.RecordAudit(ctx, failed))
	events, _, err := repo.List(ctx, controlplane.DeleteEventFilter{ReplicationID: event.ReplicationID})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "pending", events[0].Status)
	require.EqualValues(t, 1, events[0].RetryCount)
}
