package api

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/hieunyit/tutti/pkg/trafficstats"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

// ResilienceHandlers serves proxy status and failover policies/events.
// confRedis holds failover data and proxy status documents; known reports whether a storage is configured.
func ResilienceHandlers(confRedis redis.Cmdable, known func(storage string) bool) pb.ResilienceServer {
	return &resilienceHandlers{redis: confRedis, store: failover.NewStore(confRedis, known), policies: accesspolicy.NewStore(confRedis)}
}

var _ pb.ResilienceServer = &resilienceHandlers{}

type resilienceHandlers struct {
	redis    redis.Cmdable
	store    *failover.Store
	policies *accesspolicy.Store
}

func (h *resilienceHandlers) GetProxyStatus(ctx context.Context, _ *emptypb.Empty) (*pb.ProxyStatusList, error) {
	list, err := proxystatus.List(ctx, h.redis)
	if err != nil {
		return nil, err
	}
	res := &pb.ProxyStatusList{Proxies: make([]*pb.ProxyStatus, 0, len(list))}
	for _, st := range list {
		p := &pb.ProxyStatus{
			AppId:     st.AppID,
			Address:   st.Address,
			StartedAt: timestamppb.New(st.StartedAt),
			UpdatedAt: timestamppb.New(st.UpdatedAt),
			Settings: &pb.ProxySettings{
				Replication: &pb.ReplicationSettings{
					OnEventError:     st.Settings.Replication.OnEventError,
					EventRetryBudget: durationpb.New(st.Settings.Replication.EventRetryBudget),
					Sync: &pb.SyncSettings{
						Enabled:         st.Settings.Replication.Sync.Enabled,
						WriteQuorum:     int32(st.Settings.Replication.Sync.WriteQuorum),
						Timeout:         durationpb.New(st.Settings.Replication.Sync.Timeout),
						OnQuorumFailure: st.Settings.Replication.Sync.OnQuorumFailure,
					},
				},
				Health: &pb.HealthSettings{
					FailureThreshold: int32(st.Settings.Health.FailureThreshold),
					OpenDuration:     durationpb.New(st.Settings.Health.OpenDuration),
				},
				Failover: &pb.FailoverSettings{
					Enabled:       st.Settings.Failover.Enabled,
					CheckInterval: durationpb.New(st.Settings.Failover.CheckInterval),
					After:         durationpb.New(st.Settings.Failover.After),
					MaxEventLag:   int32(st.Settings.Failover.MaxEventLag),
				},
				RequireSignedContentType: st.Settings.RequireSignedContentType,
			},
		}
		p.Stats = trafficStatsToPb(st.Stats)
		for _, s := range st.Storages {
			p.Storages = append(p.Storages, &pb.StorageHealth{
				Name:                s.Name,
				Available:           s.Available,
				ConsecutiveFailures: int32(s.ConsecutiveFailures),
				LastError:           s.LastError,
				LastChange:          timestamppb.New(s.LastChange),
			})
		}
		res.Proxies = append(res.Proxies, p)
	}
	return res, nil
}

func (h *resilienceHandlers) ListFailoverPolicies(ctx context.Context, _ *emptypb.Empty) (*pb.FailoverPolicyList, error) {
	policies, err := h.store.ListFailoverPolicies(ctx)
	if err != nil {
		return nil, err
	}
	res := &pb.FailoverPolicyList{Policies: make([]*pb.FailoverPolicy, 0, len(policies))}
	for _, p := range policies {
		fp := &pb.FailoverPolicy{
			Source:  p.Source,
			Enabled: p.Enabled,
			Targets: p.Targets,
			After:   durationpb.New(p.After),
		}
		if p.MaxEventLag != nil {
			v := int32(*p.MaxEventLag)
			fp.MaxEventLag = &v
		}
		res.Policies = append(res.Policies, fp)
	}
	return res, nil
}

