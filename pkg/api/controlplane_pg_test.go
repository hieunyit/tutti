// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3users"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

func TestControlPlanePostgresWiringAndFallback(t *testing.T) {
	dsn := pgtest.NewDB(t)
	ctx := context.Background()
	ctx = xctx.SetTraceID(ctx, "req-trace-123")
	ctx = controlplane.WithActor(ctx, controlplane.Actor{Name: "alice-admin", RequestID: "req-trace-123"})

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cpCfg := controlplane.Config{
		Backend: controlplane.BackendPostgres,
		Postgres: controlplane.PostgresConfig{
			DSN:         dsn,
			AutoMigrate: true,
		},
	}.WithDefaults()
	cpStore, err := pg.Open(ctx, cpCfg)
	require.NoError(t, err)
	t.Cleanup(func() { cpStore.Close() })

	pub := pg.NewPublisher(cpStore, rdb, cpCfg.Outbox, zerolog.Nop())
	pub.MetaRedis = rdb

	stConf := &objstore.Config{
		Main: "main",
		DynamicCredentials: objstore.DynamicCredentialsConfig{
			Enabled:        true,
			MasterPassword: "super-secret-master-password",
			PollInterval:   time.Hour,
		},
		Storages: map[string]objstore.Storage{
			"main": {
				CommonConfig: objstore.CommonConfig{Type: dom.S3},
				S3: &s3.Storage{
					StorageAddress: s3.StorageAddress{Address: "http://127.0.0.1:9000", Provider: "Minio"},
					Credentials: map[string]s3.CredentialsV4{
						"staticuser": {AccessKeyID: "AKIASTATIC", SecretAccessKey: "SECRETSTATIC"},
					},
				},
			},
		},
	}
	require.NoError(t, stConf.Validate())

	credsSvc, err := objstore.NewCredsSvc(ctx, stConf, rdb, objstore.WithManagedStorages(rdb), objstore.WithControlPlane(cpStore))
	require.NoError(t, err)

	qMock := &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	policySvc := policy.NewService(rdb, qMock, "main").WithControlPlane(cpStore)
	pgGroupStore := protectiongroups.NewStore(rdb).WithControlPlane(cpStore)
	storageLocker := NewStorageLocker(rdb)

	authSvc, err := auth.NewService(auth.Config{
		Enabled: true,
		Users: []auth.User{
			{Username: "root", PasswordHash: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", Role: auth.RoleAdmin},
		},
	}, rdb, auth.WithManagedUsers(rdb), auth.WithControlPlane(cpStore))
	require.NoError(t, err)

	verifyNoop := func(context.Context, string, s3.CredentialsV4) error { return nil }
	h := ResilienceHandlers(rdb, func(storage string) bool {
		_, ok := credsSvc.Storages()[storage]
		return ok
	},
		WithProxyAccounts(func() (proxyaccount.Sealer, error) { return credsSvc, nil }, func(user string) bool {
			return credsSvc.HasUser("main", user) == nil
		}),
		WithS3Users(credsSvc, verifyNoop, func(events *failover.Store) s3users.References {
			return func(ctx context.Context, user string) ([]string, error) {
				reasons, err := s3users.PolicyReferences(policySvc, events)(ctx, user)
				if err != nil {
					return nil, err
				}
				groups, err := pgGroupStore.References(ctx, user, "")
				return append(reasons, groups...), err
			}
		}),
		WithConsoleUsers(authSvc),
		WithRuntimeStorages(credsSvc, RuntimeStorageReferences(policySvc, credsSvc, rdb, rdb)),
		WithStorageLocker(storageLocker),
		WithControlPlane(cpStore, cpCfg),
	)

	// 1. Create a runtime storage and an S3 user with credentials, policy, and proxy key
	_, err = h.CreateRuntimeStorage(ctx, &pb.CreateRuntimeStorageRequest{
		Storage: &pb.RuntimeStorage{
			Name:     "backup",
			Address:  "http://127.0.0.1:9001",
			Provider: "Minio",
		},
		Credentials: []*pb.RuntimeStorageCredential{
			{User: "staticuser", AccessKeyId: "AKIABACKUP", SecretAccessKey: "SECRETBACKUP"},
		},
		Force: true,
	})
	require.NoError(t, err)

	createdUser, err := h.CreateS3User(ctx, &pb.CreateS3UserRequest{
		Name: "dynuser",
		Credentials: []*pb.S3UserCredential{
			{Storage: "main", AccessKeyId: "AKIADYNMAIN", SecretAccessKey: "SECRETDYNMAIN"},
			{Storage: "backup", AccessKeyId: "AKIADYNBAK", SecretAccessKey: "SECRETDYNBAK"},
		},
		CreateAccessKey: true,
		Description:     "dynuser key",
		Policy: &pb.AccessPolicy{
			User:       "dynuser",
			Permission: accesspolicy.ReadWrite,
			Buckets:    []string{"orders-*"},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, createdUser.GetAccessKey().GetSecretAccessKey())

	// 2. Create routing & replication policy via policySvc (transactional in PG)
	gen, err := policySvc.AddBucketReplicationPolicy(ctx, entity.BucketReplicationPolicy{
		User:        "dynuser",
		FromStorage: "main",
		FromBucket:  "orders-1",
		ToStorage:   "backup",
		ToBucket:    "orders-1-dr",
	}, entity.ReplicationOptions{})
	require.NoError(t, err)
	require.Equal(t, int64(1), gen)

	// Custom destination bucket should be routing-blocked in PG
	blocks, err := policySvc.GetBucketRoutingBlocks(ctx, "dynuser")
	require.NoError(t, err)
	require.True(t, blocks["orders-1-dr"])

	// 3. Reference check prevents deleting referenced storage "backup" or referenced user "dynuser"
	_, err = h.DeleteRuntimeStorage(ctx, &pb.RuntimeStorageName{Name: "backup"})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)

	_, err = h.DeleteS3User(ctx, &pb.DeleteS3UserRequest{Name: "dynuser", Force: true})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)

	// 4. Delete replication & routing, then delete "dynuser" and "backup" transactionally
	require.NoError(t, policySvc.DeleteBucketReplication(ctx, entity.BucketReplicationPolicy{
		User:        "dynuser",
		FromStorage: "main",
		FromBucket:  "orders-1",
		ToStorage:   "backup",
		ToBucket:    "orders-1-dr",
	}))
	require.NoError(t, policySvc.DeleteBucketRouting(ctx, entity.NewBucketRoutingPolicyID("dynuser", "orders-1")))

	delUserResp, err := h.DeleteS3User(ctx, &pb.DeleteS3UserRequest{Name: "dynuser", Force: true})
	require.NoError(t, err)
	require.Equal(t, int32(1), delUserResp.DeletedAccessKeys)
	require.True(t, delUserResp.DeletedPolicy)

	// Re-create an access policy & failover policy so we can test outbox projection + Redis fallback
	_, err = h.SetAccessPolicy(ctx, &pb.AccessPolicy{
		User:       "staticuser",
		Permission: accesspolicy.ReadOnly,
		Buckets:    []string{"*"},
	})
	require.NoError(t, err)
	_, err = h.SetFailoverPolicy(ctx, &pb.FailoverPolicy{
		Source:  "main",
		Enabled: true,
		Targets: []string{"backup"},
		After:   durationpb.New(30 * time.Second),
	})
	require.NoError(t, err)

	// Save a protection group desired spec in PG + runtime promotion in Redis
	pgGroup := protectiongroups.Group{
		ID:      "pg-orders",
		User:    "staticuser",
		Members: []string{"main", "backup"},
		Copies:  2,
		Primary: "main",
		Mode:    "manual",
		State:   "ready",
	}
	require.NoError(t, pgGroupStore.Save(ctx, &pgGroup))

	// 5. Verify Audit Log RPC records actor, request_id, and redacts secrets
	auditResp, err := h.ListAuditLog(ctx, &pb.ListAuditLogRequest{Actor: "alice-admin", Limit: 50})
	require.NoError(t, err)
	require.True(t, auditResp.Enabled)
	require.Equal(t, controlplane.BackendPostgres, auditResp.Backend)
	require.NotEmpty(t, auditResp.Entries)
	for _, entry := range auditResp.Entries {
		require.Equal(t, "alice-admin", entry.Actor)
		require.Equal(t, "req-trace-123", entry.RequestId)
		require.NotContains(t, entry.AfterJson, "SECRETDYNMAIN")
		require.NotContains(t, entry.AfterJson, "SECRETBACKUP")
		require.NotContains(t, entry.AfterJson, createdUser.GetAccessKey().GetSecretAccessKey())
	}

	// the action filter runs in SQL: a rare action is found even when newer rows of other actions exceed the limit
	createsOnly, err := h.ListAuditLog(ctx, &pb.ListAuditLogRequest{Action: "CREATE", Limit: 1})
	require.NoError(t, err)
	require.Len(t, createsOnly.Entries, 1)
	require.Equal(t, "create", createsOnly.Entries[0].Action)

	// Update system settings in PostgreSQL
	settingsResp, err := h.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{
		Features: &pb.SystemFeaturesSettings{Tagging: true, Acl: true},
		Worker:   &pb.SystemWorkerSettings{Concurrency: 32},
	})
	require.NoError(t, err)
	require.EqualValues(t, 32, settingsResp.Worker.Concurrency)
	require.True(t, settingsResp.Features.Tagging)

	// 6. Drain outbox to Redis so Redis has the warm projection
	_, err = pub.RunOnce(ctx)
	require.NoError(t, err)

	// Redis has the projected system settings
	rawSet, err := rdb.Get(ctx, pg.KeySystemSettings).Result()
	require.NoError(t, err)
	require.Contains(t, rawSet, `"concurrency":32`)

	// 7. Read-only mode returns 503 Unavailable (ErrReadOnly) on mutations while reads still succeed
	cpStore.SetReadOnly(true)
	_, err = h.SetAccessPolicy(ctx, &pb.AccessPolicy{
		User:       "staticuser",
		Permission: accesspolicy.ReadWrite,
		Buckets:    []string{"*"},
	})
	require.ErrorIs(t, err, controlplane.ErrReadOnly)
	stErr := convertApiError(ctx, err)
	require.Equal(t, codes.Unavailable, status.Code(stErr))
	cpStore.SetReadOnly(false)

	// 8. Close PostgreSQL store: reads fall back to the Redis projection, writes fail with 503 Unavailable
	cpStore.Close()

	polList, err := h.ListAccessPolicies(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, polList.Policies, 1)
	require.Equal(t, "staticuser", polList.Policies[0].User)

	fList, err := h.ListFailoverPolicies(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, fList.Policies, 1)
	require.Equal(t, "main", fList.Policies[0].Source)

	gotGroup, err := pgGroupStore.Get(ctx, "pg-orders")
	require.NoError(t, err)
	require.Equal(t, "main", gotGroup.Primary)

	// Fallback read from Redis
	gotSettings, err := h.GetSystemSettings(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 32, gotSettings.Worker.Concurrency)

	// Write while PG is down returns ErrUnavailable -> gRPC Unavailable (503)
	_, err = h.SetAccessPolicy(ctx, &pb.AccessPolicy{
		User:       "staticuser",
		Permission: accesspolicy.ReadWrite,
		Buckets:    []string{"*"},
	})
	require.ErrorIs(t, err, dom.ErrUnavailable)
	require.Equal(t, codes.Unavailable, status.Code(convertApiError(ctx, err)))

	// the audit log cannot be read while PostgreSQL is down: 503, not an internal error
	_, err = h.ListAuditLog(ctx, &pb.ListAuditLogRequest{Limit: 5})
	require.ErrorIs(t, err, dom.ErrUnavailable)
	require.Equal(t, codes.Unavailable, status.Code(convertApiError(ctx, err)))
}
