package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/hieunyit/tutti/pkg/s3users"
	"github.com/hieunyit/tutti/pkg/trafficstats"
	"github.com/hieunyit/tutti/pkg/workerstats"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

// ResilienceHandlers serves proxy status and failover policies/events.
// confRedis holds failover data and proxy status documents; known reports whether a storage is configured.
func ResilienceHandlers(confRedis redis.Cmdable, known func(storage string) bool, opts ...ResilienceOption) pb.ResilienceServer {
	h := &resilienceHandlers{redis: confRedis, store: failover.NewStore(confRedis, known), policies: accesspolicy.NewStore(confRedis)}
	h.accounts = proxyaccount.NewStore(confRedis, nil)
	for _, o := range opts {
		o(h)
	}
	if h.cpStore != nil {
		h.store.WithControlPlane(h.cpStore)
		h.policies.WithControlPlane(h.cpStore)
		h.accounts.WithControlPlane(h.cpStore)
	}
	if h.s3users != nil {
		h.s3users.Accounts, h.s3users.Policies, h.s3users.Redis = h.accounts, h.policies, h.redis
		h.s3users.CP = h.cpStore
		if h.s3usersRefs != nil {
			h.s3users.Refs = h.s3usersRefs(h.store)
		}
	}
	return h
}

// WithControlPlane attaches a PostgreSQL control-plane store and config to the resilience handlers, enabling the
// audit log RPC and switching failover policies/events, access policies, proxy accounts and S3 users to PostgreSQL.
func WithControlPlane(cp controlplane.Store, cfg controlplane.Config) ResilienceOption {
	return func(h *resilienceHandlers) {
		h.cpStore = cp
		h.cpConfig = cfg
	}
}

// WithS3Users enables the S3 user RPCs. creds are the storage credentials, verify checks credentials against a
// storage and refs builds the lookup of replications, routings and failover events that reference a user.
func WithS3Users(creds s3users.Credentials, verify s3users.Verifier, refs func(events *failover.Store) s3users.References) ResilienceOption {
	return func(h *resilienceHandlers) {
		h.s3users = &s3users.Manager{Creds: creds, Verify: verify}
		h.s3usersRefs = refs
	}
}

// WithConsoleUsers enables the console user RPCs, backed by the auth service.
func WithConsoleUsers(svc *auth.Service) ResilienceOption {
	return func(h *resilienceHandlers) { h.auth = svc }
}

// ResilienceOption customizes ResilienceHandlers.
type ResilienceOption func(*resilienceHandlers)

// WithProxyAccounts configures runtime-managed proxy accounts: sealer provides the secret sealing (it must fail with
// dom.ErrFailedPrecondition if no master password is configured) and userExists reports whether a storage user has
// credentials. Without this option, accounts cannot be created.
func WithProxyAccounts(sealer func() (proxyaccount.Sealer, error), userExists func(user string) bool) ResilienceOption {
	return func(h *resilienceHandlers) {
		h.accounts = proxyaccount.NewStore(h.redis, sealer)
		h.sealer = sealer
		h.userExists = userExists
	}
}

var _ pb.ResilienceServer = &resilienceHandlers{}

type resilienceHandlers struct {
	runtimeStorages RuntimeStorages
	runtimeRefs     StorageReferences
	storageLocker   *StorageLocker
	redis           redis.Cmdable
	store           *failover.Store
	policies        *accesspolicy.Store
	accounts        *proxyaccount.Store
	s3users         *s3users.Manager
	s3usersRefs     func(events *failover.Store) s3users.References
	auth            *auth.Service
	cpStore         controlplane.Store
	cpConfig        controlplane.Config
	// sealer is the one given to WithProxyAccounts (nil if not configured).
	sealer func() (proxyaccount.Sealer, error)
	// userExists reports whether the storage user has credentials. Nil disables the check.
	userExists func(user string) bool
}

