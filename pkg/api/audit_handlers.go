// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

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
	actionFilter := strings.TrimSpace(req.GetAction())
	dbLimit := limit
	if actionFilter != "" {
		dbLimit = max(limit*4, 500)
	}
	filter := controlplane.AuditFilter{
		ObjectType: strings.TrimSpace(req.GetFamily()),
		ObjectID:   strings.TrimSpace(req.GetEntityKey()),
		Actor:      strings.TrimSpace(req.GetActor()),
		BeforeID:   req.GetBeforeId(),
		Limit:      dbLimit,
	}
	if req.GetSince() != nil {
		filter.Since = req.GetSince().AsTime()
	}
	if req.GetUntil() != nil {
		filter.Until = req.GetUntil().AsTime()
	}
	rows, err := h.cpStore.Repos().Audit.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.AuditLogEntry, 0, len(rows))
	for _, r := range rows {
		if actionFilter != "" && !strings.EqualFold(r.Action, actionFilter) {
			continue
		}
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
		if len(out) >= limit {
			break
		}
	}
	return &pb.AuditLogList{
		Enabled: true,
		Backend: backend,
		Entries: out,
	}, nil
}