func (h *resilienceHandlers) SetFailoverPolicy(ctx context.Context, req *pb.FailoverPolicy) (*emptypb.Empty, error) {
	p := failover.Policy{Source: req.GetSource(), Enabled: req.GetEnabled(), Targets: req.GetTargets()}
	if req.After != nil {
		if err := req.After.CheckValid(); err != nil {
			return nil, fmt.Errorf("%w: failover policy: invalid after: %v", dom.ErrInvalidArg, err)
		}
		p.After = req.After.AsDuration()
	}
	if req.MaxEventLag != nil {
		v := int(*req.MaxEventLag)
		p.MaxEventLag = &v
	}
	if err := h.store.SetFailoverPolicy(ctx, p); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *resilienceHandlers) DeleteFailoverPolicy(ctx context.Context, req *pb.FailoverPolicySource) (*emptypb.Empty, error) {
	if req.GetSource() == "" {
		return nil, fmt.Errorf("%w: failover policy: source is required", dom.ErrInvalidArg)
	}
	if _, err := h.store.GetFailoverPolicy(ctx, req.Source); err != nil {
		return nil, err // dom.ErrNotFound if missing
	}
	if err := h.store.DeleteFailoverPolicy(ctx, req.Source); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *resilienceHandlers) ListFailoverEvents(ctx context.Context, _ *emptypb.Empty) (*pb.FailoverEventList, error) {
	events, err := h.store.ListFailoverEvents(ctx)
	if err != nil {
		return nil, err
	}
	res := &pb.FailoverEventList{Events: make([]*pb.FailoverEvent, 0, len(events))}
	for _, e := range events {
		res.Events = append(res.Events, &pb.FailoverEvent{
			User: e.User, Bucket: e.Bucket, From: e.From, To: e.To,
			Time: timestamppb.New(e.Time), EventLag: int32(e.EventLag),
		})
	}
	return res, nil
}

func (h *resilienceHandlers) DeleteFailoverEvent(ctx context.Context, req *pb.FailoverEventID) (*emptypb.Empty, error) {
	if req.GetUser() == "" {
		return nil, fmt.Errorf("%w: failover event: user is required", dom.ErrInvalidArg)
	}
	events, err := h.store.ListFailoverEvents(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, e := range events {
		if e.User == req.User && e.Bucket == req.Bucket {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: no failover event for user %q bucket %q", dom.ErrNotFound, req.User, req.Bucket)
	}
	if err := h.store.DeleteFailoverEvent(ctx, req.User, req.Bucket); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func trafficStatsToPb(s *trafficstats.Snapshot) *pb.TrafficStats {
	if s == nil {
		return nil
	}
	return &pb.TrafficStats{
		CollectedAt:         timestamppb.New(s.CollectedAt),
		StartedAt:           timestamppb.New(s.StartedAt),
		RequestsTotal:       s.Requests.Total,
		RequestsByClass:     s.Requests.ByClass,
		RequestsByStorage:   s.Requests.ByStorage,
		Responses_2Xx:       s.Responses.Status2xx,
		Responses_3Xx:       s.Responses.Status3xx,
		Responses_4Xx:       s.Responses.Status4xx,
		Responses_5Xx:       s.Responses.Status5xx,
		ResponsesOther:      s.Responses.Other,
		FailFast:            s.FailFast,
		AccessDenied:        s.AccessDenied,
		EventRecordFailed:   s.EventRecordFailed,
		SyncQuorumReached:   s.Sync.QuorumReached,
		SyncBelowQuorum:     s.Sync.BelowQuorum,
		SyncSkipped:         s.Sync.Skipped,
		ChunkedVerifyFailed: s.ChunkedVerifyFailed,
		BytesIn:             s.BytesIn,
		BytesOut:            s.BytesOut,
		Latency: &pb.LatencyHistogram{
			Count:         s.Latency.Count,
			SumSeconds:    s.Latency.SumSeconds,
			BoundsSeconds: s.Latency.BoundsSeconds,
			Buckets:       s.Latency.Buckets,
		},
	}
}
