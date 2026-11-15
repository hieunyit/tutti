# Upgrading from Chorus

Tutti started as a fork of Chorus, but after the rename it is **not wire- or data-compatible** with upstream Chorus. There is no automatic upgrade of an existing Chorus deployment. This page lists what changed and every behavior that differs from upstream. Config details: [configuration.md](./configuration.md).

## Compatibility

| Area | Chorus | Tutti | Compatible |
|---|---|---|---|
| Go module | `github.com/clyso/chorus` | `github.com/hieunyit/tutti` | No (imports, `go install` paths) |
| Binary / image entrypoint | `/bin/chorus` (`worker`, `proxy`, `chorus`) | `/bin/tutti`; CLI `tuttictl` | No |
| Admin recovery | `chorus admin reset-password` | `/bin/tutti admin reset-password` | No |
| Protobuf package | `chorus` | `tutti` | No: existing gRPC clients must be regenerated |
| gRPC service paths | `/chorus.Chorus...` | `/tutti.Tutti...` (e.g. `TuttiCoreService`) | No |
| Redis keys | `chorus:*` and related | `tutti:*` | No: Tutti refuses to start with `incompatible with existing Redis schema` |
| Postgres control plane migrations | upstream set | changed set | No: existing volumes fail with `checksum mismatch` |
| S3 proxy API and config format (S3 clients, storage settings) | | | Same concepts; config keys are listed below |

## Migrating existing data

- New deployments need nothing special.
- Existing Chorus data (Redis, Postgres) cannot be upgraded in place. Re-create the configuration in a fresh Tutti deployment, or export it from Chorus and import it again (storages, users and credentials, routing, replication policies). Object data in your S3 storages is not touched; start replications again and let the initial copy reconcile.
- `docker compose down -v` removes the old volumes, which also deletes all configuration. Export storages and users first.
- Do not point Tutti at a Redis or Postgres that still holds Chorus data.
- Rolling back to Chorus requires its original volumes; Tutti-written data is not readable by Chorus.

## Renames

- The CLI is `tuttictl`; Tutti binary is `/bin/tutti`.
- Redis: Tutti uses its own `tutti:*` keys (below) and does not read Chorus keys.

## Behavior changes

| Area | Chorus | Tutti | Restore upstream |
|---|---|---|---|
| `Content-Type` signing | A request with a `Content-Type` header not listed in `SignedHeaders` is rejected | Not required (AWS SigV4 only requires `host` and `x-amz-*`), so `mc`/`minio-go` uploads work | `auth.requireSignedContentType: true` |
| Streaming uploads (`STREAMING-*`, aws-chunked) | Rejected / not decoded | Decoded and verified (chunk and trailer signatures, `x-amz-checksum-*`, length) at the proxy; storage receives a plain `UNSIGNED-PAYLOAD` body | No switch (upstream rejected these uploads) |
| Replication event recording failure (Redis down/OOM/read-only) | Logged; client gets the storage's 2xx; replica silently misses the change | After retrying for `replication.eventRetryBudget` (2 s) the client gets `503 ServiceUnavailable`; a retry records the event | `replication.onEventError: log` |
| Replications in the request context | All replications of the user/bucket scope | Only replications whose source is the storage the request was routed to (replications to another bucket on the same storage are kept) | No switch. Only differs after a failover; without failover every replication of a scope already starts at its routed storage |
| Requests to a dead storage | Wait for `httpTimeout` (default 1 m) | Fail fast with `503` once the storage's breaker is open | Not switchable; set a large `health.failureThreshold` to make opening unlikely |
| Background probing | None | Every S3 storage is probed every `healthCheckInterval` (default 10 s) with `ListBuckets` using a configured credential | Not switchable |

## New Redis keys

- `tutti:proxy-status:<app_id>` (config DB, TTL 3x interval): written by every proxy every `status.interval` (default 2 s). This is a small recurring write per proxy.
- `tutti:failover-policies` (config DB): created only when a policy is set.
- `tutti:failover:user:<user>`, `tutti:failover:bucket:<user>:<bucket>` (config DB): created only on an automatic failover.
- `tutti:failover-lock:<storage>` (lock DB): only while a failover runs.

These keys are Tutti-specific.

## New config sections

All default to upstream-compatible or off, except `replication.onEventError` (now `fail`) and `auth.requireSignedContentType` (now `false`, i.e. looser):

| Section | Default | Effect |
|---|---|---|
| `replication.sync.*` | `enabled: false` | Asynchronous replication as in Chorus |
| `health.*` | threshold 3, open 30 s | Circuit breaker is always active, fail-fast is on |
| `failover.*` | `enabled: false` | No automatic failover |
| `storage.storages.<n>.healthCheckInterval` | `10s` | Probe period |

## Keeping upstream behavior

```yaml
auth:
  requireSignedContentType: true   # strict content-type signing
replication:
  onEventError: log                # never answer 503 for event recording failures
  sync:
    enabled: false
failover:
  enabled: false
```

The breaker and probes cannot be disabled. With `health.failureThreshold` set very high the breaker practically never opens, but the probes still run.

## Operational checklist

1. Make sure clients retry on `503` (most S3 SDKs and tools retry `503` by default; check yours) before leaving `onEventError: fail`.
2. Alert on `storage_available == 0` and on `storage_breaker_transitions_total` (enable `metrics.enabled`).
3. If you enable `failover`, create replications first; failover needs a replication to each target. See [operations](./operations.md#automatic-failover).
4. Deploy the worker with the new version before using the Resilience API or UI pages; the REST paths live on the worker.
