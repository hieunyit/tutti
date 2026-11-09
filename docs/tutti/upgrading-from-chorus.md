# Upgrading from Chorus

Tutti is a drop-in fork of Chorus (compatible protobuf package, gRPC service names, Redis schema, config format). This page lists every behavior that differs from upstream and how to restore upstream behavior. Config details: [configuration.md](./configuration.md).

## Renames

- Go module: `github.com/clyso/chorus` -> `github.com/hieunyit/tutti` (imports, `go install` paths). Binary names (`worker`, `proxy`, `chorus`, `chorctl`) and the protobuf package/service names (`chorus.*`) are unchanged, so existing gRPC/REST clients keep working.
- Redis: all existing keys are unchanged. Tutti only adds new `tutti:*` keys (below).

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

- `tutti:proxy-status:<app_id>` (config DB, TTL 30 s): written by every proxy every 10 s. This is a small recurring write per proxy.
- `tutti:failover-policies` (config DB): created only when a policy is set.
- `tutti:failover:user:<user>`, `tutti:failover:bucket:<user>:<bucket>` (config DB): created only on an automatic failover.
- `tutti:failover-lock:<storage>` (lock DB): only while a failover runs.

Rolling back to Chorus leaves these keys behind; they are Tutti-specific and can be deleted.

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
