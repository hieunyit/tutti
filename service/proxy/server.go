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

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/config"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/replication"
	"github.com/hieunyit/tutti/pkg/rpc"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/storage"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/trace"
	"github.com/hieunyit/tutti/pkg/trafficstats"
	"github.com/hieunyit/tutti/pkg/util"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/hieunyit/tutti/service/proxy/access"
	"github.com/hieunyit/tutti/service/proxy/auth"
	"github.com/hieunyit/tutti/service/proxy/cors"
	failovercontroller "github.com/hieunyit/tutti/service/proxy/failover"
	"github.com/hieunyit/tutti/service/proxy/router"
	"github.com/hieunyit/tutti/service/worker/copy"
	"github.com/hieunyit/tutti/service/worker/syncer"
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
	redis.SetLogger(log.NewStdLogger())
	logger.Info().Msg("app redis connected")

	verSvc := meta.NewVersionService(appRedis)
	uploadSvc := storage.NewUploadSvc(appRedis)
	limiter := ratelimit.New(appRedis, conf.Storage.RateLimitConf())

	confRedis := util.NewRedisRole(conf.Redis, config.RedisRoleConfig)
	defer confRedis.Close()
	err = redisotel.InstrumentTracing(confRedis, redisotel.WithTracerProvider(tp))
	if err != nil {
		return fmt.Errorf("%w: unable to instrument tracing app redis", err)
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
	policySvc := policy.NewService(confRedis, queueSvc, conf.Storage.Main)
	// the snapshot keeps running while the listener drains, so it has its own context
	snapCtx, snapStop := context.WithCancel(context.WithoutCancel(ctx))
	defer snapStop()
	if !conf.Snapshot.Disabled {
		if err := policySvc.EnableSnapshot(snapCtx, policy.SnapshotConfig{
			PollInterval: conf.Snapshot.PollInterval,
			FullReload:   conf.Snapshot.FullReload,
			Logger:       logger,
		}); err != nil {
			return fmt.Errorf("unable to load policy snapshot: %w", err)
		}
		logger.Info().Msg("policy snapshot enabled")
	}
	storagesKnown := func() int { return len(conf.Storage.Storages) }

	startupSettings := buildProxySettings(conf)
	group := conf.EffectiveGroup()
	fleetStore := proxystatus.NewFleetConfigStore(confRedis)
	initialFC, err := fleetStore.Get(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("unable to load initial proxy fleet config, using startup settings")
		initialFC = &proxystatus.FleetConfig{Version: 0, Groups: map[string]proxystatus.SettingsOverride{}}
	}
	initEff, initPending, initReasons := proxystatus.Resolve(startupSettings, group, initialFC)
	cfgState := &fleetConfigState{
		settings:       initEff,
		version:        initialFC.Version,
		pendingRestart: initPending,
		reasons:        initReasons,
	}

	// configure object storage proxies:
	proxyConfig := router.Config{
		Storages:          map[dom.StorageType]router.StorageProxy{},
		LogMiddleware:     log.HttpMiddleware(conf.Log, app.App, app.AppID, xctx.Event),
		TraceMiddleware:   nil,
		MetricsMiddleware: nil,
		PolicyMiddleware:  router.PolicyMiddleware(policySvc),
		Replication: router.ReplicationOptions{
			OnEventError:     initEff.Replication.OnEventError,
			EventRetryBudget: initEff.Replication.EventRetryBudget,
		},
	}
	if conf.Metrics.Enabled {
		proxyConfig.MetricsMiddleware = metrics.ProxyMiddleware()
	}
	if conf.Trace.Enabled {
		proxyConfig.TraceMiddleware = trace.HttpMiddleware(tp)
	}
	if conf.Cors.Enabled {
		proxyConfig.CorsMiddleware = cors.HttpMiddleware(conf.Cors)
	}
	var (
		healthReg    *health.Registry
		failoverCtrl *failovercontroller.Controller
		statusPub    *proxystatus.Publisher
		updateAuth   func(bool)
	)
	if s3Conf := conf.Storage.S3Storages(); len(s3Conf) != 0 {
		metricsSvc := metrics.NewS3Service(conf.Metrics.Enabled)
		credsConf, err := ProxyToCredsConf(conf.Storage)
		if err != nil {
			return err
		}
		credsSvc, err := objstore.NewCredsSvc(ctx, &credsConf, appRedis, objstore.WithManagedStorages(confRedis))
		if err != nil {
			return err
		}
		policySvc.WithSharedAccount(func(storage, user string) bool {
			if credsSvc.CredentialMode(storage, user) == objstore.ModeConnection {
				return true
			}
			_, ok := credsSvc.ConnectionAccount(storage)
			return ok
		})
		healthReg = health.NewRegistry(health.Config{
			FailureThreshold: initEff.Health.FailureThreshold,
			OpenDuration:     initEff.Health.OpenDuration,
		}, health.WithLogger(logger), health.WithOnChange(func(storage string, available bool) {
			if conf.Metrics.Enabled {
				metrics.SetStorageAvailable(storage, available)
				metrics.CountBreakerTransition(storage, available)
			}
		}))
		clientRegistry, err := objstore.NewRegistry(ctx, credsSvc, metricsSvc, objstore.WithHealth(healthReg))
		if err != nil {
			return err
		}
		manageStorageHealth(ctx, credsSvc, clientRegistry, healthReg, func(name string) {
			if conf.Metrics.Enabled {
				metrics.SetStorageAvailable(name, true)
			}
		})
		storagesKnown = func() int { return len(credsSvc.Storages()) }
		limiter.SetLookup(credsSvc.RateLimit)
		startedAt := time.Now().UTC()
		rateWindow := trafficstats.NewWindow(trafficstats.DefaultWindow)
		rateWindow.Add(trafficstats.Default.Snapshot())
		go func() { // 1s samples keep the window fine-grained even with a long status.interval
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					rateWindow.Add(trafficstats.Default.Snapshot())
				}
			}
		}()
		statusPub = proxystatus.NewPublisher(confRedis, app.AppID, conf.Status.Interval, func() proxystatus.Status {
			st := proxyStatus(conf, group, cfgState, healthReg, startedAt, proxyAccounts(conf.Auth, credsSvc))
			rateWindow.Add(*st.Stats)
			rates := rateWindow.Rates()
			st.Rates = &rates
			return st
		}, logger)
		go statusPub.Run(ctx)
		go proxystatus.RunHistory(ctx, confRedis, app.AppID, rateWindow.Rates, logger)

		lockRedis := util.NewRedisRole(conf.Redis, config.RedisRoleLock)
		defer lockRedis.Close()

		failoverStore := failover.NewStore(confRedis, func(name string) bool { _, ok := credsSvc.Storages()[name]; return ok })
		failoverCtrl = failovercontroller.New(failovercontroller.Config{
			DefaultAfter:       initEff.Failover.After,
			DefaultMaxEventLag: initEff.Failover.MaxEventLag,
			CheckInterval:      initEff.Failover.CheckInterval,
			MainStorage:        conf.Storage.Main,
		}, policySvc, healthReg, failovercontroller.RedisLocker{Client: lockRedis}, failoverStore, logger,
			failovercontroller.WithEnabled(initEff.Failover.Enabled),
			failovercontroller.WithUsers(func() []string {
				var users []string
				for name := range credsSvc.Storages() {
					users = append(users, credsSvc.ListUsers(name)...)
				}
				return users
			}),
			failovercontroller.WithIsProtected(func(ctx context.Context, user, bucket string) bool {
				group, ok := policySvc.ProtectionScope(ctx, user, bucket)
				return ok && group != ""
			}),
			failovercontroller.WithOnFailover(func(from, to string) {
				if conf.Metrics.Enabled {
					metrics.CountFailover(from, to)
				}
			}))
		go failoverCtrl.Run(ctx)
		if initEff.Failover.Enabled {
			logger.Warn().Dur("check_interval", initEff.Failover.CheckInterval).Msg("automatic failover controller enabled; only storages with an enabled failover policy can fail over")
		}

		objectLocker := store.NewObjectLocker(lockRedis, syncLockOverlap)
		// the copy service only reports in-progress bytes through the worker metrics: reuse the S3 metrics service
		copySvc := copy.NewS3CopySvc(credsSvc, clientRegistry, metricsSvc)
		proxyConfig.Sync = router.SyncOptions{
			Health:          healthReg,
			Enabled:         initEff.Replication.Sync.Enabled,
			Syncer:          syncer.New(verSvc, objectLocker, copySvc, clientRegistry),
			WriteQuorum:     initEff.Replication.Sync.WriteQuorum,
			Timeout:         initEff.Replication.Sync.Timeout,
			OnQuorumFailure: initEff.Replication.Sync.OnQuorumFailure,
		}
		if initEff.Replication.Sync.Enabled {
			logger.Info().Int("write_quorum", initEff.Replication.Sync.WriteQuorum).Dur("timeout", initEff.Replication.Sync.Timeout).Msg("synchronous replication enabled")
		}
		routeSvc := router.NewS3Router(clientRegistry, verSvc, uploadSvc, limiter)
		replSvc := replication.NewS3(queueSvc, verSvc, policySvc)
		accessCache := accesspolicy.NewCache(accesspolicy.NewStore(confRedis), accesspolicy.DefaultTTL)
		defaultPolicy := ""
		if conf.Auth != nil {
			defaultPolicy = conf.Auth.DefaultPolicy
		}
		accessMiddleware := access.Middleware(accessCache.Lookup,
			access.WithDefaultPolicy(defaultPolicy),
			access.WithSharedAccount(func(ctx context.Context, user string) bool {
				// the policy middleware has set the routing policy: the storage the request goes to
				return credsSvc.CredentialMode(xctx.GetRoutingPolicy(ctx), user) == objstore.ModeConnection
			}))
		// Writer lease of this proxy: refreshed while it runs, so a crashed proxy cannot block group promotion forever.
		groupStore := protectiongroups.NewStore(confRedis)
		// unprotected writes then cost one lookup (the fence rebuilds a missing index itself)
		if err := groupStore.EnsureIndex(ctx); err != nil {
			logger.Warn().Err(err).Msg("unable to prepare the protection group scope index")
		}
		groupLease := groupStore.NewLease(app.AppID, false, conf.ProtectionGroups.WriterLease)
		// keep the lease (and its self-fencing) alive while the listener drains in-flight writes
		leaseCtx, leaseStop := context.WithCancel(context.WithoutCancel(ctx))
		defer leaseStop()
		go groupLease.Run(leaseCtx)
		// unprotected scopes are admitted from the policy snapshot's copy of the scope index (0 Redis commands)
		groupFence := groupStore.Middleware(groupLease, policySvc.ProtectionScope)
		proxyConfig.AccessMiddleware = func(next http.Handler) http.Handler { return groupFence(accessMiddleware(next)) }
		// runtime-managed accounts: secrets are sealed with the dynamic credentials master password.
		// Without it no dynamic account can be created or authenticate.
		accountCache := proxyaccount.NewCache(proxyaccount.NewStore(confRedis, func() (proxyaccount.Sealer, error) {
			return credsSvc, nil
		}), proxyaccount.DefaultTTL)
		authCheck := auth.Middleware(conf.Auth, credsSvc, conf.Address, auth.WithAccounts(accountCache.Lookup))
		authCheck.SetRequireSignedContentType(initEff.RequireSignedContentType)
		updateAuth = authCheck.SetRequireSignedContentType
		proxyConfig.Storages[dom.S3] = router.StorageProxy{
			Router:             routeSvc,
			Replicator:         replSvc,
			AuthMiddleware:     authCheck.Wrap,
			ReqParseMiddleware: router.S3Middleware(conf.Address),
		}
		logger.Info().Msg("s3 proxy configured")
	}
	if swiftConf := conf.Storage.SwiftStorages(); len(swiftConf) != 0 {
		routeSvc := router.NewSwiftRouter(swiftConf, limiter)
		replSvc := replication.NewSwift(queueSvc, policySvc)
		proxyConfig.Storages[dom.Swift] = router.StorageProxy{
			Router:             routeSvc,
			Replicator:         replSvc,
			AuthMiddleware:     nil, // no auth check for swift
			ReqParseMiddleware: router.SwiftMiddleware(),
		}
		logger.Info().Msg("swift proxy configured")
	}
	proxyMux, err := router.New(proxyConfig)
	if err != nil {
		return err
	}
	ready := &readiness{
		snapshot:     policySvc.SnapshotStatus,
		storages:     storagesKnown,
		maxStaleness: initEff.ReadyMaxStaleness,
	}
	if ready.maxStaleness <= 0 {
		ready.maxStaleness = DefaultReadyMaxStaleness
	}
	if conf.Snapshot.Disabled {
		// without the snapshot, Redis reachability is checked directly
		ready.snapshot = func() policy.SnapshotStatus { return directRedisStatus(ctx, confRedis) }
	}
	// 0 = no drain wait (programmatic configs such as the standalone binary and tests); the embedded
	// config.yaml sets drainDelay: 5s for the proxy binary
	var drainDelayNs atomic.Int64
	drainDelayNs.Store(int64(initEff.DrainDelay))

	reloadFleetConfig := func() {
		fc, err := fleetStore.Get(ctx)
		if err != nil {
			if ctx.Err() == nil {
				logger.Warn().Err(err).Msg("unable to reload proxy fleet config")
			}
			return
		}
		eff, pending, reasons := proxystatus.Resolve(startupSettings, group, fc)
		proxyMux.SetReplicationOptions(router.ReplicationOptions{
			OnEventError:     eff.Replication.OnEventError,
			EventRetryBudget: eff.Replication.EventRetryBudget,
		})
		proxyMux.SetSyncOptions(router.SyncOptions{
			Enabled:         eff.Replication.Sync.Enabled,
			WriteQuorum:     eff.Replication.Sync.WriteQuorum,
			Timeout:         eff.Replication.Sync.Timeout,
			OnQuorumFailure: eff.Replication.Sync.OnQuorumFailure,
		})
		if healthReg != nil {
			healthReg.SetConfig(health.Config{
				FailureThreshold: eff.Health.FailureThreshold,
				OpenDuration:     eff.Health.OpenDuration,
			})
		}
		if failoverCtrl != nil {
			failoverCtrl.SetConfig(failovercontroller.Config{
				DefaultAfter:       eff.Failover.After,
				DefaultMaxEventLag: eff.Failover.MaxEventLag,
				CheckInterval:      eff.Failover.CheckInterval,
				MainStorage:        conf.Storage.Main,
			}, eff.Failover.Enabled)
		}
		if updateAuth != nil {
			updateAuth(eff.RequireSignedContentType)
		}
		ready.setMaxStaleness(eff.ReadyMaxStaleness)
		drainDelayNs.Store(int64(eff.DrainDelay))

		changed := cfgState.update(eff, fc.Version, pending, reasons)
		if changed && statusPub != nil {
			if err := statusPub.PublishOnce(ctx); err != nil && ctx.Err() == nil {
				logger.Warn().Err(err).Msg("unable to publish updated proxy status after fleet config reload")
			}
		}
	}
	go watchFleetConfig(ctx, confRedis, reloadFleetConfig)

	// no WriteTimeout: large uploads and downloads stream for a long time
	readHeaderTimeout := conf.ReadHeaderTimeout
	if readHeaderTimeout <= 0 {
		readHeaderTimeout = DefaultReadHeaderTimeout
	}
	proxyServer := http.Server{
		Addr:              fmt.Sprintf(":%d", conf.Port),
		Handler:           ready.wrap(proxyMux),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       120 * time.Second,
	}

	server := util.NewServer()
	err = server.Add("proxy_http", func(_ context.Context) error {
		return proxyServer.ListenAndServe()
	}, func(shutdownCtx context.Context) error {
		// graceful drain: report not-ready first so the load balancer stops sending new requests,
		// then stop the listener and wait for in-flight requests
		ready.draining.Store(true)
		drainDelay := time.Duration(drainDelayNs.Load())
		logger.Info().Dur("drain_delay", drainDelay).Msg("shutdown: reporting not ready, draining")
		if drainDelay > 0 {
			select {
			case <-time.After(drainDelay):
			case <-shutdownCtx.Done():
			}
		}
		return proxyServer.Shutdown(shutdownCtx)
	})
	if err != nil {
		return err
	}
	logger.Info().Msg("proxy created")

	credentials := []map[string]s3.CredentialsV4{conf.Auth.Custom}
	if s3torageConf, ok := conf.Storage.S3Storages()[conf.Auth.UseStorage]; ok {
		credentials = append(credentials, s3torageConf.Credentials)
	}
	err = server.Add("proxy_request_reply", func(ctx context.Context) error {
		return rpc.ProxyServe(ctx, appRedis, requestReplyServer(conf.Address, credentials...))
	}, func(ctx context.Context) error { return nil })
	if err != nil {
		return err
	}

	if conf.Metrics.Enabled {
		start, stop := metrics.Server(ctx, conf.Metrics.Port, app)
		err = server.Add("proxy_metrics", start, stop)
		if err != nil {
			return err
		}
		logger.Info().Msg("metrics enabled")
	}

	return server.Start(ctx)
}

