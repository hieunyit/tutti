# Operating Tutti

How each Tutti mechanism behaves and how to run it. Config keys are described in [configuration.md](./configuration.md); API and CLI in [api.md](./api.md).

## Payload handling (aws-chunked)

Source: `service/proxy/auth/streaming.go`, `pkg/s3/chunked/chunked.go`.

Clients such as `mc`, `minio-go` and AWS SDKs upload with `x-amz-content-sha256: STREAMING-*` and an `aws-chunked` body. Chunk signatures chain from the client's key, so they cannot be forwarded once the proxy re-signs the request for the storage. Tutti therefore:

1. Authenticates the request (SigV4 header signature) and keeps the signing material.
2. Wraps the body in a decoder that yields only the decoded object bytes and verifies as it goes.
3. Forwards a plain body with `x-amz-content-sha256: UNSIGNED-PAYLOAD`, `Content-Length` set to `x-amz-decoded-content-length`, and the `aws-chunked` token removed from `Content-Encoding` (`X-Amz-Decoded-Content-Length` and `X-Amz-Trailer` are removed).

Supported modes: `STREAMING-AWS4-HMAC-SHA256-PAYLOAD`, `STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER`, `STREAMING-UNSIGNED-PAYLOAD-TRAILER`. Other `STREAMING-*` values get `501 NotImplemented`. `x-amz-decoded-content-length` is required (`MissingContentLength` otherwise).

Verification:

- Every chunk signature, the final empty chunk, and the trailer signature (signed-trailer mode) are checked. A mismatch gives `403 SignatureDoesNotMatch`.
- `x-amz-checksum-crc32 | crc32c | crc64nvme | sha1 | sha256` are verified against the decoded payload, either from the trailer (when declared in `x-amz-trailer`) or from the request header. Header checksums are verified only for `PutObject` and `UploadPart`. Mismatch gives `400 XAmzContentChecksumMismatch`.
- The decoded length must match `x-amz-decoded-content-length`; a truncated body gives `400 IncompleteBody`; malformed framing gives `400 InvalidRequest`.
- The decoder always withholds the most recent piece until the next one is verified and withholds the final piece until all end-of-body checks pass. A consumer that stops after the declared length therefore cannot commit a payload that failed verification. Tested against MinIO: tampered chunks are rejected and no object is created.

`Content-Type` signing: by default it need not be signed; set `auth.requireSignedContentType: true` to require it. `host` and all `x-amz-*` headers must always be signed.

Presigned URLs and plain `UNSIGNED-PAYLOAD`/hex-SHA256 uploads are not affected.

## Replication event recording and 503 semantics

Source: `service/proxy/router/server.go`.

Order of events for a write that is replicated:

1. The proxy forwards the request to the routed storage.
2. If the storage answered successfully, it records a replication task per change in the Redis work queue, retrying with exponential backoff (50 ms, doubling) within `replication.eventRetryBudget` (default 2 s) shared across the request.
3. If recording still fails and `onEventError: fail` (default), the client gets `503 ServiceUnavailable`: "The change was applied but could not be recorded for replication. Please retry the request."

Meaning for operators and clients:

- The data **is on the routed (main) storage** when this 503 is returned.
- Retrying is safe: S3 writes are idempotent, and the retry records the event again. Only a successful retry guarantees the replica will receive the change.
- Typical causes: Redis out of memory with `noeviction`, Redis read-only after failover, Redis unreachable.
- Verified against MinIO + Redis: with Redis rejecting writes the proxy answers 503 and a client retry (after Redis recovers) records the event.
- With `onEventError: log` the response is the storage's, the error is only logged, and the follower can silently diverge.
- Tasks for a request that failed on the storage (S3 API error) are not recorded.

Log line to watch: `unable to handle replication` with fields `bucket`, `object`, `on_event_error`.

## Synchronous replication

Source: `service/proxy/router/sync_replication.go`, `service/proxy/server.go`, `test/sync_replication_test.go`.

With `replication.sync.enabled: true` the proxy, after a successful main write and successful event recording, copies the object to every replication destination of the request in parallel and only then answers.

What is synced: only `ObjectSyncPayload` tasks, which are produced by `PutObject`, `CopyObject`, `CompleteMultipartUpload`, `DeleteObject` and `DeleteObjects`. In practice: object content and deletes. ACL, tagging and bucket-level tasks (including ACL/tag tasks that accompany a PutObject) stay asynchronous and are processed by the worker.

Multipart: `UploadPart` requests are not synced. At `CompleteMultipartUpload` the finished object is copied in full to each destination, so completion latency grows with object size.

Destinations are those replications in the request context, that is, replications whose source is the storage the request was routed to (see [upgrading](./upgrading-from-chorus.md)).

Quorum math:

