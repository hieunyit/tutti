// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"fmt"
	"github.com/hieunyit/tutti/pkg/dom"
	pg "github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/store"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type protectionHandlers struct {
	service      *pg.Service
	storageLocks *StorageLocker
	userLocks    *store.UserLocker
}

func ProtectionGroupHandlers(s *pg.Service, storageLocks *StorageLocker, userLocks *store.UserLocker) pb.ProtectionGroupsServer {
	return &protectionHandlers{s, storageLocks, userLocks}
}
func groupToPB(g pg.Group) *pb.ProtectionGroup {
	p := &pb.ProtectionGroup{Id: g.ID, User: g.User, Buckets: g.Buckets, Members: g.Members, Copies: int32(g.Copies), MaxEventLag: int32(g.MaxEventLag), Mode: g.Mode, After: durationpb.New(g.After), Electorate: g.Electorate, MaxSwitchesPerHour: int32(g.MaxSwitchesPerHour), Primary: g.Primary, Generation: g.Generation, Revision: g.Revision, State: g.State, Target: g.Target, Frozen: g.Frozen, LastError: g.LastError, Quarantined: g.Quarantined}
	for _, e := range g.History {
		p.History = append(p.History, &pb.ProtectionGroupEvent{From: e.From, To: e.To, At: timestamppb.New(e.At), Generation: e.Generation, Automatic: e.Automatic, EventLag: int32(e.EventLag)})
	}
	return p
}
func groupFromPB(p *pb.ProtectionGroup) (pg.Group, error) {
	if p == nil {
		return pg.Group{}, fmt.Errorf("%w: group is required", dom.ErrInvalidArg)
	}
	if p.After != nil {
		if err := p.After.CheckValid(); err != nil {
			return pg.Group{}, fmt.Errorf("%w: invalid after", dom.ErrInvalidArg)
		}
	}
	return pg.Group{ID: p.Id, User: p.User, Buckets: p.Buckets, Members: p.Members, Copies: int(p.Copies), MaxEventLag: int(p.MaxEventLag), Mode: p.Mode, After: p.After.AsDuration(), Electorate: p.Electorate, MaxSwitchesPerHour: int(p.MaxSwitchesPerHour), Primary: p.Primary, Revision: p.Revision}, nil
}
func (h *protectionHandlers) List(ctx context.Context, _ *emptypb.Empty) (*pb.ProtectionGroupList, error) {
	groups, err := h.service.Store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := &pb.ProtectionGroupList{}
	for _, g := range groups {
		out.Groups = append(out.Groups, groupToPB(g))
	}
	return out, nil
}
func (h *protectionHandlers) Get(ctx context.Context, r *pb.ProtectionGroupID) (*pb.ProtectionGroupStatus, error) {
	st, err := h.service.Status(ctx, r.GetId())
	if err != nil {
		return nil, err
	}
	out := &pb.ProtectionGroupStatus{Group: groupToPB(st.Group), HealthyCopies: int32(st.HealthyCopies), ActiveWriters: st.ActiveWriters}
	for _, m := range st.Members {
		out.Members = append(out.Members, &pb.ProtectionGroupMemberStatus{Storage: m.Storage, Available: m.Available, Initialized: m.Initialized, EventLag: int32(m.EventLag), Current: m.Current})
	}
	return out, nil
}
func (h *protectionHandlers) Create(ctx context.Context, r *pb.CreateProtectionGroupRequest) (*pb.ProtectionGroup, error) {
	g, err := groupFromPB(r.GetGroup())
	if err != nil {
		return nil, err
	}
	// No silent default: the group manages the routing and replications of exactly this (existing) user.
	if g.User == "" {
		return nil, fmt.Errorf("%w: protection group: user is required", dom.ErrInvalidArg)
	}
	var result pg.Group
	work := func(ctx context.Context) error {
		return h.storageLocks.With(ctx, g.Members, func(ctx context.Context) error {
			var e error
			result, e = h.service.Create(ctx, g, r.AdoptExisting)
			return e
		})
	}
	if h.userLocks != nil {
		lock, e := h.userLocks.Lock(ctx, g.User, store.WithDuration(30*time.Second))
		if e != nil {
			return nil, e
		}
		defer lock.Release(ctx)
		err = lock.DoCtx(ctx, 20*time.Second, work)
	} else {
		err = work(ctx)
	}
	if err != nil {
		return nil, err
	}
	return groupToPB(result), nil
}
func (h *protectionHandlers) Update(ctx context.Context, r *pb.ProtectionGroup) (*pb.ProtectionGroup, error) {
	g, err := groupFromPB(r)
	if err != nil {
		return nil, err
	}
	g, err = h.service.Update(ctx, g)
	if err != nil {
		return nil, err
	}
	return groupToPB(g), nil
}
func (h *protectionHandlers) Delete(ctx context.Context, r *pb.ProtectionGroupID) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, h.service.Delete(ctx, r.GetId())
}
func (h *protectionHandlers) Switchover(ctx context.Context, r *pb.ProtectionGroupSwitchRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, h.service.RequestSwitch(ctx, r.GetId(), r.GetTarget())
}
func (h *protectionHandlers) AbortSwitch(ctx context.Context, r *pb.ProtectionGroupID) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, h.service.AbortSwitch(ctx, r.GetId())
}
func (h *protectionHandlers) SetAutomaticFreeze(ctx context.Context, r *pb.ProtectionAutomaticFreeze) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, h.service.Store.SetAutomaticFrozen(ctx, r.GetFrozen())
}
func (h *protectionHandlers) GetAutomaticFreeze(ctx context.Context, _ *emptypb.Empty) (*pb.ProtectionAutomaticFreeze, error) {
	f, err := h.service.Store.AutomaticFrozen(ctx)
	return &pb.ProtectionAutomaticFreeze{Frozen: f}, err
}
func (h *protectionHandlers) ListWriters(ctx context.Context, r *pb.ProtectionGroupID) (*pb.ProtectionWriters, error) {
	writers, err := h.service.Store.Writers(ctx, r.GetId())
	if err != nil {
		return nil, err
	}
	workers, err := h.service.Store.Redis.HGetAll(ctx, "tutti:protection:writers:"+r.GetId()+":workers").Result()
	return &pb.ProtectionWriters{Writers: writers, Workers: workers}, err
}
func (h *protectionHandlers) ClearWriter(ctx context.Context, r *pb.ProtectionWriterClearRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, h.service.Store.ClearWriterKind(ctx, r.GetId(), r.GetToken(), r.GetExternallyFenced(), r.GetWorker())
}