func requestReplyServer(proxyUrl string, credentials ...map[string]s3.CredentialsV4) rpc.ProxyGetCredentials {
	creds := make([]*pb.Credential, 0)
	for _, c := range credentials {
		for alias, cred := range c {
			creds = append(creds, &pb.Credential{
				Alias:     alias,
				AccessKey: cred.AccessKeyID,
				SecretKey: cred.SecretAccessKey,
			})
		}
	}
	slices.SortFunc(creds, func(a, b *pb.Credential) int {
		if n := strings.Compare(a.Alias, b.Alias); n != 0 {
			return n
		}
		return strings.Compare(a.AccessKey, b.AccessKey)
	})
	res := &pb.GetProxyCredentialsResponse{
		Address:     proxyUrl,
		Credentials: creds,
	}
	return func(ctx context.Context) (*pb.GetProxyCredentialsResponse, error) {
		return res, nil
	}
}

// syncLockOverlap matches the default worker lock.overlap: the proxy takes the same distributed
// destination-object lock as the worker when syncing synchronously.
const syncLockOverlap = 2 * time.Second

// storageProbe is a cheap authenticated call (ListBuckets) with one configured credential of the storage.
func storageProbe(clients objstore.Clients, creds objstore.CredsService, storage string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		users := creds.ListUsers(storage)
		if len(users) == 0 {
			return health.ErrSkipProbe
		}
		slices.Sort(users)
		c, err := clients.AsS3(ctx, storage, users[0])
		if err != nil {
			return err
		}
		_, err = c.S3().ListBuckets(ctx)
		return err
	}
}