- `total = 1 + destinations`. `quorum = writeQuorum`, or `total` if `writeQuorum` is `0` or larger than `total`.
- `copies = 1 + destinations that synced successfully`. All destinations are always attempted (unless skipped, see below) and the proxy waits for all of them up to `timeout`; the quorum only decides the outcome, it does not shorten the wait.
- If `copies >= quorum`: normal response. If some destinations failed, a warning `sync replication: not all destinations are up to date, remaining replication is asynchronous` is logged and the worker completes them.
- If `copies < quorum`: `onQuorumFailure: fail` answers `503 ServiceUnavailable` ("the write was stored on the primary storage but the replication quorum was not reached; the change will be replicated asynchronously. Retry the request to obtain the guarantee."). `accept` forwards the main response.
- Destinations whose breaker is open are skipped and counted as failed. If `1 + available destinations < quorum`, nothing is attempted and the request fails (or is accepted) immediately.

Header: `X-Tutti-Replicas: <copies>/<total>` is set on responses of synced requests (both normal and `accept`), for example `2/3` means main plus one of two destinations. It is absent when there was nothing to sync. A 503 quorum failure has no such header.

Failure semantics: in all cases the main write stands and the replication task remains queued, so a destination that is down converges once it is back. `fail` only changes what the client sees.

Latency trade-off: write latency becomes main write + the slowest destination (bounded by `timeout`, default 30 s). Choose `writeQuorum` and `timeout` accordingly; a small `timeout` with `onQuorumFailure: accept` bounds latency while still replicating asynchronously.

Verified: with sync mode, objects are visible on followers when the PUT returns.

## Circuit breaker and health checks

Source: `pkg/health/`, `pkg/s3client/http_forward.go`, `service/proxy/server.go`.

Each proxy keeps one breaker per configured storage. Inputs:

- Active probes: every `healthCheckInterval` (default 10 s) a `ListBuckets` call with one configured credential. A probe still running when the next tick fires makes that tick skipped.
- Real traffic: every request forwarded to the storage reports its outcome.

States:

- **Closed** (available): requests pass. A success resets the failure counter.
- **Open**: after `failureThreshold` consecutive failures (default 3). Requests to this storage fail immediately with `503 ServiceUnavailable` (`storage <name> is temporarily unavailable`) instead of waiting for `httpTimeout`. Synchronous replication skips it.
- **Trial**: once `openDuration` (default 30 s) has elapsed, exactly one request is let through; success closes the breaker, failure keeps it open for another period. Probes keep running while open, and any successful observation (probe or trial) closes the breaker.

Detection time: with probes only, about `failureThreshold x healthCheckInterval` (defaults: 3 x 10 s = ~30 s), plus up to one probe timeout (`min(3s, interval/2)`) for a blackholed address. Failed real requests count toward the threshold, so under traffic it can be faster. Recovery is noticed at the next successful probe (up to one interval).

Verified against a blackholed main: once the breaker is open, requests fail in about 50 ms instead of the 60 s `httpTimeout`; opening took 3 failures, about 30 s with defaults.

Observability:

- Metrics (only when `metrics.enabled`): gauge `storage_available{storage}` (1 closed, 0 open), counter `storage_breaker_transitions_total{storage,to}` with `to` = `open` or `closed`.
- Logs: warn `storage circuit breaker opened: requests fail fast until the storage recovers` and info `storage circuit breaker closed: storage is available again`.
- `chorctl proxies`, `GET /tutti/proxies`, UI `/health`.

Breaker state is per proxy instance and in memory; it starts closed after a restart.

## Automatic failover

Source: `service/proxy/failover/failover.go`, `pkg/policy/failover.go`, `pkg/failover/store.go`, `test/resilience/failover_test.go`.

Off by default. Needs `failover.enabled: true` in the proxy **and** an enabled policy for the source storage (`{source, enabled, targets: [ordered], after, maxEventLag}`), set via the [API](./api.md).

### Trigger

Every `checkInterval` the controller looks at the proxy's own breaker states. For each storage with an open breaker and an enabled policy with targets, it proceeds when the breaker has been open for at least the policy `after` (time since the breaker opened; default `failover.after` = 60 s).

### Scopes

A scope is a user-level or bucket-level routing target that is currently routed to the failed storage (by explicit routing, or implicitly because the storage is `main`) and has replications from it. Each scope is handled independently. User-level and bucket-level replications of the same user are mutually exclusive in Chorus.

### Candidate checks

Targets are tried in the operator's order; the first that passes all checks wins. A replication source -> target qualifies when:

- it exists for that scope (bucket-level for bucket scopes, else user-level) and the destination bucket name equals the source bucket name;
- the target is not the source and the target's breaker is closed;
- the replication is not archived, has no replication switch in progress, and its initial migration is done; an operator pause blocks promotion, while a pause belonging to an unfinished automatic failover may be resumed;
- unprocessed replication events (queued event tasks) <= `maxEventLag`.

Only storages listed in `targets` are considered, even if other replications exist. If nothing qualifies, a warning with the reasons is logged once per scope per outage and routing is unchanged.

### Single actor

The controller takes a Redis lock `tutti:failover-lock:<failed storage>` (lock DB, TTL 30 s) and re-reads the scopes under the lock. If another proxy holds it, this one does nothing. All proxies read routing from Redis on every request without caching, so the new routing is effective everywhere immediately.

