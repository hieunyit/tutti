# Tutti configuration reference

Tutti adds the keys below to the Tutti proxy config ([service/proxy/config.yaml](../../service/proxy/config.yaml), embedded as the default config and overridden by your file). Everything else in the proxy and worker config is unchanged from Tutti.

Source: `service/proxy/config.go`, `service/proxy/auth/config.go`, `pkg/health/breaker.go`, `pkg/s3/config.go`.

Defaults below are the effective values: the embedded `config.yaml` is always loaded first, so omitting a key gives the default listed here.

## Summary

| Key | Type | Default | Section |
|---|---|---|---|
| `auth.requireSignedContentType` | bool | `false` | [Auth](#auth) |
| `auth.defaultPolicy` | `allow-legacy` \| `deny` | `allow-legacy` | [Auth](#auth) |
| `replication.onEventError` | `fail` \| `log` | `fail` | [Event recording](#replication-event-recording) |
| `replication.eventRetryBudget` | duration | `2s` | [Event recording](#replication-event-recording) |
| `replication.sync.enabled` | bool | `false` | [Synchronous replication](#synchronous-replication) |
| `replication.sync.writeQuorum` | int | `0` (all) | [Synchronous replication](#synchronous-replication) |
| `replication.sync.timeout` | duration | `30s` | [Synchronous replication](#synchronous-replication) |
| `replication.sync.onQuorumFailure` | `fail` \| `accept` | `fail` | [Synchronous replication](#synchronous-replication) |
| `health.failureThreshold` | int | `3` | [Circuit breaker](#circuit-breaker-and-health-checks) |
| `health.openDuration` | duration | `30s` | [Circuit breaker](#circuit-breaker-and-health-checks) |
| `storage.storages.<name>.healthCheckInterval` | duration | `10s` | [Circuit breaker](#circuit-breaker-and-health-checks) |
| `status.interval` | duration | `2s` | [Status](#status) |
| `ready.maxStaleness` | duration | `30s` | [Readiness and drain](#readiness-and-graceful-drain) |
| `drainDelay` | duration | `5s` | [Readiness and drain](#readiness-and-graceful-drain) |
| `snapshot.disabled` | bool | `false` | [Policy snapshot](#policy-snapshot) |
| `snapshot.pollInterval` | duration | `1s` | [Policy snapshot](#policy-snapshot) |
| `snapshot.fullReload` | duration | `30s` | [Policy snapshot](#policy-snapshot) |
| `protectionGroups.writerLease` | duration | `30s` | Writer lease of the protection group write fence (0 = default, else ≥ 1s); see [Protection groups](./protection-groups.md#writer-leases-and-crashed-proxies) |
| `redis.addresses`, `redis.sentinel.*` | | none | [Redis](#redis) |
| `redis.config` / `queue` / `lock` / `meta` | object | inherit | [Redis](#redis) |
| `failover.enabled` | bool | `false` | [Automatic failover](#automatic-failover) |
| `failover.checkInterval` | duration | `10s` | [Automatic failover](#automatic-failover) |
| `failover.after` | duration | `60s` | [Automatic failover](#automatic-failover) |
| `failover.maxEventLag` | int | `0` | [Automatic failover](#automatic-failover) |

Durations use Go syntax (`500ms`, `30s`, `2m`). Every key is validated at proxy startup (`Config.Validate`); an invalid value stops the proxy with an error naming the key.

## Auth

```yaml
auth:
  requireSignedContentType: false
  defaultPolicy: allow-legacy
```

- `defaultPolicy` (`allow-legacy` | `deny`, default `allow-legacy`): what a user **without an access policy** may do. `allow-legacy`: a user whose storage credentials are its own (override or config `credentials.<user>`) has full access as before, while a user that resolves through a shared connection account on the storage it is routed to is denied (`403 AccessDenied`, empty `ListBuckets`): see [isolation on a shared account](./storage-accounts.md#isolation-on-a-shared-account-default-deny). `deny`: every user without a policy is denied. A user with a policy is never affected by this setting. Any other value stops the proxy at startup.

- `requireSignedContentType` (bool, default `false`): when `true`, a SigV4 request carrying a `Content-Type` header is rejected unless `content-type` is listed in `SignedHeaders`. When `false`, `Content-Type` need not be signed (AWS only requires `host` and `x-amz-*` headers to be signed; `mc` and `minio-go` do not sign it).
- `host` and every `x-amz-*` header present in the request must be signed regardless of this setting.
- Not present in the shipped `config.yaml`; set it only if you want the strict behavior.

Source: `service/proxy/auth/config.go`, `pkg/s3/auth.go` (`validateSignedHeaders`).

## Managed console users (worker)

The first admin password (`TUTTI_BOOTSTRAP_ADMIN_PASSWORD`, `api.auth.bootstrapAdminPassword`, or generated), its minimum length and the `worker admin reset-password` recovery command are described in [First login and password recovery](operations.md#first-login-and-password-recovery).

Console users sign in to the worker management API and the UI. They come from two sources:

- `api.auth.users` in the worker config (bcrypt `passwordHash`, role `admin` or `viewer`): read-only, always win on name conflicts.
- Managed users stored in Redis hash `tutti:console-users` in the **config DB** (`redis.configDB`), created and changed through the console users API ([api.md](api.md#console-users-api)). Nothing extra needs to be configured: they are available whenever `api.auth.enabled: true`.

```yaml
api:
  auth:
    enabled: true
    sessionIdleTimeout: 30m   # session ends after this long without POST /auth/refresh (default 30m)
    sessionMaxLifetime: 24h   # absolute cap from login; refresh never extends past it (default 24h)
    users:            # at least one is required; keep an admin here so the system is always administrable
      - username: admin
        passwordHash: "$2b$12$..."
        role: admin
```

`sessionTTL` is deprecated: when `sessionMaxLifetime` is unset, `sessionTTL` is used as the maximum lifetime (the idle timeout stays 30m, or the `sessionTTL` if that is shorter). `sessionMaxLifetime` must not be shorter than `sessionIdleTimeout`. Only the console's `POST /auth/refresh` extends a session (the UI calls it on real user activity); ordinary API calls and polling do not. Sessions created before an upgrade keep working until their existing expiry.

Passwords of managed users must be 12-72 bytes and are stored as bcrypt (cost 12). Sessions live in the meta DB (`redis.metaDB`). With `api.auth.enabled: false` the console users API returns failed precondition (`sign-in is disabled`).

### Login protection (worker)

Failed sign-ins are counted per (username, client IP) and per username across all addresses (Redis, 15 minute window):

- **Per (username, IP)**: 10 failures block that pair for the rest of the window (`429`), even with the right password. A successful login clears this counter. `remainingAttempts` in the `401` response is the remaining budget of the pair.
- **Per username, all addresses**: after 100 failures, addresses that have **not** logged in successfully as that user in the last 30 days are delayed progressively (1 s, doubling every 10 further failures, capped at 30 s; the delay never blocks other users or other requests), and beyond 1000 failures in the window they are refused (`429`). Addresses that logged in successfully as that user in the last 30 days (remembered per (username, IP) in Redis) are **not** affected, so an attacker spraying passwords from many addresses cannot lock a known administrator out; the per-(username, IP) budget still applies to them. A successful login never resets the username-wide counter. `remainingAttempts` reports the username-wide ceiling for unknown addresses once it is the tighter bound.
- Behind a reverse proxy set `api.auth.trustedProxies`; the client IP is then taken from `X-Forwarded-For` / `X-Real-IP` of trusted peers only (unchanged).

Source: `pkg/auth/service.go` (`throttled`, `userDelay`).

### Webhook token (worker)

```yaml
api:
  webhook:
    enabled: true
    baseUrl: https://tutti-api.example.com
    token: "long-random-secret"   # optional but strongly recommended
```

The webhook endpoints (`/tutti.Webhook/*`, HTTP `/webhook/{storage}/s3-notifications` and `/webhook/{storage}/swift`) cannot use console sign-in, so they are public unless `api.webhook.token` is set. When set, every webhook call on both the main API ports and the separate `api.webhook.grpcPort`/`httpPort` must carry the token as `Authorization: Bearer <token>` or `X-Tutti-Webhook-Token: <token>` (gRPC metadata for gRPC calls); it is compared in constant time and a missing or wrong token gets `401` / `Unauthenticated`. Rotate it by changing the config and restarting the workers, then updating the notification targets.

When webhooks are enabled on the main API port (no `grpcPort`) and no token is set, the worker logs a warning at startup and `GET /auth/me` returns `"warnings": ["webhook_unauthenticated"]` so the UI can show a banner. Use the token, or run the webhooks on separate ports that only the storage systems can reach.

Configuring the storage side:

- **MinIO**: set the token as the webhook target `auth_token` (MinIO sends it as `Authorization: Bearer <token>`):
  `mc admin config set myminio notify_webhook:tutti endpoint="https://tutti-api.example.com/webhook/<storage>/s3-notifications" auth_token="long-random-secret"` then `mc admin service restart myminio`.
- **Ceph RGW** (topic with an HTTP endpoint): the endpoint URL cannot carry headers; use a reverse proxy in front of the webhook port that adds `Authorization: Bearer <token>` or `X-Tutti-Webhook-Token`, or restrict the webhook port by network.
- The `token` is read-only config; do not put it in a URL.

Source: `pkg/auth/webhook_token.go`, `pkg/api/config.go` (`WebhookConfig`).

## Replication event recording

```yaml
replication:
  onEventError: fail
  eventRetryBudget: 2s
```

After a write succeeded on the routed storage, the proxy records a replication event (a task in the Redis work queue) for each replication task.

- `onEventError`
  - `fail` (default): if the event cannot be recorded within the retry budget the proxy answers `503 ServiceUnavailable` (S3 error code `ServiceUnavailable`). The data is already on the routed storage; the client should retry the request (it is idempotent) which records the event again.
  - `log`: upstream Chorus behavior. The error is logged and the client gets the storage response; the replica can silently miss the change.
  - Any other value is rejected: `invalid replication.onEventError`.
- `eventRetryBudget` (duration, default `2s`): total time spent retrying the recording, with exponential backoff starting at 50 ms. The budget is shared by all tasks of one request (for example a `DeleteObjects` with many keys). `0` falls back to `2s` in the router; negative values are rejected. Retrying stops early if the client disconnects.

Source: `service/proxy/router/server.go` (`replicateWithRetry`, `ReplicationOptions`).

## Synchronous replication

```yaml
replication:
  sync:
    enabled: true
    writeQuorum: 2        # main + at least one destination
    timeout: 30s
    onQuorumFailure: fail # or accept
```

When enabled, after the main write succeeded and its event was recorded, the proxy starts copying the object to the replication destinations and answers when the write quorum is confirmed. The queued replication task stays as a safety net: destinations that failed, timed out, or were still pending are completed by the worker.

- `enabled` (bool, default `false`).
- `writeQuorum` (int, default `0`): number of copies, **main included**, that must exist before answering. `0` means main plus all destinations. A value larger than `1 + destinations` is capped to `1 + destinations`. Negative values are rejected.
- `timeout` (duration, default `30s`): maximum time spent waiting for the write quorum per request (destinations run in parallel). Must be greater than `0` when `enabled` is `true`, otherwise startup fails.
- `onQuorumFailure` (`fail` | `accept`, default `fail`): what to do when fewer than `writeQuorum` copies exist. `fail` answers `503 ServiceUnavailable`; `accept` forwards the main storage response. Any other value is rejected. An empty string behaves as `fail`.

Interactions:

- Sync only does something for requests that have replication destinations, that is, an active replication policy (user-level or bucket-level) from the routed storage. Without replication policies nothing is synced and no header is added.
- Only object content/delete tasks are synced (see [operations](./operations.md#synchronous-replication)). ACL, tagging and bucket tasks stay asynchronous.
- Sync is skipped for a request whose event recording failed with `onEventError: log`.
- A destination whose circuit breaker is open is not attempted; if that makes the quorum unreachable the proxy does not wait for any destination.
- The proxy answers as soon as the configured quorum is confirmed and cancels unfinished synchronous attempts. The already queued replication tasks let the worker finish those destinations. With `writeQuorum: 1`, no destination is attempted on the request path.
- The proxy needs Redis lock access (`redis.lockDB`) for the destination-object lock, the same one the worker uses.
- The response carries `X-Tutti-Replicas: <copies>/<1+destinations>`.

Source: `service/proxy/config.go` (`SyncConfig`), `service/proxy/router/sync_replication.go`, `service/proxy/server.go`.

## Storage connection account

```yaml
storage:
  storages:
    main:
      type: S3
      address: "minio.example.internal:9000"
      provider: Minio
      defaultRegion: us-east-1
      connectionAccount:           # optional: the dedicated account Tutti uses on this storage
        accessKeyID: tutti-svc
        secretAccessKey: "<secret>"
      credentials:                 # optional per-user overrides (win over the connection account)
        alice:
          accessKeyID: ALICEKEY
          secretAccessKey: "<secret>"
```

- `storage.storages.<name>.connectionAccount.accessKeyID` / `secretAccessKey` (both required if the block is present): the account the proxy, the worker (replication, diff, migration, switch), failover and the health checks use for every user without an override on this storage. Resolution order: `credentials.<user>` or dynamic credentials of the user, then `connectionAccount`, otherwise `storage S has no credentials for user U` (proxy: 503).
- Config-defined accounts are read-only in the API (`connectionAccount.source: "config"`, rotation through the API is refused) and are stored in plain text in the file: restrict it. Storages created through the API store the account sealed with `storage.dynamicCredentials.masterPassword`.
- Set it on the worker **and** the proxies (both read `storage.storages`). Prepare the account and its policy first: [Storage accounts](./storage-accounts.md).

Source: `pkg/s3/config.go`, `pkg/objstore/connection.go`.

## Circuit breaker and health checks

```yaml
health:
  failureThreshold: 3
  openDuration: 30s

storage:
  storages:
    main:
      healthCheckInterval: 10s
```

- `health.failureThreshold` (int, default `3`): consecutive availability failures that open a storage's breaker. `0` means the default; negative is rejected.
- `health.openDuration` (duration, default `30s`): how long an open breaker rejects requests before one trial request is allowed. `0` means the default; negative is rejected.
- `storage.storages.<name>.healthCheckInterval` (duration, default `10s` when unset or `0`): period of the active probe for that S3 storage. Negative is rejected (`healthCheckInterval must not be negative`). The probe timeout is `min(3s, interval/2)`.

What counts as a failure: network errors, timeouts and HTTP 5xx (except 501). Any other definitive answer (2xx, 3xx, 4xx, including `AccessDenied`/`NoSuchKey`) counts as success. Client cancellations are ignored.

Interactions:

- `healthCheckInterval` times `failureThreshold` is the typical time to detect a dead storage (for example 3 x 10s = about 30 s). Real requests that fail also count, so a busy proxy can detect it earlier. See [operations](./operations.md#circuit-breaker-and-health-checks).
- Probes run only for S3 storages. With a connection account the probe runs steps 1 to 5 of the [connection test](./storage-accounts.md#test-connection) (DNS/TCP, TLS, S3 endpoint, clock skew, authentication) and reports a problem as `lastError` with the step name (`authentication: SignatureDoesNotMatch`); host, TLS and endpoint failures count as outages, authentication and clock problems are only reported. Without a connection account the probe is a `ListBuckets` call with the alphabetically first per-user credential of the storage; a storage without any credential is not probed. Swift storages are not probed.
- The same `health` settings apply to all storages in one proxy; the interval is per storage.
- Metrics (when `metrics.enabled`): `storage_available`, `storage_breaker_transitions_total`.

Source: `pkg/health/`, `pkg/storagetest/probe.go`, `pkg/s3/config.go`, `service/proxy/managed_health.go`.

## Redis

Shared by proxy and worker (`redis:` block, common config, env prefix `CFG_`). Default: one standalone Redis, four logical DBs.

```yaml
redis:
  addresses: [redis:6379]   # standalone: one address. Sentinel: the sentinel addresses. (address: is the deprecated single form)
  user: ""
  password: ""
  tls: {enabled: false, insecure: false}
  sentinel:                 # set masterName to use Redis Sentinel
    masterName: ""
    user: ""
    password: ""
  metaDB: 0
  queueDB: 1
  lockDB: 2
  configDB: 3
  # optional per-role instances (each may use db: 0), unset fields inherit from above
  config: {addresses: [], user: "", password: "", tls: "", sentinel: {masterName: "", user: "", password: ""}, db: }
  queue:  {}   # same keys as config
  lock:   {}
  meta:   {}
```

| Key | Meaning |
|---|---|
| `redis.sentinel.masterName` | Enables Sentinel (go-redis failover client, and asynq `RedisFailoverClientOpt` for the queue). `addresses` are then the sentinels. Env: `CFG_REDIS_SENTINEL_MASTERNAME`, `..._USER`, `..._PASSWORD`. |
| `redis.<role>.addresses` | The role (`config`, `queue`, `lock`, `meta`) uses its own Redis, or its own Sentinel group when `redis.<role>.sentinel.masterName` is set. Without it the role uses the top-level instance and its `*DB` number. |
| `redis.<role>.user`, `.password` | Credentials of the role's instance. Empty inherits the top-level value. |
| `redis.<role>.tls` | `""` inherits `redis.tls.enabled`, `true`/`false` overrides it (`insecure` is inherited). |
| `redis.<role>.db` | Logical DB on the role's instance. Unset: the role's top-level DB key. Use `0` on a dedicated instance. YAML only. |

Roles: `config` = routing, replication, failover policies, storages, users, keys (`configDB`); `queue` = asynq replication queues (`queueDB`); `lock` = distributed locks (`lockDB`); `meta` = version vectors, uploads, sessions, migration state (`metaDB`). Existing configs without the role blocks behave exactly as before. A role with its own `addresses` does not inherit the top-level `sentinel` block. Use the same topology on proxy and worker.

Not supported: **Redis Cluster** (it only has DB 0, and Tutti's Lua scripts and transactions touch several keys). More than one address without `sentinel.masterName` selects a cluster client and is not a supported configuration. Operating guidance, an example Sentinel topology and the per-role layout: [redis-operations.md](./redis-operations.md#high-availability-with-redis-sentinel).

## Policy snapshot

```yaml
snapshot:
  disabled: false
  pollInterval: 1s
  fullReload: 30s
```

The proxy keeps the routing, routing blocks, zero-downtime switch, replication policy and replication generation data of each (user, bucket) in memory instead of reading it from Redis (about 12 commands in one MULTI/EXEC) on every request. Writers (worker API, failover controller, policy service) bump `tutti:cfg:version` and publish on `tutti:cfg:changed` in the config Redis in the same transaction as the change. A proxy drops its cache when it sees a new version (notification, or the `pollInterval` poll). `fullReload` also drops the cache periodically, as a safety net for edits made outside Tutti. `disabled: true` restores the previous behavior (every request reads Redis). GET/HEAD/LIST are served without any Redis command and keep working from the last known data while Redis is unreachable; every other request first reads the version key (1 command) and returns `503 ServiceUnavailable` if Redis is unreachable. Details: [performance-and-redis.md](./performance-and-redis.md#redis-commands-per-request-after-the-policy-snapshot). Prometheus: `policy_snapshot_reloads_total`, `_hits_total`, `_misses_total`, `_stale_served_total`, `_refresh_errors_total`, `policy_snapshot_age_seconds`.

## Readiness and graceful drain

```yaml
ready:
  maxStaleness: 30s
drainDelay: 5s
```

`GET /ready` (also `HEAD`) on the proxy port needs no authentication, is not counted in traffic statistics and is not logged. It answers `200` only when all hold, otherwise `503`, with a JSON body listing `reasons`:

- the policy snapshot is loaded,
- Redis is reachable, or the last successful contact is younger than `ready.maxStaleness` (mode `read-only`: the proxy still serves reads, writes would answer 503),
- at least one storage is known,
- the proxy is not draining.

```json
{"ready":true,"mode":"ok","snapshotLoaded":true,"snapshotVersion":"12","snapshotAgeSeconds":0.4,"redisReachable":true,"storages":2,"draining":false,"maxStalenessSeconds":30}
```

`/ready` is a reserved path: only an unauthenticated request without query string is answered by it, so authenticated and presigned requests to a bucket that is named `ready` still reach the S3 chain. On SIGTERM/SIGINT the proxy first reports not ready (`draining: true`), waits `drainDelay` (default `5s`; set it to at least the load balancer's check interval times its unhealthy threshold), then stops the listener and waits for in-flight requests. `drainDelay: 0` skips the wait. The all-in-one `tutti` standalone binary builds its proxy config in code and uses no drain wait and the default snapshot settings. The worker has no plain HTTP server on its own port besides optional metrics, so it has no `/ready`.

## Status

```yaml
status:
  interval: 2s
```

`status.interval` (duration, default `2s`, must be greater than `0`) is how often the proxy publishes its status document (`tutti:proxy-status:<app_id>`: storage health, settings, cumulative stats and current rates) to Redis. The key TTL is 3x the interval, so a stopped proxy disappears after at most 3 intervals. Cost: one Redis `SET` of a few KB per interval per proxy (0.5 command/s at the default). Independent of it, every proxy samples its counters every second for the 10 s rate window and appends a history sample every 10 s (see [Resilience API](./api.md#getstatshistory)).

## Automatic failover

```yaml
failover:
  enabled: true
  checkInterval: 10s
  after: 60s
  maxEventLag: 0
```

`failover` is only the global switch and the defaults. What may fail over to where is defined by **per-source-storage failover policies** stored in Redis and managed with the [Resilience API](./api.md) (REST, gRPC, `tuttictl`, or the UI `/failover` page).

- `enabled` (bool, default `false`): starts the failover controller in this proxy.
- `checkInterval` (duration, default `10s`): how often conditions are evaluated.
- `after` (duration, default `60s`): default for policies that omit `after`; how long the source storage's breaker must have been open.
- `maxEventLag` (int, default `0`): default for policies that omit `maxEventLag`; maximum unprocessed replication events allowed on the replication to the target.
- Validation: `maxEventLag`, `after`, `checkInterval` must not be negative; when `enabled`, `after` and `checkInterval` must be greater than `0`.

Policy fields (stored in Redis hash `tutti:failover-policies`, one entry per source storage):

| Field | Meaning |
|---|---|
| `source` | Storage that may fail. Required, must be a configured storage. |
| `enabled` | Must be `true` for automatic failover. An enabled policy needs at least one target. |
| `targets` | Ordered list of storages that may be promoted. Must be configured, not equal to `source`, no duplicates, no empty names. |
| `after` | Duration; `0`/unset uses `failover.after`. Must not be negative. |
| `maxEventLag` | Integer or unset (unset uses `failover.maxEventLag`). Must not be negative. |

Interactions:

- Automatic failover requires **both** `failover.enabled: true` in the proxy and an enabled policy with targets for the failed storage. A storage without an enabled policy never fails over; policies stored while `failover.enabled` is false are inactive.
- A target only qualifies if a replication from the source to it exists (see [operations](./operations.md#automatic-failover)). Failover therefore also requires replication policies.
- Failover depends on the source's breaker, so `health.*` and `healthCheckInterval` determine when the clock starts. Rough time to failover: detection time + `after` + up to `checkInterval`.
- Set `failover.enabled` identically on all proxy instances you want to act; only one acts at a time (Redis lock).
- Policy validation (known storages) is done by the worker that serves the API, using the worker's storage config.

Source: `service/proxy/config.go` (`FailoverConfig`), `pkg/failover/store.go`, `service/proxy/failover/failover.go`.

## Fleet configuration (`group`, `tutti:proxy-config`)

Each proxy has an optional top-level `group` (default `default`). Behaviour settings are layered as
YAML baseline, then the cluster `defaults`, then the override of the proxy's group, stored in Redis under
`tutti:proxy-config` and edited on the Console **Proxies** page (`Resilience.GetProxyFleetConfig` /
`SetProxyFleetConfig`).

Central settings: `replication.onEventError`, `replication.eventRetryBudget`, `replication.sync.*`, `health.*`,
`failover.*`, `readHeaderTimeout`, `writerLease`, `drainDelay`, `readyMaxStaleness` and the auth option
`requireSignedContentType` (the only central auth option; the rest of `auth` stays in YAML).

- Updates carry `expected_version`; a stale version is rejected with `FailedPrecondition` (reload and retry).
- Every change is audited (actor, before, after) in a structured log line (`audit=proxy_fleet_config`) and the
  capped Redis list `tutti:proxy-config:audit`. The fleet config is stored in Redis in every control-plane mode, so
  it has no PostgreSQL audit row.
- Validation uses the YAML validators. Lower bound `100ms` for `health.openDuration`, `failover.checkInterval`
  and `readyMaxStaleness`; `sync.enabled` needs `timeout > 0`; `sync.writeQuorum` must not exceed the number of
  configured storages (checked by each proxy against its own storages).
- A proxy that receives an invalid document keeps the last good configuration and reports `configError` in its status.
- Sync is reported as enabled only on proxies that can actually synchronize (S3 proxies).
- `readHeaderTimeout` and `writerLease` need a restart (`pendingRestart`).


## Copy engines, resumable multipart and part sizes

Objects larger than one part (default 16 MiB) are uploaded as multipart. Two engines exist, selected by `worker.copyEngine` (System setting, applies to the next copy) or `copyEngine` in the worker YAML (`""`/`legacy`/`pipeline`):

- **`legacy` (default)**: minio-go `PutObject`. With `multipartThreads` 1 it reads one part from the source into memory, uploads it, then reads the next (the source stalls during each upload); with `multipartThreads` > 1 each part is a separate ranged GET streamed straight into the upload (no buffering, several source connections).
- **`pipeline`**: one sequential source GET (continued with a ranged GET at the current offset if the connection breaks), cut into parts that are read into pooled buffers and uploaded by K uploaders, so reading part N+1 overlaps uploading part N. K = `multipartThreads` when set explicitly, otherwise 2. Peak memory per object is `(K+1) x part size` (reserved from the buffer budget up front, buffers come from one process-wide size-class pool and go back as soon as a part is done). Objects of at most one part are a single PUT in both engines.

Part size for both engines: the configured part size (default 16 MiB), raised to `ceil(size / 9000)` rounded up to a MiB so the 10000-part limit is never reached (5 MiB parts of a 100 GB object would otherwise fail).

### Why the default is still `legacy`

Measured with the in-process benchmark (`go test ./service/worker/copy -run '^$' -bench Copy -benchtime 1x`, synthetic S3 servers in child processes, GOMAXPROCS=2, no TLS, loopback; 256 MiB objects; "MB/s" is the total over all objects; peak heap is per object). Loopback numbers are not WAN numbers.

| scenario (per-request latency 30 ms) | engine | 1 object MB/s | 8 objects MB/s | CPU s/GiB (1 / 8 obj) | peak heap MiB per object |
|---|---|---|---|---|---|
| 50 MiB/s per stream on both sides | legacy t1 | 43.7 | 348 | 1.09 / 0.64 | 22 |
| | legacy t2 (ranged GETs) | 84.4 | 674 | 1.84 / 0.66 | 6.5 |
| | pipeline t2 | 47.7 | 381 | 1.55 / 0.79 | 70 |
| | pipeline t4 | 47.7 | 381 | 1.49 / 0.74 | 70 |
| source 100, destination 25 MiB/s per stream | legacy t1 / t2 | 24 / 49 | 193 / 390 | 1.37 / 0.65 | 22 / 6.5 |
| | pipeline t2 / t4 | 46 / 77 | 366 / 615 | 1.14 / 0.75 | 70 / 102 |
| source 25, destination 100 MiB/s per stream | legacy t1 / t2 | 25 / 47 | 203 / 374 | 1.87 / 0.79 | 22 / 6.5 |
| | pipeline t2 / t4 | 25 | 203 | 2.1 / 0.88 | 54-70 |
| 50 MiB/s TOTAL per side (shared pipe) | all engines | 43-51 | 48-52 | 1.1-2.6 | - |
| unlimited loopback, 512 MiB | legacy t1 | 1720 | 2717 | 0.65 / 0.84 | 40 |
| | legacy t2 | 3307 | 2037 | 0.65 / 1.12 | 8.5 |
| | pipeline t2 | 2884 | 2292 | 0.71 / 1.08 | 74 |

What this says, honestly:

- The pipeline engine is better than `legacy` with one thread when the source and destination speeds are similar and the loss is the stall of the serial read/upload loop (+9% here, because the kernel socket buffers already hide part of the stall on loopback), and when the destination is slow per stream it scales with K (K=4: 77 vs 24 MB/s for `legacy` t1).
- It is **not** better than the existing parallel legacy path (`multipartThreads` 2..4): that path opens several source connections, so with a per-stream limit on the source it is about twice as fast, uses about a tenth of the memory (no part buffers) and about the same CPU. A single sequential source stream cannot exceed one stream's speed.
- The pipeline engine costs 3x the memory per object of `legacy` t1 and 10-15% more CPU per GiB in these runs (page faults of fresh pool buffers, extra copy through a part buffer); with one shared pipe (total cap) nothing helps, all engines are bound by the pipe.
- Hence `legacy` stays the default. If your real bottleneck is a per-connection limit, try `multipartThreads` 2..4 first (legacy, no extra memory) at lower `concurrency`; use `pipeline` when you need resume or when the source is a single fast stream and the destination is slow per stream.

Run the benchmarks yourself with `BENCH_NET=<scenario> go test ./service/worker/copy -run '^$' -bench 'Copy/' -benchtime 1x -benchmem` (scenarios in `service/worker/copy/bench_test.go`) and take a real profile first (see Profiling the worker).

### Resumable multipart (`worker.resumeMultipart`, off by default)

With `copyEngine: pipeline` and `worker.resumeMultipart: true` (System setting, or `resumeMultipart: true` in the worker YAML), an interrupted large copy continues instead of restarting. The worker stores `{uploadId, source ETag, size, last modified, part size, completed parts}` in the queue Redis under `p:mpu:{<replication id>:<generation>}:<sha1(key, version)>` (TTL 7 days, refreshed while parts complete, writes throttled to one per 2 s, deleted on success and on abort). On a retry it checks that the source ETag/size/last-modified and the part size are unchanged, asks the destination which parts exist (`ListParts` is the source of truth: part sizes and ETags are verified, parts the destination does not list are uploaded again), reads the source with a ranged GET (`If-Match`) from the first missing part, uploads only the missing parts and completes with all parts in order. Anything unexpected (changed source, different part size, unknown upload, listing error, duplicate or mismatching parts) aborts the old upload (fresh context) and restarts from zero. A failure that is not transient (client errors, source gone or changed) aborts the upload and deletes the record. The orphan sweep does not abort the upload named by the record.

**Warning.** This is new and unproven against your Ceph: it relies on `ListParts` behaving per S3, on part ETags staying stable and on the destination keeping incomplete uploads for as long as the record lives (a lifecycle rule `AbortIncompleteMultipartUpload` shorter than the retry delay defeats it). Keep it off in production until you have killed a worker mid-copy in a test setup and verified the object (size and a checksum) afterwards. Resume applies to initial-replication/diff-fix copies only (not live events, not versioned migrations). Switching it off is immediate: records are ignored and expire by themselves.

### TODO: checksum trailers

Not implemented: end-to-end integrity with `x-amz-checksum-crc32c` trailers (aws-chunked) on uploads. Before doing so, probe the destination Ceph RGW for support (a one-object test with a trailing CRC32C, reading back `x-amz-checksum-crc32c`) and measure the CPU cost, since the worker would hash every byte. Today integrity rests on TLS, the part ETags and the final size check.

## Orphaned multipart uploads

A copy that fails or is cancelled in the middle of a multipart upload leaves an incomplete upload on the destination that keeps using quota until it is aborted. minio-go aborts with the copy context, which is already cancelled on shutdown, task timeout or retry, so the abort failed silently. The worker now (a) aborts with a fresh context (1 minute, 3 tries) after a failed multipart copy and (b) at the start of a multipart copy lists the incomplete uploads of exactly that key and aborts the ones older than 5 minutes. Both run only while the worker holds the distributed destination-key lock, which guarantees that no other worker uploads the same key (a younger upload is never touched). Switch: `orphanUploadCleanup: false` in the worker YAML (env `CFG_ORPHANUPLOADCLEANUP=false`) restores the old behaviour. Cost: one `ListMultipartUploads` request per multipart copy.

Uploads older than the sweep window of keys that are never copied again are not found by this mechanism; use a bucket lifecycle rule `AbortIncompleteMultipartUpload` on the destination (for example 7 days) as a safety net.

## Profiling the worker

The worker can serve Go pprof profiles (CPU, heap, goroutines, trace) to find out where CPU time and memory go during a real copy. It is **off by default**: nothing listens unless `debug.pprofAddr` (env `CFG_DEBUG_PPROFADDR`) is set. The endpoint is unauthenticated, so bind it to loopback and reach it from inside the container or pod:

```yaml
debug:
  pprofAddr: "127.0.0.1:6060"
```

```bash
# Docker Compose: set CFG_DEBUG_PPROFADDR=127.0.0.1:6060 in the worker environment, then a 30 s CPU profile
docker compose exec worker sh -c 'wget -qO /tmp/cpu.pprof "http://127.0.0.1:6060/debug/pprof/profile?seconds=30"'
docker compose cp worker:/tmp/cpu.pprof . && go tool pprof -top cpu.pprof
# Kubernetes
kubectl port-forward deploy/<release>-worker 6060:6060   # then: go tool pprof 'http://127.0.0.1:6060/debug/pprof/profile?seconds=30'
# heap: /debug/pprof/heap, goroutines: /debug/pprof/goroutine?debug=1
```

Take the profile while copies are running. Do not publish the port (no `ports:` entry, no Service). The metrics server (`metrics.port`) does **not** serve `/debug/pprof/` (removed for security); profiling is only available through `debug.pprofAddr`, bound to `127.0.0.1` or a pod-local address.

## Replication throughput tuning

### Hardware-aware defaults

At startup the worker detects the CPU and memory **actually available to the process**: the smaller of host values and the container limit (cgroup v2 `cpu.max`/`memory.max`, cgroup v1 `cfs_quota`/`limit_in_bytes`), also capped by an explicitly set `GOMAXPROCS`. In Kubernetes these are the pod `resources.limits` (set requests equal to limits for predictable tuning); with Docker, `--memory`/`--cpus` (compose `mem_limit`/`cpus`). A fractional CPU quota is rounded up for the CPU count (1500m is 2) and the fractional value is kept and shown. When a cgroup memory limit applies and `GOMEMLIMIT` is unset, the worker sets `GOMEMLIMIT` to 90% of it.

Knobs with no explicit value (stored setting `0`, YAML/env `0`) use a value derived from that hardware. Explicit values always win. One log line at startup lists the detected hardware and each effective value as `auto` or `explicit`; `GET /tutti/system-settings` returns `hardware`, `recommended`, `effective` and `tuningSources`, and the console shows the detected hardware and an Auto badge.

| Knob | System setting (`worker.*`) | YAML / env | Auto value | Applies |
|---|---|---|---|---|
| Task concurrency | `concurrency` (0 = auto, else 1..1000) | `concurrency` / `CFG_CONCURRENCY` | about 10 per CPU, at most 128, at most buffer budget / 32 MiB, at least 10 | worker restart |
| Bulk share of concurrency | `bulkConcurrencyPercent` (0 or 5..95) | `bulkConcurrencyPercent` / `CFG_BULKCONCURRENCYPERCENT` | 20 (up to 3 CPUs), 30 (4..7), 40 (8+) | worker restart |
| Buffer budget | `bufferBudgetBytes` (0 = auto, else at least 16 MiB; never unlimited) | none | 25% of memory, clamped to 256 MiB..8 GiB | next copy, no restart |
| Parallel multipart parts | `multipartThreads` (0 or 1..16) | `multipartThreads` | 1 | next copy, no restart |
| Multipart part size | `multipartPartSizeMib` (0 or 5..512) | `multipartPartSizeMiB` | 16 MiB | next copy, no restart |
| Copy engine | `copyEngine` (`""`, `legacy`, `pipeline`) | `copyEngine` | `legacy` | next copy, no restart |
| Resumable multipart | `resumeMultipart` (bool) | `resumeMultipart` | off | next copy, no restart |
| Large-object streams | `largeObjectStreams` (0 or 1..64) | `largeObjectStreams` / `CFG_LARGEOBJECTSTREAMS` | 2 per CPU, clamped to 4..16 | next task, no restart |

| CPU / memory | concurrency | bulk % | buffer budget |
|---|---|---|---|
| 1 / 1 GiB | 10 | 20 | 256 MiB |
| 2 / 4 GiB | 20 | 20 | 1 GiB |
| 4 / 32 GiB | 40 | 30 | 8 GiB |
| 16 / 64 GiB | 128 | 40 | 8 GiB |

The floors equal the former static defaults (10, 20%, 256 MiB), so small hosts are never worse than before. The buffer never exceeds a quarter of memory (above the 256 MiB floor) so Go heap overhead, TLS and GC headroom remain.

`multipartThreads` stays at 1 unless you set it. Parallel parts were 3-4x slower than a single stream in local loopback tests (cause not isolated), so any gain over a WAN is unverified: raise it only after measuring on your own network, and revert if throughput does not improve.

#### Large-object concurrency cap

Objects of 128 MiB or more (the same threshold MinIO replication uses) take one of `largeObjectStreams` slots of their **destination storage** before they are copied. Smaller objects never take or wait for a slot. The reason: dozens of hour-long copies in parallel only share the same network pipe, keep every object unfinished for longer and hold buffers and sockets, while a few streams finish objects in minutes. The cap also covers the case where the live worker slots pick up bulk copies (queue fallback, see `pkg/tasks/queue.go`), because the check runs in the copy task itself on whatever slot runs it.

How waiting works: the slot is requested at the very start of the initial-replication/diff-fix copy task, before the object lock and rate-limit tokens. If all slots of that destination are busy the task does **not** wait: it returns a retryable "rate limited" error and asynq re-queues it after a random 3..10 s without counting a failed attempt. No worker slot is parked, so live replication events (which are never capped or delayed) and small objects keep flowing. Cost: a polling retry per waiting large object (one Redis round trip each 3..10 s). The cap is per worker process (each worker replica has its own counters), so N replicas allow N times the setting. Versioned-object migration copies are not capped.

Roll back by setting a large value (for example 64): then the cap effectively never triggers. The value is read at every task, so changes apply immediately.

In the System settings API an omitted worker field keeps the stored value; an explicit `0` resets it to auto (`bandwidthLimitBytes`: unlimited).

Precedence: non-zero system setting, then non-zero YAML/env, then the hardware-derived value. **Upgrading:** installations created before this feature stored `concurrency: 10` and `bufferBudgetBytes: 268435456` as explicit values; they stay in force. Set both to `0` (console: Settings, Worker, Use auto) to switch to auto tuning. The Helm chart no longer sets `worker.config.concurrency`; remove such an override from your values if you want auto tuning.

Throughput is visible at `GET /tutti/worker-stats` and as Prometheus metrics `tutti_copy_objects_total{result}`, `tutti_copy_bytes_total`, `tutti_copy_duration_seconds`, `tutti_copy_inflight`, `tutti_copy_buffer_wait_seconds_total`, `tutti_copy_buffer_used_bytes`, `tutti_copy_buffer_total_bytes`. A growing `bufferWaitSecondsTotal` means the buffer budget limits throughput.