type fleetConfigState struct {
	mu             sync.RWMutex
	settings       proxystatus.Settings
	version        int64
	pendingRestart bool
	reasons        []string
}

func (s *fleetConfigState) update(eff proxystatus.Settings, ver int64, pending bool, reasons []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.version != ver || s.pendingRestart != pending ||
		!slices.Equal(s.reasons, reasons) || !reflect.DeepEqual(s.settings, eff)
	s.settings = eff
	s.version = ver
	s.pendingRestart = pending
	s.reasons = slices.Clone(reasons)
	return changed
}

func (s *fleetConfigState) get() (proxystatus.Settings, int64, bool, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings, s.version, s.pendingRestart, slices.Clone(s.reasons)
}

func buildProxySettings(conf *Config) proxystatus.Settings {
	hc := conf.Health.WithDefaults()
	readHeaderTimeout := conf.ReadHeaderTimeout
	if readHeaderTimeout <= 0 {
		readHeaderTimeout = DefaultReadHeaderTimeout
	}
	writerLease := conf.ProtectionGroups.WriterLease
	if writerLease <= 0 {
		writerLease = DefaultWriterLease
	}
	readyMaxStaleness := conf.Ready.MaxStaleness
	if readyMaxStaleness <= 0 {
		readyMaxStaleness = DefaultReadyMaxStaleness
	}
	onEventError := conf.Replication.OnEventError
	if onEventError == "" {
		onEventError = router.OnEventErrorFail
	}
	eventRetryBudget := conf.Replication.EventRetryBudget
	if eventRetryBudget <= 0 {
		eventRetryBudget = router.DefaultEventRetryBudget
	}
	onQuorumFailure := conf.Replication.Sync.OnQuorumFailure
	if onQuorumFailure == "" {
		onQuorumFailure = router.OnQuorumFailureFail
	}
	syncTimeout := conf.Replication.Sync.Timeout
	if syncTimeout <= 0 {
		syncTimeout = 30 * time.Second
	}
	checkInterval := conf.Failover.CheckInterval
	if checkInterval <= 0 {
		checkInterval = 10 * time.Second
	}
	after := conf.Failover.After
	if after <= 0 {
		after = 60 * time.Second
	}
	settings := proxystatus.Settings{
		Replication: proxystatus.ReplicationSettings{
			OnEventError:     onEventError,
			EventRetryBudget: eventRetryBudget,
			Sync: proxystatus.SyncSettings{
				Enabled:         conf.Replication.Sync.Enabled,
				WriteQuorum:     conf.Replication.Sync.WriteQuorum,
				Timeout:         syncTimeout,
				OnQuorumFailure: onQuorumFailure,
			},
		},
		Health: proxystatus.HealthSettings{
			FailureThreshold: hc.FailureThreshold,
			OpenDuration:     hc.OpenDuration,
		},
		Failover: proxystatus.FailoverSettings{
			Enabled:       conf.Failover.Enabled,
			CheckInterval: checkInterval,
			After:         after,
			MaxEventLag:   conf.Failover.MaxEventLag,
		},
		ReadHeaderTimeout: readHeaderTimeout,
		WriterLease:       writerLease,
		DrainDelay:        conf.DrainDelay,
		ReadyMaxStaleness: readyMaxStaleness,
	}
	if conf.Auth != nil {
		settings.RequireSignedContentType = conf.Auth.RequireSignedContentType
	}
	return settings
}

