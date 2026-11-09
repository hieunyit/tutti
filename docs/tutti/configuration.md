# Tutti configuration reference

Tutti adds the keys below to the Chorus proxy config ([service/proxy/config.yaml](../../service/proxy/config.yaml), embedded as the default config and overridden by your file). Everything else in the proxy and worker config is unchanged from Chorus.

Source: `service/proxy/config.go`, `service/proxy/auth/config.go`, `pkg/health/breaker.go`, `pkg/s3/config.go`.

Defaults below are the effective values: the embedded `config.yaml` is always loaded first, so omitting a key gives the default listed here.

## Summary

| Key | Type | Default | Section |
|---|---|---|---|
| `auth.requireSignedContentType` | bool | `false` | [Auth](#auth) |
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
| `failover.enabled` | bool | `false` | [Automatic failover](#automatic-failover) |
| `failover.checkInterval` | duration | `10s` | [Automatic failover](#automatic-failover) |
| `failover.after` | duration | `60s` | [Automatic failover](#automatic-failover) |
| `failover.maxEventLag` | int | `0` | [Automatic failover](#automatic-failover) |

Durations use Go syntax (`500ms`, `30s`, `2m`). Every key is validated at proxy startup (`Config.Validate`); an invalid value stops the proxy with an error naming the key.

## Auth

```yaml
auth:
  requireSignedContentType: false
```

- `requireSignedContentType` (bool, default `false`): when `true`, a SigV4 request carrying a `Content-Type` header is rejected unless `content-type` is listed in `SignedHeaders`. When `false`, `Content-Type` need not be signed (AWS only requires `host` and `x-amz-*` headers to be signed; `mc` and `minio-go` do not sign it).
- `host` and every `x-amz-*` header present in the request must be signed regardless of this setting.
- Not present in the shipped `config.yaml`; set it only if you want the strict behavior.

Source: `service/proxy/auth/config.go`, `pkg/s3/auth.go` (`validateSignedHeaders`).

## Managed console users (worker)

Console users sign in to the worker management API and the UI. They come from two sources:

- `api.auth.users` in the worker config (bcrypt `passwordHash`, role `admin` or `viewer`): read-only, always win on name conflicts.
- Managed users stored in Redis hash `tutti:console-users` in the **config DB** (`redis.configDB`), created and changed through the console users API ([api.md](api.md#console-users-api)). Nothing extra needs to be configured: they are available whenever `api.auth.enabled: true`.

```yaml
api:
  auth:
    enabled: true
    sessionTTL: 12h
    users:            # at least one is required; keep an admin here so the system is always administrable
      - username: admin
        passwordHash: "$2b$12$..."
        role: admin
```

Passwords of managed users must be 12-72 bytes and are stored as bcrypt (cost 12). Sessions live in the meta DB (`redis.metaDB`). With `api.auth.enabled: false` the console users API returns failed precondition (`sign-in is disabled`).

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

When enabled, after the main write succeeded and its event was recorded, the proxy copies the object to the replication destinations before answering the client. The queued replication task stays as a safety net: destinations that failed or timed out are completed by the worker.

- `enabled` (bool, default `false`).
- `writeQuorum` (int, default `0`): number of copies, **main included**, that must exist before answering. `0` means main plus all destinations. A value larger than `1 + destinations` is capped to `1 + destinations`. Negative values are rejected.
- `timeout` (duration, default `30s`): maximum time spent syncing destinations per request (all destinations in parallel). Must be greater than `0` when `enabled` is `true`, otherwise startup fails.
- `onQuorumFailure` (`fail` | `accept`, default `fail`): what to do when fewer than `writeQuorum` copies exist. `fail` answers `503 ServiceUnavailable`; `accept` forwards the main storage response. Any other value is rejected. An empty string behaves as `fail`.

Interactions:

- Sync only does something for requests that have replication destinations, that is, an active replication policy (user-level or bucket-level) from the routed storage. Without replication policies nothing is synced and no header is added.
- Only object content/delete tasks are synced (see [operations](./operations.md#synchronous-replication)). ACL, tagging and bucket tasks stay asynchronous.
- Sync is skipped for a request whose event recording failed with `onEventError: log`.
- A destination whose circuit breaker is open is not attempted; if that makes the quorum unreachable the proxy does not wait for any destination.
- The proxy needs Redis lock access (`redis.lockDB`) for the destination-object lock, the same one the worker uses.
- The response carries `X-Tutti-Replicas: <copies>/<1+destinations>`.

Source: `service/proxy/config.go` (`SyncConfig`), `service/proxy/router/sync_replication.go`, `service/proxy/server.go`.

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
- Probes (a `ListBuckets` call with the alphabetically first configured credential of the storage) run only for S3 storages. A storage without credentials is not probed. Swift storages are not probed.
- The same `health` settings apply to all storages in one proxy; the interval is per storage.
- Metrics (when `metrics.enabled`): `storage_available`, `storage_breaker_transitions_total`.

Source: `pkg/health/`, `pkg/s3/config.go`, `service/proxy/server.go` (`storageProbe`).

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

`failover` is only the global switch and the defaults. What may fail over to where is defined by **per-source-storage failover policies** stored in Redis and managed with the [Resilience API](./api.md) (REST, gRPC, `chorctl`, or the UI `/failover` page).

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
