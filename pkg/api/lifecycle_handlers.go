// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/lifecycle"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

type lifecycleHandlers struct {
	pb.UnimplementedLifecycleServer
	svc lifecycle.Service
}

func LifecycleHandlers(svc lifecycle.Service) pb.LifecycleServer {
	return &lifecycleHandlers{svc: svc}
}

func (h *lifecycleHandlers) CreateRule(ctx context.Context, req *pb.CreateLifecycleRuleRequest) (*pb.LifecycleRule, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is required", dom.ErrInvalidArg)
	}
	if strings.TrimSpace(req.GetName()) == "" || len(req.GetName()) > 64 {
		return nil, fmt.Errorf("%w: name is required (max 64 characters)", dom.ErrInvalidArg)
	}
	if strings.TrimSpace(req.GetStorageName()) == "" {
		return nil, fmt.Errorf("%w: storage_name is required", dom.ErrInvalidArg)
	}
	if strings.TrimSpace(req.GetBucket()) == "" {
		return nil, fmt.Errorf("%w: bucket is required", dom.ErrInvalidArg)
	}
	if req.GetExpirationDays() < 1 || req.GetExpirationDays() > 36500 {
		return nil, fmt.Errorf("%w: expiration_days must be between 1 and 36500", dom.ErrInvalidArg)
	}
	rule := controlplane.LifecycleRule{
		Name:           req.GetName(),
		StorageName:    req.GetStorageName(),
		Bucket:         req.GetBucket(),
		Prefix:         req.GetPrefix(),
		ExpirationDays: req.GetExpirationDays(),
		Schedule:       req.GetSchedule(),
		Enabled:        req.GetEnabled(),
	}
	created, err := h.svc.CreateRule(ctx, rule)
	if err != nil {
		return nil, err
	}
	return mapLifecycleRuleToPb(created), nil
}

func (h *lifecycleHandlers) ListRules(ctx context.Context, req *pb.ListLifecycleRulesRequest) (*pb.ListLifecycleRulesResponse, error) {
	rules, err := h.svc.ListRules(ctx, req.GetStorageName(), req.GetBucket())
	if err != nil {
		return nil, err
	}
	pbRules := make([]*pb.LifecycleRule, 0, len(rules))
	for _, r := range rules {
		pbRules = append(pbRules, mapLifecycleRuleToPb(r))
	}
	return &pb.ListLifecycleRulesResponse{Rules: pbRules}, nil
}

func (h *lifecycleHandlers) GetRule(ctx context.Context, req *pb.GetLifecycleRuleRequest) (*pb.LifecycleRule, error) {
	rule, err := h.svc.GetRule(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return mapLifecycleRuleToPb(rule), nil
}

func (h *lifecycleHandlers) UpdateRule(ctx context.Context, req *pb.UpdateLifecycleRuleRequest) (*pb.LifecycleRule, error) {
	if req.GetExpirationDays() < 1 || req.GetExpirationDays() > 36500 {
		return nil, fmt.Errorf("%w: expiration_days must be between 1 and 36500", dom.ErrInvalidArg)
	}
	rule := controlplane.LifecycleRule{
		ID:             req.GetId(),
		Name:           req.GetName(),
		Prefix:         req.GetPrefix(),
		ExpirationDays: req.GetExpirationDays(),
		Schedule:       req.GetSchedule(),
		Enabled:        req.GetEnabled(),
	}
	updated, err := h.svc.UpdateRule(ctx, rule)
	if err != nil {
		return nil, err
	}
	return mapLifecycleRuleToPb(updated), nil
}

func (h *lifecycleHandlers) DeleteRule(ctx context.Context, req *pb.DeleteLifecycleRuleRequest) (*emptypb.Empty, error) {
	err := h.svc.DeleteRule(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *lifecycleHandlers) EnableRule(ctx context.Context, req *pb.EnableLifecycleRuleRequest) (*emptypb.Empty, error) {
	err := h.svc.SetRuleEnabled(ctx, req.GetId(), true)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *lifecycleHandlers) DisableRule(ctx context.Context, req *pb.DisableLifecycleRuleRequest) (*emptypb.Empty, error) {
	err := h.svc.SetRuleEnabled(ctx, req.GetId(), false)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *lifecycleHandlers) DryRun(ctx context.Context, req *pb.LifecycleDryRunRequest) (*pb.LifecycleDryRunResponse, error) {
	res, err := h.svc.DryRun(ctx, lifecycle.DryRunRequest{
		StorageName:    req.GetStorageName(),
		Bucket:         req.GetBucket(),
		Prefix:         req.GetPrefix(),
		ExpirationDays: req.GetExpirationDays(),
		SampleSize:     req.GetSampleSize(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.LifecycleDryRunResponse{
		EstimatedDeletedCount: res.EstimatedDeletedCount,
		EstimatedBytesFreed:   res.EstimatedBytesFreed,
		SampleObjects:         res.SampleObjects,
	}, nil
}

func (h *lifecycleHandlers) ListAuditLogs(ctx context.Context, req *pb.ListLifecycleAuditLogsRequest) (*pb.ListLifecycleAuditLogsResponse, error) {
	if req.GetRuleId() <= 0 {
		return nil, fmt.Errorf("%w: rule_id must be a positive integer", dom.ErrInvalidArg)
	}
	limit := int64(req.GetLimit())
	offset := req.GetOffset()
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset cannot be negative", dom.ErrInvalidArg)
	}
	if limit <= 0 {
		limit = 50
	} else if limit > 500 {
		limit = 500
	}
	entries, total, err := h.svc.ListAuditLogs(ctx, req.GetRuleId(), limit, offset)
	if err != nil {
		return nil, err
	}
	pbEntries := make([]*pb.LifecycleAuditLogEntry, 0, len(entries))
	for _, e := range entries {
		pbEntries = append(pbEntries, mapLifecycleAuditToPb(e))
	}
	return &pb.ListLifecycleAuditLogsResponse{
		Entries: pbEntries,
		Total:   total,
	}, nil
}

func mapLifecycleRuleToPb(r controlplane.LifecycleRule) *pb.LifecycleRule {
	res := &pb.LifecycleRule{
		Id:             r.ID,
		Name:           r.Name,
		StorageName:    r.StorageName,
		Bucket:         r.Bucket,
		Prefix:         r.Prefix,
		ExpirationDays: r.ExpirationDays,
		Schedule:       r.Schedule,
		Enabled:        r.Enabled,
		CreatedAt:      timestamppb.New(r.CreatedAt),
		UpdatedAt:      timestamppb.New(r.UpdatedAt),
	}
	if r.NextRunAt != nil {
		res.NextRunAt = timestamppb.New(*r.NextRunAt)
	}
	if r.LastRunAt != nil {
		res.LastRunAt = timestamppb.New(*r.LastRunAt)
	}
	if r.LastRunStats != nil {
		res.LastRunStats = &pb.LifecycleRunStats{
			DeletedCount: r.LastRunStats.DeletedCount,
			BytesFreed:   r.LastRunStats.BytesFreed,
			DurationMs:   r.LastRunStats.DurationMs,
		}
	}
	return res
}

func mapLifecycleAuditToPb(a controlplane.LifecycleAuditEntry) *pb.LifecycleAuditLogEntry {
	return &pb.LifecycleAuditLogEntry{
		Id:           a.ID,
		RuleId:       a.RuleID,
		Bucket:       a.Bucket,
		ObjectKey:    a.ObjectKey,
		SizeBytes:    a.SizeBytes,
		LastModified: timestamppb.New(a.LastModified),
		DeletedAt:    timestamppb.New(a.DeletedAt),
	}
}