func watchFleetConfig(ctx context.Context, rc redis.UniversalClient, reload func()) {
	sub := rc.Subscribe(ctx, proxystatus.ConfigChangedChannel)
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				ch = nil
				continue
			}
			reload()
		case <-ticker.C:
			reload()
		}
	}
}

// proxyStatus builds the status document published to Redis (config DB, tutti:proxy-status:<app_id>).
func proxyStatus(conf *Config, group string, cfgState *fleetConfigState, reg *health.Registry, startedAt time.Time, accounts []proxystatus.Account) proxystatus.Status {
	snap := reg.Snapshot()
	storages := make([]proxystatus.StorageHealth, 0, len(snap))
	for _, s := range snap {
		storages = append(storages, proxystatus.StorageHealth{
			Name: s.Name, Available: s.Available, ConsecutiveFailures: s.ConsecutiveFailures,
			LastError: s.LastError, LastChange: s.LastChange,
		})
	}
	effSettings, version, pending, reasons := cfgState.get()
	st := proxystatus.Status{
		Address:               conf.Address,
		Group:                 group,
		StartedAt:             startedAt,
		Storages:              storages,
		Accounts:              accounts,
		Settings:              effSettings,
		AppliedConfigVersion:  version,
		PendingRestart:        pending,
		PendingRestartReasons: reasons,
	}
	snapStats := trafficstats.Default.Snapshot()
	st.Stats = &snapStats
	return st
}

