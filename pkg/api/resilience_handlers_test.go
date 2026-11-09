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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/proxystatus"
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
