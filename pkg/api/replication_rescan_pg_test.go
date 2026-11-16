// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package api

import (
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/replscope"
	"github.com/hieunyit/tutti/pkg/store"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

// With the PostgreSQL control plane the rescan interval is stored in the replication status row and the
// mutations are audited (RESCAN_INTERVAL, RESCAN) next to the other replication mutations.
func TestRescanPostgresStorageAndAudit(t *testing.T) {
	dsn := pgtest.NewDB(t)
	ctx := controlplane.WithActor(t.Context(), controlplane.Actor{Name: "alice-admin", RequestID: "req-1"})
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	cpCfg := controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: dsn, AutoMigrate: true}}.WithDefaults()
	cpStore, err := pg.Open(ctx, cpCfg)
	require.NoError(t, err)
	t.Cleanup(func() { cpStore.Close() })

	q := &rsQueue{slQueue: slQueue{rc: rc}, unprocessed: map[string]int{}, paused: map[string]bool{}}
	policySvc := policy.NewService(rc, q, "main").WithControlPlane(cpStore)
	h := PolicyHandlers(slCreds{conn: allConn}, slClients{buckets: []string{"bkt"}}, q, policySvc, meta.NewVersionService(rc),
		store.NewMigrationObjectListStateStore(rc), store.NewMigrationBucketListStateStore(rc), nil,
		store.NewReplicationStatusLocker(rc, time.Second), store.NewUserLocker(rc, time.Second), nil,
		WithPolicyScopeLocker(replscope.NewRedisLocker(rc)), WithPolicyControlPlaneStore(cpStore))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrorInterceptor()))
	pb.RegisterPolicyServer(srv, h)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := pb.NewPolicyClient(conn)

	_, err = client.AddReplication(ctx, &pb.AddReplicationRequest{Id: rsID("a"), Opts: &pb.ReplicationOpts{RescanInterval: durationpb.New(time.Hour)}})
	require.NoError(t, err)
	r, err := client.GetReplication(ctx, rsID("a"))
	require.NoError(t, err)
	require.Equal(t, time.Hour, r.RescanInterval.AsDuration(), "stored in PostgreSQL and read back")

	_, err = client.UpdateReplicationRescanInterval(ctx, &pb.UpdateReplicationRescanIntervalRequest{Id: rsID("a"), RescanInterval: durationpb.New(15 * time.Minute)})
	require.NoError(t, err)
	r, _ = client.GetReplication(ctx, rsID("a"))
	require.Equal(t, 15*time.Minute, r.RescanInterval.AsDuration())

	// Redis projection carries it (the proxies and the Redis fallback read it there)
	id := entityID(t, rsID("a"))
	raw, err := rc.HGet(ctx, "p:repl_status_bucket:"+id.String(), "rescan_seconds").Result()
	if err == nil {
		require.Equal(t, "900", raw)
	}

	_, err = client.RescanReplication(ctx, rsID("a"))
	require.NoError(t, err)

	entries, err := cpStore.Repos().Audit.List(ctx, controlplane.AuditFilter{ObjectType: "replication"})
	require.NoError(t, err)
	var actions []string
	for _, e := range entries {
		actions = append(actions, e.Action)
	}
	require.Contains(t, actions, "RESCAN_INTERVAL")
	require.Contains(t, actions, "RESCAN")
}