// proxyAccounts lists the accounts that can authenticate against the proxy: users of the auth.useStorage
// credentials and auth.custom. Only access key ids are listed, never secrets.
func proxyAccounts(conf *auth.Config, creds objstore.CredsService) []proxystatus.Account {
	if conf == nil {
		return nil
	}
	var res []proxystatus.Account
	if conf.UseStorage != "" {
		for _, user := range creds.ListUsers(conf.UseStorage) {
			c, err := creds.GetS3Credentials(conf.UseStorage, user)
			if err != nil {
				continue
			}
			res = append(res, proxystatus.Account{User: user, AccessKeyID: c.AccessKeyID, Source: "storage:" + conf.UseStorage})
		}
	}
	for user, c := range conf.Custom {
		res = append(res, proxystatus.Account{User: user, AccessKeyID: c.AccessKeyID, Source: "custom"})
	}
	slices.SortFunc(res, func(a, b proxystatus.Account) int {
		if c := strings.Compare(a.User, b.User); c != 0 {
			return c
		}
		return strings.Compare(a.AccessKeyID, b.AccessKeyID)
	})
	return res
}

// directRedisStatus is the readiness input when the policy snapshot is disabled: a Redis PING.
func directRedisStatus(ctx context.Context, rc redis.UniversalClient) policy.SnapshotStatus {
	pctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	ok := rc.Ping(pctx).Err() == nil
	st := policy.SnapshotStatus{Loaded: true, RedisReachable: ok}
	if ok {
		st.LastRefresh = time.Now()
	} else {
		st.Age = time.Hour // unknown: no cached state to serve from
	}
	return st
}
