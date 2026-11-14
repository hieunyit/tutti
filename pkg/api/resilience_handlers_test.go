package api

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/hieunyit/tutti/pkg/trafficstats"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

// resilienceClient starts the handlers behind a real gRPC server with the production error interceptor.
func resilienceClient(t *testing.T) (pb.ResilienceClient, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	known := func(s string) bool { return s == "main" || s == "b" || s == "c" }
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrorInterceptor()))
	pb.RegisterResilienceServer(srv, ResilienceHandlers(rc, known))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewResilienceClient(conn), rc, mr
}

func TestResilience_PolicyCRUD(t *testing.T) {
	ctx := context.Background()
	c, _, _ := resilienceClient(t)

	l, err := c.ListFailoverPolicies(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Empty(t, l.Policies)

	lag := int32(5)
	_, err = c.SetFailoverPolicy(ctx, &pb.FailoverPolicy{Source: "main", Enabled: true, Targets: []string{"b", "c"}, MaxEventLag: &lag,
		After: durationpbSeconds(90)})
	require.NoError(t, err)
	_, err = c.SetFailoverPolicy(ctx, &pb.FailoverPolicy{Source: "b", Targets: []string{"main"}})
	require.NoError(t, err)

	l, err = c.ListFailoverPolicies(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, l.Policies, 2)
	require.Equal(t, "b", l.Policies[0].Source)
	require.Nil(t, l.Policies[0].MaxEventLag)
	p := l.Policies[1]
	require.Equal(t, "main", p.Source)
	require.True(t, p.Enabled)
	require.Equal(t, []string{"b", "c"}, p.Targets)
	require.Equal(t, int32(5), p.GetMaxEventLag())
	require.Equal(t, 90*time.Second, p.After.AsDuration())

	// validation errors
	for name, pol := range map[string]*pb.FailoverPolicy{
		"unknown source": {Source: "nope", Targets: []string{"b"}},
		"unknown target": {Source: "main", Targets: []string{"nope"}},
		"self target":    {Source: "main", Targets: []string{"main"}},
		"enabled empty":  {Source: "main", Enabled: true},
		"no source":      {Targets: []string{"b"}},
		"negative after": {Source: "main", After: durationpbSeconds(-1)},
	} {
		_, err = c.SetFailoverPolicy(ctx, pol)
		require.Equal(t, codes.InvalidArgument, status.Code(err), name)
	}

	_, err = c.DeleteFailoverPolicy(ctx, &pb.FailoverPolicySource{Source: "main"})
	require.NoError(t, err)
	_, err = c.DeleteFailoverPolicy(ctx, &pb.FailoverPolicySource{Source: "main"})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.DeleteFailoverPolicy(ctx, &pb.FailoverPolicySource{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestResilience_Events(t *testing.T) {
	ctx := context.Background()
	c, rc, _ := resilienceClient(t)
	store := failover.NewStore(rc, nil)
	now := time.Now().UTC()
	require.NoError(t, store.SaveFailoverEvent(ctx, failover.Event{User: "u", From: "main", To: "b", Time: now, EventLag: 2}))
	require.NoError(t, store.SaveFailoverEvent(ctx, failover.Event{User: "u", Bucket: "bk", From: "main", To: "c", Time: now}))

	l, err := c.ListFailoverEvents(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, l.Events, 2)

	_, err = c.DeleteFailoverEvent(ctx, &pb.FailoverEventID{User: "u", Bucket: "bk"})
	require.NoError(t, err)
	_, err = c.DeleteFailoverEvent(ctx, &pb.FailoverEventID{User: "u", Bucket: "bk"})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.DeleteFailoverEvent(ctx, &pb.FailoverEventID{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	l, err = c.ListFailoverEvents(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, l.Events, 1)
	require.Equal(t, "b", l.Events[0].To)
	require.Equal(t, int32(2), l.Events[0].EventLag)
	require.Equal(t, "", l.Events[0].Bucket)
}

func TestResilience_ProxyStatusAggregation(t *testing.T) {
	ctx := context.Background()
	c, rc, mr := resilienceClient(t)
	mk := func(id string, interval time.Duration, avail bool) {
		p := proxystatus.NewPublisher(rc, id, interval, func() proxystatus.Status {
			return proxystatus.Status{
				Address: "http://" + id, StartedAt: time.Now(),
				Storages: []proxystatus.StorageHealth{{Name: "main", Available: avail, LastChange: time.Now()}},
				Settings: proxystatus.Settings{RequireSignedContentType: true,
					Replication: proxystatus.ReplicationSettings{Sync: proxystatus.SyncSettings{Enabled: true, WriteQuorum: 2, Timeout: time.Second}},
					Health:      proxystatus.HealthSettings{FailureThreshold: 3, OpenDuration: 30 * time.Second}},
			}
		}, zerolog.Nop())
		require.NoError(t, p.PublishOnce(ctx))
	}
	mk("p1", 10*time.Second, true)
	mk("p2", time.Second, false) // ttl 3s, will expire
	l, err := c.GetProxyStatus(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, l.Proxies, 2)

	mr.FastForward(5 * time.Second)
	l, err = c.GetProxyStatus(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, l.Proxies, 1)
	px := l.Proxies[0]
	require.Equal(t, "p1", px.AppId)
	require.Equal(t, "http://p1", px.Address)
	require.True(t, px.Storages[0].Available)
	require.True(t, px.Settings.RequireSignedContentType)
	require.Equal(t, int32(2), px.Settings.Replication.Sync.WriteQuorum)
	require.Equal(t, 30*time.Second, px.Settings.Health.OpenDuration.AsDuration())
}

func durationpbSeconds(s int64) *durationpb.Duration { return &durationpb.Duration{Seconds: s} }

func TestResilience_AccessPolicies(t *testing.T) {
	ctx := context.Background()
	c, rc, _ := resilienceClient(t)

	_, err := c.SetAccessPolicy(ctx, &pb.AccessPolicy{User: "ro", Permission: "read-only", Buckets: []string{"photos-*"}, Description: "d"})
	require.NoError(t, err)
	_, err = c.SetAccessPolicy(ctx, &pb.AccessPolicy{User: "all", Permission: "write-only"})
	require.NoError(t, err)
	for name, p := range map[string]*pb.AccessPolicy{
		"no user":  {Permission: "read-only"},
		"bad perm": {User: "u", Permission: "admin"},
		"bad glob": {User: "u", Permission: "read-only", Buckets: []string{"[x"}},
	} {
		_, err = c.SetAccessPolicy(ctx, p)
		require.Equal(t, codes.InvalidArgument, status.Code(err), name)
	}
	l, err := c.ListAccessPolicies(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, l.Policies, 2)
	require.Equal(t, "all", l.Policies[0].User)
	require.Equal(t, []string{"*"}, l.Policies[0].Buckets)
	require.Equal(t, "ro", l.Policies[1].User)

	// proxy accounts: listed by id, never by secret
	pub := proxystatus.NewPublisher(rc, "app1", time.Second, func() proxystatus.Status {
		return proxystatus.Status{Address: "x", Accounts: []proxystatus.Account{
			{User: "ro", AccessKeyID: "AKRO", Source: "custom"},
			{User: "free", AccessKeyID: "AKFREE", Source: "storage:main"},
		}}
	}, zerolog.Nop())
	require.NoError(t, pub.PublishOnce(ctx))
	acc, err := c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, acc.Accounts, 3) // all (policy only), free, ro
	by := map[string]*pb.ProxyAccount{}
	for _, a := range acc.Accounts {
		by[a.User] = a
	}
	require.True(t, by["ro"].HasPolicy)
	require.Equal(t, []string{"AKRO"}, by["ro"].AccessKeyIds)
	require.Equal(t, "read-only", by["ro"].Policy.Permission)
	require.False(t, by["free"].HasPolicy)
	require.Equal(t, "read-write", by["free"].Policy.Permission)
	require.Equal(t, []string{"storage:main"}, by["free"].Sources)
	raw, err := protojsonMarshal(acc)
	require.NoError(t, err)
	require.NotContains(t, raw, "ecret")

	_, err = c.DeleteAccessPolicy(ctx, &pb.AccessPolicyUser{User: "ro"})
	require.NoError(t, err)
	_, err = c.DeleteAccessPolicy(ctx, &pb.AccessPolicyUser{User: "ro"})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.DeleteAccessPolicy(ctx, &pb.AccessPolicyUser{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func protojsonMarshal(m proto.Message) (string, error) {
	b, err := protojson.Marshal(m)
	return string(b), err
}

func TestResilience_ProxyStatusTrafficStats(t *testing.T) {
	ctx := context.Background()
	c, rc, _ := resilienceClient(t)
	col := trafficstats.New()
	col.Request(trafficstats.ClassWrite, "main")
	col.Response(503)
	col.FailFast()
	col.SyncOutcome(trafficstats.SyncBelowQuorum)
	col.BytesIn(10)
	col.Latency(300 * time.Millisecond)
	snap := col.Snapshot()
	withStats := proxystatus.NewPublisher(rc, "p1", 10*time.Second, func() proxystatus.Status {
		return proxystatus.Status{StartedAt: time.Now(), Stats: &snap}
	}, zerolog.Nop())
	require.NoError(t, withStats.PublishOnce(ctx))
	noStats := proxystatus.NewPublisher(rc, "p2", 10*time.Second, func() proxystatus.Status {
		return proxystatus.Status{StartedAt: time.Now()}
	}, zerolog.Nop())
	require.NoError(t, noStats.PublishOnce(ctx))

	l, err := c.GetProxyStatus(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, l.Proxies, 2)
	require.Nil(t, l.Proxies[1].Stats)
	s := l.Proxies[0].Stats
	require.NotNil(t, s)
	require.True(t, s.CollectedAt.AsTime().Equal(snap.CollectedAt))
	require.True(t, s.StartedAt.AsTime().Equal(snap.StartedAt))
	require.EqualValues(t, 1, s.RequestsTotal)
	require.EqualValues(t, 1, s.RequestsByClass["write"])
	require.EqualValues(t, 1, s.RequestsByStorage["main"])
	require.EqualValues(t, 1, s.Responses_5Xx)
	require.EqualValues(t, 1, s.FailFast)
	require.EqualValues(t, 1, s.SyncBelowQuorum)
	require.EqualValues(t, 10, s.BytesIn)
	require.EqualValues(t, 1, s.Latency.Count)
	require.Len(t, s.Latency.Buckets, len(s.Latency.BoundsSeconds)+1)
}

func TestGetStatsHistory(t *testing.T) {
	c, rc, _ := resilienceClient(t)
	ctx := t.Context()
	now := time.Now().Unix()
	for app, rps := range map[string]float64{"p1": 10, "p2": 5} {
		require.NoError(t, proxystatus.AppendSample(ctx, rc, app, proxystatus.Sample{Time: now - 20, Rates: trafficstats.Rates{RequestsPerSec: rps, LatencyP95Ms: rps}}))
		require.NoError(t, proxystatus.AppendSample(ctx, rc, app, proxystatus.Sample{Time: now - 3600, Rates: trafficstats.Rates{RequestsPerSec: 100}}))
	}
	h, err := c.GetStatsHistory(ctx, &pb.StatsHistoryRequest{Range: "5m"})
	require.NoError(t, err)
	require.Equal(t, "5m", h.Range)
	require.Equal(t, 10*time.Second, h.Bucket.AsDuration())
	require.Len(t, h.Points, 1, "hour-old samples are outside 5m")
	require.InDelta(t, 15, h.Points[0].Rates.RequestsPerSec, 1e-9)
	require.InDelta(t, 10, h.Points[0].Rates.LatencyP95Ms, 1e-9)
	require.Len(t, h.Proxies, 2)
	require.Equal(t, "p1", h.Proxies[0].AppId)

	h, err = c.GetStatsHistory(ctx, &pb.StatsHistoryRequest{Range: "6h"})
	require.NoError(t, err)
	require.Equal(t, 2*time.Minute, h.Bucket.AsDuration())
	require.Len(t, h.Points, 2)

	h, err = c.GetStatsHistory(ctx, &pb.StatsHistoryRequest{})
	require.NoError(t, err)
	require.Equal(t, "15m", h.Range)

	_, err = c.GetStatsHistory(ctx, &pb.StatsHistoryRequest{Range: "2d"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestResilience_ProxyFleetConfig(t *testing.T) {
	ctx := context.Background()
	c, rc, _ := resilienceClient(t)

	// Initial fleet config is version 0.
	fc0, err := c.GetProxyFleetConfig(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 0, fc0.Version)
	require.Empty(t, fc0.Groups)

	// Set fleet config with defaults and a group override.
	enabled := true
	quorum := int32(2)
	thresh := int32(5)
	reqSigned := true
	fc1, err := c.SetProxyFleetConfig(ctx, &pb.SetProxyFleetConfigRequest{
		Defaults: &pb.ProxySettingsOverride{
			Replication: &pb.ProxyReplicationSettingsOverride{
				Sync: &pb.ProxySyncSettingsOverride{
					Enabled:     &enabled,
					WriteQuorum: &quorum,
					Timeout:     durationpb.New(10 * time.Second),
				},
			},
			RequireSignedContentType: &reqSigned,
		},
		Groups: map[string]*pb.ProxySettingsOverride{
			"canary": {
				Health: &pb.ProxyHealthSettingsOverride{
					FailureThreshold: &thresh,
				},
				ReadHeaderTimeout: durationpb.New(20 * time.Second),
			},
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, fc1.Version)
	require.True(t, fc1.GetDefaults().GetReplication().GetSync().GetEnabled())
	require.EqualValues(t, 2, fc1.GetDefaults().GetReplication().GetSync().GetWriteQuorum())
	require.EqualValues(t, 5, fc1.GetGroups()["canary"].GetHealth().GetFailureThreshold())
	require.Equal(t, 20*time.Second, fc1.GetGroups()["canary"].GetReadHeaderTimeout().AsDuration())

	// Publish proxy status with group, appliedConfigVersion, and pendingRestart.
	pub := proxystatus.NewPublisher(rc, "p-canary", 10*time.Second, func() proxystatus.Status {
		return proxystatus.Status{
			Address:               "http://p-canary",
			Group:                 "canary",
			StartedAt:             time.Now(),
			AppliedConfigVersion:  1,
			PendingRestart:        true,
			PendingRestartReasons: []string{"readHeaderTimeout"},
			Settings: proxystatus.Settings{
				ReadHeaderTimeout: 20 * time.Second,
			},
		}
	}, zerolog.Nop())
	require.NoError(t, pub.PublishOnce(ctx))

	stList, err := c.GetProxyStatus(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 1, stList.ConfigVersion)
	require.Len(t, stList.Proxies, 1)
	require.Equal(t, "canary", stList.Proxies[0].Group)
	require.EqualValues(t, 1, stList.Proxies[0].AppliedConfigVersion)
	require.True(t, stList.Proxies[0].PendingRestart)
	require.Equal(t, []string{"readHeaderTimeout"}, stList.Proxies[0].PendingRestartReasons)
	require.Equal(t, 20*time.Second, stList.Proxies[0].Settings.ReadHeaderTimeout.AsDuration())
}

func TestResilience_ProxyFleetConfigExpectedVersionAndAudit(t *testing.T) {
	ctx := context.Background()
	c, rc, _ := resilienceClient(t)

	thresh := int32(5)
	req := func(expected *int64) *pb.SetProxyFleetConfigRequest {
		return &pb.SetProxyFleetConfigRequest{
			Defaults:        &pb.ProxySettingsOverride{Health: &pb.ProxyHealthSettingsOverride{FailureThreshold: &thresh}},
			ExpectedVersion: expected,
		}
	}
	zero, one := int64(0), int64(1)
	fc1, err := c.SetProxyFleetConfig(ctx, req(&zero))
	require.NoError(t, err)
	require.EqualValues(t, 1, fc1.Version)

	// stale version: rejected, nothing changes
	_, err = c.SetProxyFleetConfig(ctx, req(&zero))
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	cur, err := c.GetProxyFleetConfig(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 1, cur.Version)

	// matching version and unset (no check) both succeed
	fc2, err := c.SetProxyFleetConfig(ctx, req(&one))
	require.NoError(t, err)
	require.EqualValues(t, 2, fc2.Version)
	fc3, err := c.SetProxyFleetConfig(ctx, req(nil))
	require.NoError(t, err)
	require.EqualValues(t, 3, fc3.Version)

	// every successful change is audited with before/after
	recs, err := proxystatus.NewFleetConfigStore(rc).ListAudit(ctx, 10)
	require.NoError(t, err)
	require.Len(t, recs, 3)
	require.EqualValues(t, 3, recs[0].After.Version)
	require.EqualValues(t, 2, recs[0].Before.Version)
	require.NotEmpty(t, recs[0].Actor)
	require.EqualValues(t, 0, recs[2].Before.Version)

	// too-small interval rejected
	tiny := durationpb.New(time.Millisecond)
	_, err = c.SetProxyFleetConfig(ctx, &pb.SetProxyFleetConfigRequest{
		Defaults: &pb.ProxySettingsOverride{ReadyMaxStaleness: tiny},
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestResilience_SystemSettings(t *testing.T) {
	ctx := context.Background()
	c, _, _ := resilienceClient(t)

	// Initial Get returns defaults
	s0, err := c.GetSystemSettings(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.NotNil(t, s0)
	require.False(t, s0.Features.Tagging)
	require.EqualValues(t, 10, s0.Worker.GetConcurrency())

	// Update features & worker settings
	s1, err := c.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{
		Features: &pb.SystemFeaturesSettings{
			Tagging: true,
			Acl:     true,
			Policy:  true,
		},
		Worker: &pb.SystemWorkerSettings{
			Concurrency: proto.Int32(24),
		},
		Lock: &pb.SystemLockSettings{
			Overlap: durationpb.New(3 * time.Second),
		},
		Webhook: &pb.SystemWebhookSettings{
			Enabled: true,
			BaseUrl: "http://webhook.internal",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, s1)
	require.True(t, s1.Features.Tagging)
	require.True(t, s1.Features.Acl)
	require.True(t, s1.Features.Policy)
	require.EqualValues(t, 24, s1.Worker.GetConcurrency())
	require.True(t, s1.Webhook.Enabled)
	require.Equal(t, "http://webhook.internal", s1.Webhook.BaseUrl)

	// Verify in-memory features updated
	require.True(t, features.Tagging(ctx))
	require.True(t, features.ACL(ctx))
	require.True(t, features.Policy(ctx))

	// Get again confirms persistence
	s2, err := c.GetSystemSettings(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.True(t, s2.Features.Tagging)
	require.EqualValues(t, 24, s2.Worker.GetConcurrency())
}

