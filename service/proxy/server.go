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
	"slices"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
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

	appRedis := util.NewRedis(conf.Redis, conf.Redis.MetaDB)
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

	confRedis := util.NewRedis(conf.Redis, conf.Redis.ConfigDB)
	defer confRedis.Close()
	err = redisotel.InstrumentTracing(confRedis, redisotel.WithTracerProvider(tp))
	if err != nil {
		return fmt.Errorf("%w: unable to instrument tracing app redis", err)
	}
	queueRedis := util.NewRedisAsynq(conf.Redis, conf.Redis.QueueDB)
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

	// configure object storage proxies:
	proxyConfig := router.Config{
		Storages:          map[dom.StorageType]router.StorageProxy{},
		LogMiddleware:     log.HttpMiddleware(conf.Log, app.App, app.AppID, xctx.Event),
		TraceMiddleware:   nil,
		MetricsMiddleware: nil,
		PolicyMiddleware:  router.PolicyMiddleware(policySvc),
		Replication: router.ReplicationOptions{
			OnEventError:     conf.Replication.OnEventError,
			EventRetryBudget: conf.Replication.EventRetryBudget,
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
		healthReg := health.NewRegistry(conf.Health, health.WithLogger(logger), health.WithOnChange(func(storage string, available bool) {
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
		statusPub := proxystatus.NewPublisher(confRedis, app.AppID, conf.Status.Interval, func() proxystatus.Status {
			st := proxyStatus(conf, healthReg, startedAt, proxyAccounts(conf.Auth, credsSvc))
			rateWindow.Add(*st.Stats)
			rates := rateWindow.Rates()
			st.Rates = &rates
			return st
		}, logger)
		go statusPub.Run(ctx)
		go proxystatus.RunHistory(ctx, confRedis, app.AppID, rateWindow.Rates, logger)
		if conf.Failover.Enabled {
			failoverLock := util.NewRedis(conf.Redis, conf.Redis.LockDB)
			defer failoverLock.Close()
			failoverStore := failover.NewStore(confRedis, func(name string) bool { _, ok := credsSvc.Storages()[name]; return ok })
			ctrl := failovercontroller.New(failovercontroller.Config{
				DefaultAfter:       conf.Failover.After,
				DefaultMaxEventLag: conf.Failover.MaxEventLag,
				CheckInterval:      conf.Failover.CheckInterval,
				MainStorage:        conf.Storage.Main,
			}, policySvc, healthReg, failovercontroller.RedisLocker{Client: failoverLock}, failoverStore, logger,
				failovercontroller.WithUsers(func() []string {
					var users []string
					for name := range credsSvc.Storages() {
						users = append(users, credsSvc.ListUsers(name)...)
					}
					return users
				}),
				failovercontroller.WithOnFailover(func(from, to string) {
					if conf.Metrics.Enabled {
						metrics.CountFailover(from, to)
					}
				}))
			go ctrl.Run(ctx)
			logger.Warn().Dur("check_interval", conf.Failover.CheckInterval).Msg("automatic failover controller enabled; only storages with an enabled failover policy can fail over")
		}
		if conf.Replication.Sync.Enabled {
			lockRedis := util.NewRedis(conf.Redis, conf.Redis.LockDB)
			defer lockRedis.Close()
			objectLocker := store.NewObjectLocker(lockRedis, syncLockOverlap)
			// the copy service only reports in-progress bytes through the worker metrics: reuse the S3 metrics service
			copySvc := copy.NewS3CopySvc(credsSvc, clientRegistry, metricsSvc)
			proxyConfig.Sync = router.SyncOptions{
				Health:          healthReg,
				Enabled:         true,
				Syncer:          syncer.New(verSvc, objectLocker, copySvc, clientRegistry),
				WriteQuorum:     conf.Replication.Sync.WriteQuorum,
				Timeout:         conf.Replication.Sync.Timeout,
				OnQuorumFailure: conf.Replication.Sync.OnQuorumFailure,
			}
			logger.Info().Int("write_quorum", conf.Replication.Sync.WriteQuorum).Dur("timeout", conf.Replication.Sync.Timeout).Msg("synchronous replication enabled")
		}
		routeSvc := router.NewS3Router(clientRegistry, verSvc, uploadSvc, limiter)
		replSvc := replication.NewS3(queueSvc, verSvc, policySvc)
		accessCache := accesspolicy.NewCache(accesspolicy.NewStore(confRedis), accesspolicy.DefaultTTL)
		proxyConfig.AccessMiddleware = access.Middleware(accessCache.Lookup)
		// runtime-managed accounts: secrets are sealed with the dynamic credentials master password.
		// Without it no dynamic account can be created or authenticate.
		accountCache := proxyaccount.NewCache(proxyaccount.NewStore(confRedis, func() (proxyaccount.Sealer, error) {
			return credsSvc, nil
		}), proxyaccount.DefaultTTL)
		authCheck := auth.Middleware(conf.Auth, credsSvc, conf.Address, auth.WithAccounts(accountCache.Lookup))
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
	proxyServer := http.Server{Addr: fmt.Sprintf(":%d", conf.Port), Handler: proxyMux}

	server := util.NewServer()
	err = server.Add("proxy_http", func(_ context.Context) error {
		return proxyServer.ListenAndServe()
	}, proxyServer.Shutdown)
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

// proxyStatus builds the status document published to Redis (config DB, tutti:proxy-status:<app_id>).
func proxyStatus(conf *Config, reg *health.Registry, startedAt time.Time, accounts []proxystatus.Account) proxystatus.Status {
	snap := reg.Snapshot()
	storages := make([]proxystatus.StorageHealth, 0, len(snap))
	for _, s := range snap {
		storages = append(storages, proxystatus.StorageHealth{
			Name: s.Name, Available: s.Available, ConsecutiveFailures: s.ConsecutiveFailures,
			LastError: s.LastError, LastChange: s.LastChange,
		})
	}
	hc := conf.Health.WithDefaults()
	st := proxystatus.Status{
		Address:   conf.Address,
		StartedAt: startedAt,
		Storages:  storages,
		Accounts:  accounts,
		Settings: proxystatus.Settings{
			Replication: proxystatus.ReplicationSettings{
				OnEventError:     conf.Replication.OnEventError,
				EventRetryBudget: conf.Replication.EventRetryBudget,
				Sync: proxystatus.SyncSettings{
					Enabled:         conf.Replication.Sync.Enabled,
					WriteQuorum:     conf.Replication.Sync.WriteQuorum,
					Timeout:         conf.Replication.Sync.Timeout,
					OnQuorumFailure: conf.Replication.Sync.OnQuorumFailure,
				},
			},
			Health: proxystatus.HealthSettings{FailureThreshold: hc.FailureThreshold, OpenDuration: hc.OpenDuration},
			Failover: proxystatus.FailoverSettings{
				Enabled: conf.Failover.Enabled, CheckInterval: conf.Failover.CheckInterval,
				After: conf.Failover.After, MaxEventLag: conf.Failover.MaxEventLag,
			},
		},
	}
	snapStats := trafficstats.Default.Snapshot()
	st.Stats = &snapStats
	if conf.Auth != nil {
		st.Settings.RequireSignedContentType = conf.Auth.RequireSignedContentType
	}
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
