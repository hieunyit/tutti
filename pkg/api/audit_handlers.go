// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

func normalizeAuditFamily(fam string) string {
	switch strings.TrimSpace(fam) {
	case "console_user":
		return "console_users"
	case "storage":
		return "storages"
	case "s3_identity":
		return "s3_identities"
	case "dynamic_credential":
		return "dynamic_credentials"
	case "proxy_account":
		return "proxy_accounts"
	case "access_policy":
		return "access_policies"
	case "route":
		return "routes"
	case "route_block":
		return "route_blocks"
	case "replication_policy":
		return "replication_policies"
	case "replication_generation":
		return "replication_generations"
	case "failover_policy":
		return "failover_policies"
	case "failover_event":
		return "failover_events"
	case "protection_group":
		return "protection_groups"
	case "storage_test":
		return "storage_tests"
	default:
		return strings.TrimSpace(fam)
	}
}

func (h *resilienceHandlers) ListAuditLog(ctx context.Context, req *pb.ListAuditLogRequest) (*pb.AuditLogList, error) {
	backend := strings.TrimSpace(h.cpConfig.Backend)
	if backend == "" {
		if h.cpStore != nil {
			backend = controlplane.BackendPostgres
		} else {
			backend = controlplane.BackendRedis
		}
	}
	if h.cpStore == nil {
		return &pb.AuditLogList{
			Enabled: false,
			Backend: backend,
			Entries: []*pb.AuditLogEntry{},
		}, nil
	}
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	filter := controlplane.AuditFilter{
		ObjectType: normalizeAuditFamily(req.GetFamily()),
		ObjectID:   strings.TrimSpace(req.GetEntityKey()),
		Actor:      strings.TrimSpace(req.GetActor()),
		Action:     strings.TrimSpace(req.GetAction()),
		BeforeID:   req.GetBeforeId(),
		Limit:      limit,
	}
	if req.GetSince() != nil {
		filter.Since = req.GetSince().AsTime()
	}
	if req.GetUntil() != nil {
		filter.Until = req.GetUntil().AsTime()
	}
	rows, err := h.cpStore.Repos().Audit.List(ctx, filter)
	if err != nil {
		// a database outage is 503 (retryable), not an internal error
		return nil, controlplane.WrapWriteError(err)
	}
	out := make([]*pb.AuditLogEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, &pb.AuditLogEntry{
			Id:         r.ID,
			OccurredAt: timestamppb.New(r.At),
			Actor:      r.Actor,
			Action:     r.Action,
			Family:     r.ObjectType,
			EntityKey:  r.ObjectID,
			BeforeJson: string(r.Before),
			AfterJson:  string(r.After),
			RequestId:  r.RequestID,
		})
	}
	return &pb.AuditLogList{
		Enabled: true,
		Backend: backend,
		Entries: out,
	}, nil
}
