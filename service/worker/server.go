/*
 * Copyright © 2024 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package worker

import (
	"context"
	"errors"
	"fmt"
	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"sort"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/hieunyit/tutti/pkg/api"
	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/config"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/notifications"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/replication"
	"github.com/hieunyit/tutti/pkg/rpc"
	"github.com/hieunyit/tutti/pkg/s3users"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/trace"
	"github.com/hieunyit/tutti/pkg/util"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/hieunyit/tutti/service/worker/copy"
	"github.com/hieunyit/tutti/service/worker/handler"
	swift_worker "github.com/hieunyit/tutti/service/worker/handler/swift"
)

func Start(ctx context.Context, app dom.AppInfo, conf *Config) error {
	if err := conf.Validate(); err != nil {
		return err
	}
	features.Set(conf.Features)
	logger := log.GetLogger(conf.Log, app.App, app.AppID)
	logger.Info().
		Str("version", app.Version).
		Str("commit", app.Commit).
		Msg("app starting...")

	shutdown, tp, err := trace.NewTracerProvider(conf.Trace, app)
	if err != nil {
		return err
	}
	defer func() {
		_ = shutdown(context.Background())
	}()

	appRedis := util.NewRedisRole(conf.Redis, config.RedisRoleMeta)
	defer appRedis.Close()
	err = appRedis.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("%w: unable to reach app redis", err)
	}
	err = redisotel.InstrumentTracing(appRedis, redisotel.WithTracerProvider(tp))
	if err != nil {
		return fmt.Errorf("%w: unable to instrument tracing app redis", err)
	}
	logger.Info().Msg("app redis connected")

	versionSvc := meta.NewVersionService(appRedis)
	objectListStateStore := store.NewMigrationObjectListStateStore(appRedis)
	bucketListStateStore := store.NewMigrationBucketListStateStore(appRedis)

	confRedis := util.NewRedisRole(conf.Redis, config.RedisRoleConfig)
	defer confRedis.Close()
	err = redisotel.InstrumentTracing(confRedis, redisotel.WithTracerProvider(tp))
	if err != nil {
		return fmt.Errorf("%w: unable to instrument tracing app redis", err)
	}

	var cpStore controlplane.Store
	var cpPublisher *pg.Publisher
	if conf.ControlPlane.WithDefaults().Backend == controlplane.BackendPostgres {
		// Lazy: the worker must start (and serve Redis-backed config) even when PostgreSQL is down at boot. Until the
		// database answers the store is read-only and reads fall back to the Redis projection; readiness is retried below.
		pgStore, err := pg.OpenLazy(ctx, conf.ControlPlane)
		if err != nil {
			return fmt.Errorf("unable to open control-plane postgres store: %w", err)
		}
		defer pgStore.Close()
		cpReady := pgStore.Ready()
		if !cpReady {
			if rerr := pgStore.TryReady(ctx); rerr != nil {
				logger.Error().Err(rerr).Msg("control-plane postgres is not reachable: starting degraded (read-only, Redis projection), retrying in the background")
			}
		}
		metrics.SetControlPlanePostgresReady(pgStore.Ready())
		if !pgStore.Ready() {
			go func() {
				pgStore.RunReadiness(ctx, 5*time.Second, func(err error) {
					if err == nil {
						logger.Info().Msg("control-plane postgres is now ready")
						metrics.SetControlPlanePostgresReady(true)
					}
				})
			}()
		}
		cpStore = pgStore
		cpPublisher = pg.NewPublisher(pgStore, confRedis, conf.ControlPlane.Outbox, logger)
		cpPublisher.MetaRedis = appRedis
		cpPublisher.OnStats = metrics.SetControlPlaneOutbox
		cpPublisher.OnFailure = func(error) { metrics.CountControlPlaneOutboxFailure() }
		logger.Info().Bool("read_only", conf.ControlPlane.ReadOnly).Bool("ready", pgStore.Ready()).Msg("control-plane postgres configured")
	}

	credsOpts := []objstore.CredsSvcOption{objstore.WithManagedStorages(confRedis)}
	if cpStore != nil {
		credsOpts = append(credsOpts, objstore.WithControlPlane(cpStore))
	}
	credsSvc, err := objstore.NewCredsSvc(ctx, &conf.Storage, appRedis, credsOpts...)
	if err != nil {
		return err
	}
	metricsSvc := metrics.NewS3Service(conf.Metrics.Enabled)
	clientRegistry, err := objstore.NewRegistry(ctx, credsSvc, metricsSvc)
	if err != nil {
		return err
	}

	queueRedis := util.NewRedisAsynqRole(conf.Redis, config.RedisRoleQueue)
	taskClient := asynq.NewClient(queueRedis)
	defer taskClient.Close()
	inspector := asynq.NewInspector(queueRedis)
	defer inspector.Close()
	queueSvc := tasks.NewQueueService(taskClient, inspector)
	err = policy.CheckSchemaCompatibility(ctx, app.Version, confRedis)
	if err != nil {
		return err
	}
	policySvc := policy.NewService(confRedis, queueSvc, conf.Storage.Main).WithControlPlane(cpStore)

	limiter := ratelimit.New(appRedis, nil)
	limiter.SetLookup(credsSvc.RateLimit)
	lockRedis := util.NewRedisRole(conf.Redis, config.RedisRoleLock)
	defer lockRedis.Close()
	replicationStatusLocker := store.NewReplicationStatusLocker(lockRedis, conf.Lock.Overlap)
	userLocker := store.NewUserLocker(lockRedis, conf.Lock.Overlap)
	storageLocker := api.NewStorageLocker(lockRedis)
	objectLocker := store.NewObjectLocker(lockRedis, conf.Lock.Overlap)
	bucketLocker := store.NewBucketLocker(lockRedis, conf.Lock.Overlap)

	objectVersionInfoStore := store.NewObjectVersionInfoStore(confRedis)
	copySvc := copy.NewS3CopySvc(credsSvc, clientRegistry, metricsSvc)
	versionedMigrationSvc := handler.NewVersionedMigrationSvc(copySvc, objectVersionInfoStore, objectLocker)

	checkSvc := handler.NewDiffSvc(confRedis, clientRegistry, queueSvc)
	checkCtrl := handler.NewDiffCtrl(checkSvc, queueSvc)

	accessPolicyStore := accesspolicy.NewStore(confRedis).WithControlPlane(cpStore)
	bucketScope := accesspolicy.NewScope(func(storage, user string) bool {
		return credsSvc.CredentialMode(storage, user) == objstore.ModeConnection
	}, accessPolicyStore)
	groupStore := protectiongroups.NewStore(confRedis).WithControlPlane(cpStore)
	groupStore.SharedAccount = func(storage, user string) bool {
		return credsSvc.CredentialMode(storage, user) == objstore.ModeConnection
	}
	groupService := &protectiongroups.Service{Store: groupStore, Backend: policySvc.ProtectionBackend(),
		Resyncer:            protectiongroups.NewS3Resyncer(clientRegistry, bucketScope),
		ValidateCredentials: protectiongroups.CredentialValidator(credsSvc, accessPolicyStore),
		ValidateLegacyFailover: protectiongroups.LegacyFailoverValidator(
			failover.NewStore(confRedis, nil).WithControlPlane(cpStore).ListFailoverPolicies),
		Reports: func(ctx context.Context) ([]proxystatus.Status, error) { return proxystatus.List(ctx, confRedis) }}
	workerSvc := handler.New(conf.Worker, credsSvc, clientRegistry, versionSvc, copySvc, queueSvc, limiter, objectListStateStore, objectLocker, bucketLocker, replicationStatusLocker, versionedMigrationSvc)

	workerSvc.SetBucketScope(bucketScope)

	stdLogger := log.NewStdLogger()
	redis.SetLogger(stdLogger)

	defaultRetry := fallbackRetryDelay(conf.Worker.CustomErrRetryInterval)
	srv := asynq.NewServer(
		queueRedis,
		asynq.Config{
			ShutdownTimeout: conf.ShutdownTimeout,
			Concurrency:     conf.Concurrency,
			IsFailure: func(err error) bool {
				var rlErr *dom.ErrRateLimitExceeded
				return !errors.As(err, &rlErr)
			},
			RetryDelayFunc: retryDelayFunc(defaultRetry),
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)
				taskID, _ := asynq.GetTaskID(ctx)
				queue, _ := asynq.GetQueueName(ctx)
				taskLogger := zerolog.Ctx(ctx).With().Str("task_type", task.Type()).Str("task_id", taskID).Str("task_queue", queue).Int("task_max_retry", maxRetry).Int("task_retried", retried).Logger()
				if retried >= maxRetry {
					taskLogger.Error().RawJSON("task_payload", task.Payload()).Err(err).Msg("process task failed. task will be dropped")
					return
				}
				var rlErr *dom.ErrRateLimitExceeded
				if errors.As(err, &rlErr) {
					taskLogger.Debug().Err(err).Msg("process task failed due to the rate limit")
					return
				}
				taskLogger.Warn().Err(err).Msg("process task failed. task will be retried")
			}),
			Logger:                     stdLogger,
			LogLevel:                   asynq.LogLevel(zerolog.GlobalLevel() + 1),
			Queues:                     tasks.Priority,
			StrictPriority:             true,
			DynamicQueues:              true,
			DynamicQueueUpdateInterval: conf.Worker.QueueUpdateInterval,
			TaskCheckInterval:          conf.Worker.TaskCheckInterval,
			DelayedTaskCheckInterval:   conf.Worker.DelayedTaskCheckInterval,
		},
	)

	// mux maps a type to a handler
	mux := asynq.NewServeMux()
	mux.Use(log.WorkerMiddleware(conf.Log, app.App, app.AppID))
	mux.Use(trace.WorkerMiddleware(tp))
	if conf.Metrics.Enabled {
		mux.Use(metrics.WorkerMiddleware())
	}
	groupLease := groupStore.NewLease(app.AppID, true, protectiongroups.DefaultWriterLease)
	// keep the lease alive until Start returns, i.e. after in-flight tasks finished on shutdown
	leaseCtx, leaseStop := context.WithCancel(context.WithoutCancel(ctx))
	defer leaseStop()
	go groupLease.Run(leaseCtx)
	mux.Use(groupStore.WorkerMiddleware(groupLease))
	mux.Use(tasks.DropStaleReplicationTasks(policySvc.GetReplicationGeneration))
	// common workers
	switchWorker := handler.NewSwitchSvc(conf.Worker, policySvc, clientRegistry, replicationStatusLocker)
	switchWorker.SetBucketScope(bucketScope)
	mux.HandleFunc(tasks.TypeApiZeroDowntimeSwitch, switchWorker.HandleZeroDowntimeReplicationSwitch)
	mux.HandleFunc(tasks.TypeApiSwitchWithDowntime, switchWorker.HandleSwitchWithDowntime)
	logger.Info().Msg("registered common workers")

	// S3 workers
	if len(conf.Storage.S3Storages()) != 0 {
		mux.HandleFunc(tasks.TypeBucketCreate, workerSvc.HandleBucketCreate)
		mux.HandleFunc(tasks.TypeBucketDelete, workerSvc.HandleBucketDelete)
		mux.HandleFunc(tasks.TypeBucketSyncTags, workerSvc.HandleBucketTags)
		mux.HandleFunc(tasks.TypeBucketSyncACL, workerSvc.HandleBucketACL)
		mux.HandleFunc(tasks.TypeObjectSync, workerSvc.HandleObjectSync)
		mux.HandleFunc(tasks.TypeObjectSyncTags, workerSvc.HandleObjectTags)
		mux.HandleFunc(tasks.TypeObjectSyncACL, workerSvc.HandleObjectACL)
		mux.HandleFunc(tasks.TypeMigrateS3User, workerSvc.HandleMigrationS3User)
		mux.HandleFunc(tasks.TypeMigrateBucketListObjects, workerSvc.HandleMigrationBucketListObj)
		mux.HandleFunc(tasks.TypeMigrateObjCopy, workerSvc.HandleMigrationObjCopy)
		logger.Info().Msg("registered S3 workers")

		// versioned object migration workers
		mux.HandleFunc(tasks.TypeMigrateObjectListVersions, workerSvc.HandleObjectVersionList)
		mux.HandleFunc(tasks.TypeMigrateVersionedObject, workerSvc.HandleVersionedObjectMigration)
		logger.Info().Msg("registered S3 versioned workers")

		mux.HandleFunc(tasks.TypeDiffFixCopyS3, workerSvc.HandleMigrationObjCopy)
		mux.HandleFunc(tasks.TypeDiffFixS3ListVersions, workerSvc.HandleObjectVersionList)
		logger.Info().Msg("registered diff fix s3 workers")
	} else {
		logger.Info().Msg("s3 workers not registered: no s3 storage configured")
	}

	// swift workers
	if len(conf.Storage.SwiftStorages()) != 0 {
		swiftWorkerSvc := swift_worker.New(conf.Worker, clientRegistry, bucketListStateStore, objectListStateStore, queueSvc, limiter, objectLocker, userLocker, bucketLocker)
		// swift tasks:
		mux.HandleFunc(tasks.TypeSwiftAccountUpdate, swiftWorkerSvc.HandleAccountUpdate)
		mux.HandleFunc(tasks.TypeSwiftContainerUpdate, swiftWorkerSvc.HandleContainerUpdate)
		mux.HandleFunc(tasks.TypeSwiftObjUpdate, swiftWorkerSvc.HandleObjectUpdate)
		mux.HandleFunc(tasks.TypeSwiftObjMetaUpdate, swiftWorkerSvc.HandleObjectMetaUpdate)
		mux.HandleFunc(tasks.TypeSwiftObjDelete, swiftWorkerSvc.HandleObjectDelete)
		mux.HandleFunc(tasks.TypeSwiftAccountMigration, swiftWorkerSvc.HandleSwiftAccountMigration)
		mux.HandleFunc(tasks.TypeSwiftContainerMigration, swiftWorkerSvc.HandleSwiftContainerMigration)
		mux.HandleFunc(tasks.TypeSwiftObjectMigration, swiftWorkerSvc.HandleSwiftObjectMigration)
		logger.Info().Msg("registered swift workers")

		mux.HandleFunc(tasks.TypeDiffFixCopySwift, swiftWorkerSvc.HandleSwiftObjectMigration)
		logger.Info().Msg("registered diff fix swift workers")
	} else {
		logger.Info().Msg("swift workers not registered: no swift storage configured")
	}

	// diff check
	mux.HandleFunc(tasks.TypeDiff, checkCtrl.HandleDiff)
	mux.HandleFunc(tasks.TypeDiffListObjects, checkCtrl.HandleDiffList)
	mux.HandleFunc(tasks.TypeDiffListVersions, checkCtrl.HandleDiffListVersions)
	logger.Info().Msg("registered diff workers")

	// diff fix
	mux.HandleFunc(tasks.TypeDiffFixCollectObjects, checkCtrl.HandleDiffCollectObjects)
	mux.HandleFunc(tasks.TypeDiffFixRemoveObjects, checkCtrl.HandleDiffRemoveObjects)
	mux.HandleFunc(tasks.TypeDiffFixEnsureRemove, checkCtrl.HandleDiffEnsureObjectsRemoved)
	logger.Info().Msg("registered diff fix workers")

	server := util.NewServer()
	if cpPublisher != nil {
		err = server.Add("controlplane_outbox", func(ctx context.Context) error { cpPublisher.Run(ctx); return nil }, func(context.Context) error { return nil })
		if err != nil {
			return err
		}
	}
	// Tell the proxies which control plane is authoritative: with postgres they delegate automatic failover to the
	// workers instead of writing routing to Redis themselves (see pkg/failover/remote.go).
	cpBackend := conf.ControlPlane.WithDefaults().Backend
	if err := failover.PublishBackend(ctx, confRedis, cpBackend); err != nil {
		return fmt.Errorf("unable to publish the control-plane backend: %w", err)
	}
	if cpStore != nil {
		err = server.Add("failover_executor", func(ctx context.Context) error {
			failover.RunExecutor(ctx, confRedis, policySvc, logger)
			return nil
		}, func(context.Context) error { return nil })
		if err != nil {
			return err
		}
	}
	err = server.Add("protection_groups", func(ctx context.Context) error { groupService.Run(ctx, time.Second); return nil }, func(context.Context) error { return nil })
	if err != nil {
		return err
	}
	// Every user that may own bucket replications: users with credentials on a storage, all identities,
	// users with routing and protection group users (which need not be identities). Duplicates are skipped.
	users := func() []string {
		var out []string
		for name := range credsSvc.Storages() {
			out = append(out, credsSvc.ListUsers(name)...)
		}
		out = append(out, credsSvc.Identities()...)
		if routes, err := policySvc.GetUserRoutings(ctx); err == nil {
			for u := range routes {
				out = append(out, u)
			}
		}
		if groups, err := groupStore.List(ctx); err == nil {
			for _, g := range groups {
				out = append(out, g.User)
			}
		}
		return out
	}
	failoverStore := failover.NewStore(confRedis, func(name string) bool { _, ok := credsSvc.Storages()[name]; return ok }).WithControlPlane(cpStore)
	unprotectedOpts := func() protectiongroups.UnprotectedOptions {
		var storages []string
		for name := range credsSvc.Storages() {
			storages = append(storages, name)
		}
		sort.Strings(storages)
		return protectiongroups.UnprotectedOptions{Storages: storages, MainStorage: conf.Storage.Main, Users: users,
			Groups: groupStore.List, Policies: failoverStore.ListFailoverPolicies}
	}
	migrator := &protectiongroups.Migrator{Service: groupService, Policies: failoverStore, Source: policySvc,
		MainStorage: conf.Storage.Main, Users: users, Log: logger,
		Electorate: func(ctx context.Context) []string {
			list, err := proxystatus.List(ctx, confRedis)
			if err != nil {
				return nil
			}
			var ids []string
			for _, st := range list {
				ids = append(ids, st.AppID)
			}
			sort.Strings(ids)
			return ids
		}}
	if conf.Metrics.Enabled {
		err = server.Add("unprotected_scopes_metrics", func(ctx context.Context) error {
			protectiongroups.RunUnprotectedMetrics(ctx, policySvc, unprotectedOpts, 30*time.Second, func(err error) {
				logger.Warn().Err(err).Msg("unable to collect unprotected scope metrics")
			})
			return nil
		}, func(context.Context) error { return nil })
		if err != nil {
			return err
		}
		err = server.Add("replication_metrics", func(ctx context.Context) error {
			policy.RunReplicationMetrics(ctx, policySvc, users, 15*time.Second, func(err error) {
				logger.Warn().Err(err).Msg("unable to collect replication metrics")
			})
			return nil
		}, func(context.Context) error { return nil })
		if err != nil {
			return err
		}
	}
	err = server.Add("queue_workers", func(ctx context.Context) error {
		err := srv.Start(mux)
		if err != nil {
			return err
		}
		<-ctx.Done()
		return nil
	}, func(_ context.Context) error {
		srv.Stop()
		srv.Shutdown()
		return nil
	})
	if err != nil {
		return err
	}

	if conf.Api.Enabled {
		if conf.Api.Secure {
			return fmt.Errorf("api.secure is unsupported by the embedded API listeners")
		}
		chorusHandler := api.ChorusHandlers(credsSvc, rpc.NewProxyClient(appRedis), &app)
		diffOpts := []api.DiffOption{}
		policyOpts := []api.PolicyOption{
			api.WithPolicyStorageLocker(storageLocker),
			api.WithPolicyScope(bucketScope),
		}
		if cpStore != nil {
			diffOpts = append(diffOpts, api.WithDiffControlPlaneStore(cpStore))
			policyOpts = append(policyOpts, api.WithPolicyControlPlaneStore(cpStore))
		}
		diffHandler := api.DiffHandlers(credsSvc, checkSvc, diffOpts...)
		var webhookConf *api.WebhookConfig
		if conf.Api.Webhook.Enabled {
			webhookConf = &conf.Api.Webhook
		}
		policyHandler := api.PolicyHandlers(credsSvc, clientRegistry, queueSvc, policySvc, versionSvc, objectListStateStore, bucketListStateStore, notifications.NewService(clientRegistry), replicationStatusLocker, userLocker, webhookConf, policyOpts...)

		webhookSharesMainPort := conf.Api.Webhook.Enabled && conf.Api.Webhook.GrpcPort <= 0
		var authOpts []auth.Option
		authOpts = append(authOpts, auth.WithManagedUsers(confRedis))
		if cpStore != nil {
			authOpts = append(authOpts, auth.WithControlPlane(cpStore))
		}
		if webhookSharesMainPort && conf.Api.Webhook.Token == "" {
			logger.Warn().Msg("webhook api is served on the main api port WITHOUT api.webhook.token: anyone who can reach the api can inject fake bucket events. Set api.webhook.token or use separate webhook ports")
			authOpts = append(authOpts, auth.WithWarnings(auth.WarningWebhookUnauthenticated))
		}
		authSvc, err := auth.NewService(conf.Api.Auth, appRedis, authOpts...)
		if err != nil {
			return err
		}
		webhookTokenUnary, webhookTokenStream := auth.WebhookTokenInterceptors(conf.Api.Webhook.Token)
		if !authSvc.Enabled() {
			logger.Warn().Msg("management api is enabled WITHOUT authentication (api.auth.enabled=false): anyone who can reach the api ports has full admin access")
		}

		resilienceHandler := api.ResilienceHandlers(confRedis, func(storage string) bool {
			_, ok := credsSvc.Storages()[storage]
			return ok
		}, api.WithProxyAccounts(func() (proxyaccount.Sealer, error) {
			if err := credsSvc.SealingAvailable(); err != nil {
				return nil, err
			}
			return credsSvc, nil
		}, func(user string) bool {
			// dynamic proxy accounts act as a storage user of the main storage
			return credsSvc.HasUser(credsSvc.MainStorage(), user) == nil
		}), api.WithS3Users(credsSvc, s3users.MinioVerifier(credsSvc.GetS3Address),
			func(events *failover.Store) s3users.References {
				return func(ctx context.Context, user string) ([]string, error) {
					reasons, err := s3users.PolicyReferences(policySvc, events)(ctx, user)
					if err != nil {
						return nil, err
					}
					groups, err := groupStore.References(ctx, user, "")
					return append(reasons, groups...), err
				}
			}),
			api.WithConsoleUsers(authSvc), api.WithRuntimeStorages(credsSvc, api.RuntimeStorageReferences(policySvc, credsSvc, confRedis, appRedis)),
			api.WithStorageLocker(storageLocker), api.WithControlPlane(cpStore, conf.ControlPlane))

		webhookEnabled := conf.Api.Webhook.Enabled
		webhookOnSeparatePorts := webhookEnabled && conf.Api.Webhook.GrpcPort > 0

		var webhookHandler pb.WebhookServer
		if webhookEnabled {
			swiftReplSvc := replication.NewSwift(queueSvc, policySvc)
			s3ReplSvc := replication.NewS3(queueSvc, versionSvc, policySvc)
			webhookHandler = api.WebhookHandlers(credsSvc, policySvc, swiftReplSvc, s3ReplSvc)
		}

		registerServices := func(srv *grpc.Server) {
			pb.RegisterProtectionGroupsServer(srv, api.ProtectionGroupHandlers(groupService, storageLocker, userLocker, api.WithFailoverMigration(migrator, unprotectedOpts)))
			pb.RegisterChorusServer(srv, chorusHandler)
			pb.RegisterDiffServer(srv, diffHandler)
			pb.RegisterPolicyServer(srv, policyHandler)
			pb.RegisterResilienceServer(srv, resilienceHandler)
			if webhookEnabled && !webhookOnSeparatePorts {
				pb.RegisterWebhookServer(srv, webhookHandler)
			}
		}
		start, stop, err := api.NewGrpcServer(conf.Api.GrpcPort, registerServices, tp, conf.Log, app,
			api.WithAuthInterceptors(webhookTokenUnary, webhookTokenStream),
			api.WithAuthInterceptors(authSvc.UnaryInterceptor(), authSvc.StreamInterceptor()))
		if err != nil {
			return err
		}
		err = server.Add("grpc_api", start, stop)
		if err != nil {
			return err
		}
		start, stop, err = api.GRPCGateway(ctx, conf.Api, func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
			if err := authSvc.RegisterHTTP(mux); err != nil {
				return err
			}
			if err := pb.RegisterChorusHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
				return err
			}
			if err := pb.RegisterDiffHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
				return err
			}
			if err := pb.RegisterPolicyHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
				return err
			}
			if err := pb.RegisterResilienceHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
				return err
			}
			if err := pb.RegisterProtectionGroupsHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
				return err
			}
			if webhookEnabled && !webhookOnSeparatePorts {
				if err := pb.RegisterWebhookHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		err = server.Add("http_api", start, stop)
		if err != nil {
			return err
		}
		logger.Info().Msg("management api created")

		if webhookOnSeparatePorts {
			start, stop, err = api.NewGrpcServer(conf.Api.Webhook.GrpcPort, func(srv *grpc.Server) {
				pb.RegisterWebhookServer(srv, webhookHandler)
			}, tp, conf.Log, app, api.WithAuthInterceptors(webhookTokenUnary, webhookTokenStream))
			if err != nil {
				return err
			}
			err = server.Add("webhook_grpc", start, stop)
			if err != nil {
				return err
			}

			webhookApiConf := &api.Config{
				GrpcPort: conf.Api.Webhook.GrpcPort,
				HttpPort: conf.Api.Webhook.HttpPort,
				Secure:   conf.Api.Secure,
			}
			start, stop, err = api.GRPCGateway(ctx, webhookApiConf, pb.RegisterWebhookHandlerFromEndpoint)
			if err != nil {
				return err
			}
			err = server.Add("webhook_http", start, stop)
			if err != nil {
				return err
			}
			logger.Info().Int("grpc_port", conf.Api.Webhook.GrpcPort).Int("http_port", conf.Api.Webhook.HttpPort).Msg("webhook api created on separate ports")
		}
	}

	if conf.Metrics.Enabled {
		start, stop := metrics.Server(ctx, conf.Metrics.Port, app)
		err = server.Add("worker_metrics", start, stop)
		if err != nil {
			return err
		}
		zerolog.Ctx(ctx).Info().Msg("metrics enabled")
	}

	zerolog.Ctx(ctx).Info().Msg("starting workers...")
	return server.Start(ctx)
}

func retryDelayFunc(fallback asynq.RetryDelayFunc) asynq.RetryDelayFunc {
	return func(n int, err error, task *asynq.Task) time.Duration {
		var rlErr *dom.ErrRateLimitExceeded
		if errors.As(err, &rlErr) {
			return rlErr.RetryIn
		}
		return fallback(n, err, task)
	}
}

func fallbackRetryDelay(customInterval *time.Duration) asynq.RetryDelayFunc {
	if customInterval == nil || *customInterval <= 0 {
		return asynq.DefaultRetryDelayFunc
	}
	return func(_ int, _ error, _ *asynq.Task) time.Duration {
		return *customInterval
	}
}