func (h *resilienceHandlers) GetProxyStatus(ctx context.Context, _ *emptypb.Empty) (*pb.ProxyStatusList, error) {
	list, err := proxystatus.List(ctx, h.redis)
	if err != nil {
		return nil, err
	}
	res := &pb.ProxyStatusList{Proxies: make([]*pb.ProxyStatus, 0, len(list))}
	if fc, err := proxystatus.NewFleetConfigStore(h.redis).Get(ctx); err == nil && fc != nil {
		res.ConfigVersion = fc.Version
	}
	var all []trafficstats.Rates
	for _, st := range list {
		p := &pb.ProxyStatus{
			AppId:                 st.AppID,
			Address:               st.Address,
			Group:                 st.Group,
			StartedAt:             timestamppb.New(st.StartedAt),
			UpdatedAt:             timestamppb.New(st.UpdatedAt),
			AppliedConfigVersion:  st.AppliedConfigVersion,
			PendingRestart:        st.PendingRestart,
			PendingRestartReasons: st.PendingRestartReasons,
			ConfigError:           st.ConfigError,
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
				ReadHeaderTimeout:        durationpb.New(st.Settings.ReadHeaderTimeout),
				WriterLease:              durationpb.New(st.Settings.WriterLease),
				DrainDelay:               durationpb.New(st.Settings.DrainDelay),
				ReadyMaxStaleness:        durationpb.New(st.Settings.ReadyMaxStaleness),
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
		if st.Rates != nil {
			p.Rates = ratesToPb(*st.Rates)
			all = append(all, *st.Rates)
		}
		res.Proxies = append(res.Proxies, p)
	}
	if len(all) > 0 {
		res.TotalRates = ratesToPb(trafficstats.SumRates(all))
	}
	return res, nil
}

// statsRanges are the accepted GetStatsHistory ranges.
var statsRanges = map[string]time.Duration{"5m": 5 * time.Minute, "15m": 15 * time.Minute, "1h": time.Hour, "6h": 6 * time.Hour}

func (h *resilienceHandlers) GetStatsHistory(ctx context.Context, req *pb.StatsHistoryRequest) (*pb.StatsHistory, error) {
	rng := req.GetRange()
	if rng == "" {
		rng = "15m"
	}
	d, ok := statsRanges[rng]
	if !ok {
		return nil, fmt.Errorf("%w: stats history: invalid range %q (use 5m, 15m, 1h or 6h)", dom.ErrInvalidArg, rng)
	}
	now := time.Now()
	per, err := proxystatus.ReadHistory(ctx, h.redis, now.Add(-d))
	if err != nil {
		return nil, err
	}
	bucket := proxystatus.BucketForRange(d)
	res := &pb.StatsHistory{Range: rng, Bucket: durationpb.New(bucket)}
	for _, pt := range proxystatus.Aggregate(per, bucket) {
		res.Points = append(res.Points, &pb.StatsHistoryPoint{Time: timestamppb.New(pt.Time), Rates: ratesToPb(pt.Rates)})
	}
	apps := make([]string, 0, len(per))
	for a := range per {
		apps = append(apps, a)
	}
	sort.Strings(apps)
	for _, a := range apps {
		last := per[a][len(per[a])-1]
		res.Proxies = append(res.Proxies, &pb.ProxyLatestRates{AppId: a, Time: timestamppb.New(time.Unix(last.Time, 0)), Rates: ratesToPb(last.Rates)})
	}
	return res, nil
}

func ratesToPb(r trafficstats.Rates) *pb.TrafficRates {
	return &pb.TrafficRates{
		WindowSeconds:      r.WindowSeconds,
		RequestsPerSec:     r.RequestsPerSec,
		RequestsByClass:    r.ByClass,
		RequestsByStorage:  r.ByStorage,
		BytesInPerSec:      r.BytesInPerSec,
		BytesOutPerSec:     r.BytesOutPerSec,
		Status_4XxPerSec:   r.Status4xxPerSec,
		Status_5XxPerSec:   r.Status5xxPerSec,
		FailFastPerSec:     r.FailFastPerSec,
		AccessDeniedPerSec: r.AccessDeniedPerSec,
		LatencyAvgMs:       r.LatencyAvgMs,
		LatencyP95Ms:       r.LatencyP95Ms,
	}
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
	// the policy references its source and targets: serialize with runtime storage update/delete
	if err := h.storageLocker.With(ctx, append([]string{p.Source}, p.Targets...), func(ctx context.Context) error {
		// protection groups own the failover of their members: never let both act on the same storages
		if err := protectiongroups.CheckLegacyFailoverPolicy(ctx, h.redis, p); err != nil {
			return err
		}
		return h.store.SetFailoverPolicy(ctx, p)
	}); err != nil {
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
		NoCredentials:       s.NoCredentials,
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

// GetWorkerStats aggregates the snapshots published by the live worker processes.
func (h *resilienceHandlers) GetWorkerStats(ctx context.Context, _ *emptypb.Empty) (*pb.WorkerStatsResponse, error) {
	snaps, err := workerstats.Read(ctx, h.redis, time.Now())
	if err != nil {
		return nil, err
	}
	return workerStatsToPb(snaps), nil
}

func workerStatsToPb(snaps []workerstats.Snapshot) *pb.WorkerStatsResponse {
	res := &pb.WorkerStatsResponse{Workers: make([]*pb.WorkerStatsSnapshot, 0, len(snaps))}
	for _, s := range snaps {
		res.Workers = append(res.Workers, &pb.WorkerStatsSnapshot{
			WorkerId: s.WorkerID, StartedAt: s.StartedAt, UpdatedAt: s.UpdatedAt,
			Concurrency: int32(s.Concurrency), BulkConcurrency: int32(s.BulkConcurrency), LiveConcurrency: int32(s.LiveConcurrency),
			InFlight: s.InFlight, CopiedObjectsTotal: s.CopiedObjectsTotal, CopiedBytesTotal: s.CopiedBytesTotal, FailedTotal: s.FailedTotal,
			ObjectsPerSec_1M: s.ObjectsPerSec1m, BytesPerSec_1M: s.BytesPerSec1m,
			ObjectsPerSec_5M: s.ObjectsPerSec5m, BytesPerSec_5M: s.BytesPerSec5m,
			AvgCopySeconds_1M: s.AvgCopySeconds1m,
			BufferTotalBytes:  s.BufferTotalBytes, BufferUsedBytes: s.BufferUsedBytes, BufferWaiters: s.BufferWaiters,
			BufferWaitSecondsTotal: s.BufferWaitSecondsTotal,
		})
	}
	t := workerstats.Aggregate(snaps)
	res.Totals = &pb.WorkerStatsTotals{
		InFlight: t.InFlight, Concurrency: t.Concurrency,
		ObjectsPerSec_1M: t.ObjectsPerSec1m, BytesPerSec_1M: t.BytesPerSec1m,
		ObjectsPerSec_5M: t.ObjectsPerSec5m, BytesPerSec_5M: t.BytesPerSec5m,
		BufferTotalBytes: t.BufferTotalBytes, BufferUsedBytes: t.BufferUsedBytes, BufferWaiters: t.BufferWaiters,
		CopiedObjectsTotal: t.CopiedObjectsTotal, CopiedBytesTotal: t.CopiedBytesTotal, FailedTotal: t.FailedTotal,
	}
	return res
}