### What changes in Redis

Per failed-over scope:

1. The replication status records `failover_pending=true`, then the source -> promoted target replication is **paused**. If a later step fails, another controller can resume the attempt from this persisted state after a restart.
2. Routing for the scope is pointed to the target (user routing, or bucket routing), and `failover_pending` is removed in the same Redis transaction. For a user-level scope, explicit bucket routings of that user that point to the failed storage are moved to the target too. Explicit operator pause/resume clears the pending marker and cancels recovery ownership.
3. A failover event is stored: `tutti:failover:user:<user>` or `tutti:failover:bucket:<user>:<bucket>`, JSON `{user, bucket, from, to, time, eventLag}`.

Other replications from the failed storage (for example main -> f2 when f1 was promoted) are not paused, but requests routed to the promoted storage only carry replications whose source is the routed storage, so they do not record events against the failed side.

Observability: warn log `failover: routing switched to replica and replication from the failed storage paused; failback is manual`, counter `failover_total{from,to}` (when `metrics.enabled`), `chorctl failover events`, UI `/failover`. If the routing change succeeded but the event could not be saved the error `failover performed but event not saved` is logged.

Verified: with a policy of `after=20s` failover took about 60 s end to end (breaker detection ~30 s + 20 s + check interval). There is no automatic failback.

### Manual failback

Failback never happens automatically. After the old storage is repaired:

1. Repair/restore the old (source) storage and confirm it is healthy (`chorctl proxies` shows it `up`).
2. Delete the paused old -> new replication:
   `chorctl repl delete -f old -t new -u <user> [-b <bucket>]`.
3. Create the reverse replication so the old storage catches up with writes made during the outage:
   `chorctl repl add -f new -t old -u <user> [-b <bucket>]`, and wait until it is done and the event lag is 0 (`chorctl repl`, `chorctl dash`).
4. Switch routing back using the replication switch APIs (`chorctl repl switch zero-downtime ...` or `chorctl repl switch downtime ...`, see [live-migration](../live-migration.md)).
5. Acknowledge the failover event so it disappears from the list: `PUT /tutti/failover/events/delete` (UI `/failover` -> acknowledge), or `redis-cli del tutti:failover:user:<user>` (bucket scope: `tutti:failover:bucket:<user>:<bucket>`).
6. Re-create the old -> new replication if you want the original topology back.

Until step 5 the event stays listed; it has no effect on routing.

## Managing S3 users and console users

- **Creating an S3 user** (`POST /tutti/users`, see [api.md](api.md#s3-users-api)) needs `storage.dynamicCredentials.enabled` with a `masterPassword` on the workers and proxies. Use `verify: true` to catch wrong keys before saving. The user should have credentials on every storage (`complete: true` in `GET /tutti/users`), otherwise replication to or failover onto a storage lacking them cannot work. Give the credentials of each storage; the proxy access key (`createAccessKey: true`) is what clients use against the proxy.
- **Changing credentials**: `PUT /tutti/users/credentials` writes the sealed credentials to Redis; every worker and proxy picks them up on its next poll (`storage.dynamicCredentials.pollInterval`). Credentials defined in the worker config can only be changed in the config.
- **Deleting an S3 user** drops the dynamic credentials on all storages and deletes dynamic access keys and the access policy. Delete the user's replications, routings and failover events first. If a user's credentials come from the config, remove them there (and restart) before the remaining dynamic parts can be cleaned; the API never edits config. Deleted credentials disappear from proxies and workers on the next poll; until then requests with its access key may still be served.
- **Console users**: keep at least one admin in `api.auth.users`: it can always sign in even if all managed admins are lost or Redis data is wiped. To recover a locked-out managed user, sign in as a config admin and use `PUT /tutti/console-users/password`, or delete the record: `redis-cli -n <configDB> hdel tutti:console-users <username>`. Disabling, deleting or resetting a user invalidates its sessions immediately. A generated password is shown once; share it over a secure channel (the user is flagged `mustChangePassword` until it is changed through `POST /auth/password`).

## Multi-instance considerations

- Breakers, probes and the failover controller run in **each proxy instance** with its own view. Instances can briefly disagree about a storage's state (for example a network partition seen by one of them).
- Failover is decided by the instance(s) whose own breaker has been open for `after`; the Redis lock lets only one act per failed storage at a time, and the others re-check scopes under the lock and find nothing to do. Routing is shared through Redis, so every instance follows.
- A failover triggered by one instance is not conditional on the other instances also seeing the storage down. Use a conservative `after`.
- Enable `failover.enabled` consistently across instances; instances with it off never act, but follow routing.
- Each instance publishes its status to `tutti:proxy-status:<app_id>` every `status.interval` (default 2 s) with a TTL of 3 intervals; a stopped instance disappears from `chorctl proxies` after at most 3 intervals.
- Sync replication settings are per instance; keep them identical or clients will see different guarantees depending on which instance they hit.
- Failover policies and events live in the config Redis DB, and locks in the lock DB, shared by all instances and the worker.
