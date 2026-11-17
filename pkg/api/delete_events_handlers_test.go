// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/policy"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

type mockDeleteEventsPolicyService struct {
	policy.Service
	events []controlplane.DeleteEvent
}

func (m *mockDeleteEventsPolicyService) ListDeleteEvents(ctx context.Context, replicationID, status string, limit int32, offset int64) ([]controlplane.DeleteEvent, int64, error) {
	return m.events, int64(len(m.events)), nil
}

func (m *mockDeleteEventsPolicyService) GetReplicationPolicyInfoExtended(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatusExtended, error) {
	if id.User() == "nonexistent" {
		return entity.ReplicationStatusExtended{}, dom.ErrNotFound
	}
	return entity.ReplicationStatusExtended{}, nil
}

func (m *mockDeleteEventsPolicyService) RetryFailedDeletes(ctx context.Context, replicationID string) (int64, error) {
	return 1, nil
}

func TestListDeleteEvents_Validation(t *testing.T) {
	ctx := context.Background()
	mockSvc := &mockDeleteEventsPolicyService{}
	h := &policyHandlers{policySvc: mockSvc}

	t.Run("negative offset fails", func(t *testing.T) {
		_, err := h.ListDeleteEvents(ctx, &pb.ListDeleteEventsRequest{Offset: -1})
		require.ErrorIs(t, err, dom.ErrInvalidArg)
	})

	t.Run("invalid replication_id format fails", func(t *testing.T) {
		_, err := h.ListDeleteEvents(ctx, &pb.ListDeleteEventsRequest{ReplicationId: "bad-format"})
		require.ErrorIs(t, err, dom.ErrInvalidArg)
	})

	t.Run("nonexistent replication_id returns not found", func(t *testing.T) {
		_, err := h.ListDeleteEvents(ctx, &pb.ListDeleteEventsRequest{ReplicationId: "nonexistent:src:dst"})
		require.ErrorIs(t, err, dom.ErrNotFound)
	})

	t.Run("valid replication_id succeeds", func(t *testing.T) {
		res, err := h.ListDeleteEvents(ctx, &pb.ListDeleteEventsRequest{ReplicationId: "valid:src:dst"})
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}

func TestRetryFailedDeletes_Validation(t *testing.T) {
	ctx := context.Background()
	mockSvc := &mockDeleteEventsPolicyService{}
	h := &policyHandlers{policySvc: mockSvc}

	t.Run("invalid replication_id format fails", func(t *testing.T) {
		_, err := h.RetryFailedDeletes(ctx, &pb.RetryFailedDeletesRequest{ReplicationId: "bad-format"})
		require.ErrorIs(t, err, dom.ErrInvalidArg)
	})

	t.Run("nonexistent replication_id returns not found", func(t *testing.T) {
		_, err := h.RetryFailedDeletes(ctx, &pb.RetryFailedDeletesRequest{ReplicationId: "nonexistent:src:dst"})
		require.ErrorIs(t, err, dom.ErrNotFound)
	})

	t.Run("valid replication_id succeeds", func(t *testing.T) {
		_, err := h.RetryFailedDeletes(ctx, &pb.RetryFailedDeletesRequest{ReplicationId: "valid:src:dst"})
		require.NoError(t, err)
	})
}
