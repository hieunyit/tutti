# Tutti Management & Resilience REST/gRPC API Reference

The Tutti Worker exposes a comprehensive management API accessible via **REST (HTTP/JSON)** on port `9671` (default) and **gRPC** on port `9670` (default). All endpoints provide end-to-end management for storage clusters, multi-active protection groups, S3 proxy identities, credential lifecycle, automated circuit breakers, and security audit logs.

- **Worker Configuration**: Requires `api.enabled: true` in `worker-conf.yaml`.
- **Default REST Base URL**: `http://<worker-host>:9671`
- **Default gRPC Address**: `<worker-host>:9670`
- **REST Protobuf JSON Conventions**:
  - Field names are `camelCase`.
  - Durations are string format (e.g. `"30s"`, `"5m"`).
  - Timestamps follow RFC 3339 (e.g. `"2026-11-11T12:00:00Z"`).
  - Unset or empty fields are emitted as zero values (`""`, `0`, or `null`).

---

## Table of Contents

1. [Authentication & Session API](#1-authentication--session-api)
2. [Console Users Management API](#2-console-users-management-api)
3. [Runtime Storage Registry & Connection Accounts](#3-runtime-storage-registry--connection-accounts)
4. [Protection Groups & Multi-Active Replication](#4-protection-groups--multi-active-replication)
5. [S3 Users & Access Policies](#5-s3-users--access-policies)
6. [Dynamic Proxy Access Keys](#6-dynamic-proxy-access-keys)
7. [Proxy Fleet, Monitoring & Circuit Breakers](#7-proxy-fleet-monitoring--circuit-breakers)
8. [Audit Logging API](#8-audit-logging-api)

---

## 1. Authentication & Session API

All administrative and operational endpoints require an HTTP Bearer JWT token obtained via `/auth/login`.

### 1.1 Sign In (`POST /auth/login`)
Authenticates a console user against the PostgreSQL control-plane database or static configuration.

- **Request Body**:
  ```json
  {
    "username": "admin",
    "password": "your-password"
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "token": "9u1x...opaque-session-token",
    "expiresAt": "2026-11-13T08:30:00Z",
    "maxExpiresAt": "2026-11-14T08:00:00Z",
    "serverTime": "2026-11-13T08:00:00Z",
    "username": "admin",
    "role": "admin"
  }
  ```
  `expiresAt` is the idle expiry now (`sessionIdleTimeout`, default 30m, after the last login or refresh); `maxExpiresAt` is the absolute cap (`sessionMaxLifetime`, default 24h, after login). All times are RFC3339 UTC; `serverTime` lets clients correct clock skew.
- **Error Codes**:
  - `401 Unauthorized`: Invalid credentials, account disabled, or failed attempt count returned (`remainingAttempts`).
  - `429 Too Many Requests`: Account throttled after excessive failed attempts.

### 1.2 Sign Out (`POST /auth/logout`)
Invalidates the current session token immediately.
- **Headers**: `Authorization: Bearer <token>`
- **Response (`204 No Content`)**

### 1.3 Current User Info (`GET /auth/me`)
Returns current identity, assigned role (`admin` or `viewer`), and password change requirements.
- **Headers**: `Authorization: Bearer <token>`
- **Response (`200 OK`)**:
  ```json
  {
    "username": "admin",
    "role": "admin",
    "source": "managed",
    "mustChangePassword": false,
    "authEnabled": true,
    "expiresAt": "2026-11-13T08:30:00Z",
    "maxExpiresAt": "2026-11-14T08:00:00Z",
    "serverTime": "2026-11-13T08:10:00Z"
  }
  ```
  This call never extends the session.

### 1.3.1 Refresh Session (`POST /auth/refresh`)
Extends the session to `min(sessionIdleTimeout, time left until the absolute cap)` from now. This is the only call that extends a session; ordinary API calls and `/auth/me` do not, so background polling cannot keep an idle session alive. The token does not change. Allowed while `mustChangePassword` is true.
- **Headers**: `Authorization: Bearer <token>` (no body)
- **Response (`200 OK`)**:
  ```json
  {
    "expiresAt": "2026-11-13T08:40:00Z",
    "maxExpiresAt": "2026-11-14T08:00:00Z",
    "serverTime": "2026-11-13T08:10:00Z"
  }
  ```
- **Error Codes**:
  - `401 Unauthorized`: Token invalid or expired, or the absolute cap was reached.
  - `429 Too Many Requests`: More than 6 refreshes per minute for the token; see the `Retry-After` header.

### 1.4 Change Own Password (`POST /auth/password`)
Allows an authenticated console user to change their own password in PostgreSQL.
- **Headers**: `Authorization: Bearer <token>`
- **Request Body**:
  ```json
  {
    "currentPassword": "current-secure-password",
    "newPassword": "new-secure-password-12chars"
  }
  ```
- **Response (`204 No Content`)**

---

## 2. Console Users Management API

Manages console operator accounts stored in PostgreSQL (`console_users` table). Requires `admin` role.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/tutti/console-users` | List all console users |
| `POST` | `/tutti/console-users` | Create a new console user |
| `PUT` | `/tutti/console-users` | Update user role or disabled state |
| `PUT` | `/tutti/console-users/password` | Reset password (with auto-generation option) |
| `PUT` | `/tutti/console-users/delete` | Delete a managed console user |

### Example: Create Console User
```bash
curl -X POST http://localhost:9671/tutti/console-users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "operator1",
    "role": "viewer",
    "generatePassword": true
  }'
```
**Response**:
```json
{
  "user": {
    "username": "operator1",
    "role": "viewer",
    "source": "managed",
    "disabled": false,
    "mustChangePassword": true,
    "createdAt": "2026-11-11T10:00:00Z",
    "updatedAt": "2026-11-11T10:00:00Z"
  },
  "generatedPassword": "Xk9#mQ2$pL8v"
}
```

---

## 3. Runtime Storage Registry & Connection Accounts

Tutti enables live registration of S3-compatible storages (Ceph, MinIO, AWS S3, etc.) with encrypted connection accounts, eliminating static configuration reloads.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/tutti/storages` | List all registered storages with health and account status |
| `GET` | `/tutti/storages/{name}` | Get specific storage details |
| `POST` | `/tutti/storages` | Register new storage with 17-step connectivity probe |
| `PUT` | `/tutti/storages` | Update storage parameters or connection accounts |
| `PUT` | `/tutti/storages/delete` | Remove a storage (blocks if in active protection groups) |
| `POST` | `/tutti/storages/test` | Execute a complete 17-step S3 capability test |
| `PUT` | `/tutti/storages/connection-account` | Zero-downtime rotation of storage connection keys |
| `POST` | `/tutti/storages/test-credential` | Lightweight `ListBuckets` credential verification |

### Example: 17-Step Connection Test
Tests connectivity, permissions, chunked uploads, and ACL capabilities before committing changes:
```bash
curl -X POST http://localhost:9671/tutti/storages/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "storage": {
      "name": "minio-east",
      "address": "http://minio-east:9000",
      "provider": "Minio",
      "isSecure": false
    },
    "connectionAccount": {
      "accessKeyId": "tutti-svc",
      "secretAccessKey": "secret123"
    }
  }'
```
**Response**:
```json
{
  "ok": true,
  "hasWarnings": false,
  "durationMs": 420,
  "probeBucket": "tutti-probe-8a7f1c",
  "steps": [
    { "id": "listBuckets", "number": 1, "name": "List Buckets", "status": "pass", "latencyMs": 12 },
    { "id": "createProbeBucket", "number": 2, "name": "Create Probe Bucket", "status": "pass", "latencyMs": 35 },
    { "id": "write", "number": 3, "name": "Put Object", "status": "pass", "latencyMs": 18 },
    { "id": "read", "number": 4, "name": "Get Object", "status": "pass", "latencyMs": 15 },
    { "id": "multipartUpload", "number": 5, "name": "Multipart Upload", "status": "pass", "latencyMs": 85 },
    { "id": "deleteProbeBucket", "number": 17, "name": "Clean Probe Bucket", "status": "pass", "latencyMs": 22 }
  ]
}
```

---

## 4. Protection Groups & Multi-Active Replication

Protection Groups manage multi-directional replication, quorum consistency, writer fencing, and switchover across independent storages.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/tutti/protection-groups` | List all protection groups, state, lag, and primary storage |
| `GET` | `/tutti/protection-groups/{id}` | Get group configuration and member status |
| `POST` | `/tutti/protection-groups` | Create a new protection group |
| `PUT` | `/tutti/protection-groups` | Update members or settings |
| `POST` | `/tutti/protection-groups/delete` | Remove a protection group |
| `POST` | `/tutti/protection-groups/switchover` | Coordinated zero-downtime switchover of primary storage |
| `POST` | `/tutti/protection-groups/abort-switch` | Abort an active switchover |
| `PUT` | `/tutti/protection-groups/automatic-freeze` | Configure automatic freeze on replica lag violations |
| `GET` | `/tutti/protection-groups/automatic-freeze/status`| Get freeze status |
| `GET` | `/tutti/protection-groups/{id}/writers` | List active writer leases |
| `POST` | `/tutti/protection-groups/clear-writer` | Clear a fenced writer lease |
| `GET` | `/tutti/unprotected-scopes` | List storages/buckets not covered by any group |

### Example: Create Bucket-Scoped Protection Group
```bash
curl -X POST http://localhost:9671/tutti/protection-groups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "pg-analytics",
    "primary": "minio1",
    "members": ["minio2"],
    "buckets": ["analytics-raw", "analytics-processed"],
    "sync": {
      "enabled": true,
      "writeQuorum": 2,
      "timeout": "15s"
    }
  }'
```

### Storage-level replication (`/replication/*`)

`user` is optional on `POST /replication/add`, `get`, `list`, `PUT pause|resume|delete`, the switch endpoints and `POST /replication/list-buckets`. An empty or omitted `user` creates a **storage-level replication**: the whole storage (or the listed buckets) is copied A to B using each storage's connection account, with no S3 user. Responses return an empty user as `""`. The internal `$service` identity no longer exists and is never returned.

```bash
# storage-level, all buckets
curl -X POST http://localhost:9671/replication/add -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":{"fromStorage":"minio1","toStorage":"minio2"}}'

# storage-level, one bucket
curl -X POST http://localhost:9671/replication/add -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":{"fromStorage":"minio1","toStorage":"minio2","fromBucket":"data","toBucket":"data"}}'

# which buckets can be selected
curl -X POST http://localhost:9671/replication/list-buckets -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"fromStorage":"minio1","toStorage":"minio2","showReplicated":true}'
```

`list-buckets` returns `{"buckets":[...],"replicatedBuckets":[...],"coveredBuckets":[{"bucket","ownerKind","ownerId"}]}`. `buckets` are the ones that can be replicated now; `coveredBuckets` lists buckets already covered for that `from -> to` edge by a protection group (`ownerKind: "group"`) or another replication (`"replication"`), with the owner id. A storage with no connection account yields an error naming that storage.

**Initial replication progress by bytes.** While the initial phase runs, `get`, `list` and the stream return, besides `initObjListed`/`initObjDone`: `initBytesTotal` (bytes of objects enumerated by the listing so far), `initBytesDone` (bytes of objects copied successfully plus the bytes already transferred of copies still running, so it grows while large objects are in flight; never above `initBytesTotal`), `initBytesPerSecond`, `initEtaSeconds` and `initListingComplete` (int64 values are JSON strings). `initBytesTotal` is only a lower bound until `initListingComplete` is `true` (show "at least"). `initBytesPerSecond` is the real transfer speed: bytes sent to the destination in the last 3 minutes (1 minute buckets in Redis, fed by the progress of running copies and by the remainder at completion) divided by the elapsed window time (at least 10 s, and never longer than since the first transferred byte), so it stays accurate while huge objects take minutes to finish. Running copies report their progress at most once per second; a retried copy restarts from 0 (its re-sent bytes count for the speed but not twice for `initBytesDone`). `initEtaSeconds = (total - done) / speed`; `0` means unknown (listing still running or no throughput yet) or nothing left. Counters are Redis runtime data (same Redis as the task queues) keyed by replication id and generation (`p:repl_init_bytes:{id:gen}`, plus `:rate` and `:prog`, the latter holding per-object absolute progress with a maintained sum so a read stays O(1); in Redis-only and Postgres mode alike, no migration), reset when a replication is re-created, best effort (a Redis error never fails a copy). Versioned buckets and diff-fix copies are not counted (all byte fields `0`, only object counts are shown). `tuttictl repl` has an `INIT_BYTES` column (`12.4GB/812GB 8.7MB/s ~25h`).

**Replication activity (console log).** `POST /replication/activity` (gRPC `Policy/GetReplicationActivity`, read-only, viewer allowed) takes `{"id":{...},"afterCursor":"","limit":200}` and returns `{"inFlight":[{"key","versionId","size","startedAt","flow","attempt","bytesDone"}],"events":[{"cursor","time","level","type","key","versionId","size","durationMs","bytesPerSecond","attempt","flow","error"}],"nextCursor","serverTime"}` (int64 values are JSON strings). `inFlight` lists the object copies running now (oldest first); `bytesDone` is the number of bytes of that object transferred so far by the running attempt (initial replication only, never above `size`). `events` are in ascending cursor order; `level` is `info|warn|error` and `type` is `started|done|failed|retry|skipped`. Without `afterCursor` the newest `limit` events are returned (`limit` 1..500, default 500); with it only newer events, and `nextCursor` is the value to pass next time (it equals `afterCursor` when nothing new arrived). The cursor is opaque (a Redis stream id); a malformed one or an out-of-range `limit` is `400`, an unknown replication `404`. `done` events carry `durationMs` and `bytesPerSecond`; `error` is truncated to about 300 characters. `flow` is `init` (initial replication, diff-fix) or `live` (change events).

Where it lives and its limits: runtime data in the queue Redis, keyed by replication id and generation (`p:repl_act:{id:gen}:fly|flyz|ev`), so a re-created replication starts clean. The event log is a stream capped at about 1000 events (`MAXLEN ~`) and expires 24 h after the last write. The in-flight set is capped at 500 entries (newest kept) and an entry older than 2 h is ignored and evicted, so a crashed worker never leaves stale entries behind. The initial replication flow records every object (started, then done/skipped/retry/failed; versioned buckets record one entry per object, not per version). The live-event flow is sampled to at most 20 tracked objects per second per replication (their in-flight entry plus started and done events), failures and retries are always recorded, and "version already up to date" and delete syncs produce no event. Writes are best effort with a 250 ms timeout: a Redis error never fails or retries a copy (debug log only). `tuttictl repl logs <id> [-f] [--tail N]` prints the log, and the console opens it by clicking a replication name.

**Display name.** `POST /replication/add` accepts an optional `name`, and `PUT /replication/rename` (`{"id":{...},"name":"..."}`, empty clears it) changes it later; `get`, `list` and the stream return `name` (empty when unnamed). The name is a label only: at most 64 characters (counted as characters, after trimming), no control characters, not unique, and never part of the replication id, the scope lock or the overlap rule, so renaming does not touch copy progress or routing. A longer or invalid name is `400`. Renaming needs an admin account like the other replication mutations and is audited as `RENAME`. Unnamed replications keep working; the console shows `<from> → <to> · <bucket|all buckets>` for them.

```bash
curl -X POST http://localhost:9671/replication/add -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":{"fromStorage":"minio1","toStorage":"minio2"},"name":"prod to dr"}'
curl -X PUT http://localhost:9671/replication/rename -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":{"fromStorage":"minio1","toStorage":"minio2"},"name":"prod to dr (nightly)"}'
```

With the PostgreSQL control plane the name is stored in `replication_status.name` (migration `0004_replication_name.sql`) and projected to the Redis status hash field `name`. `tuttictl repl add --name`, `tuttictl repl rename --name` and the `NAME` column of `tuttictl repl` use it.

**Copy only recent objects (`since`).** `POST /replication/add` accepts `opts.since`, an RFC 3339 timestamp (like rclone `--max-age`, but absolute): the initial copy only copies source objects whose `LastModified` is at or after it, instead of the whole bucket. Clients turn a relative input such as "30 days" into an absolute time before sending; the server stores only the absolute value, so it does not drift. `get`, `list` and the stream return it as `since` (absent when the whole source is copied). It must not be in the future and not earlier than year 2000 (otherwise `400`). It is not part of the replication id, the scope lock or the overlap rule, and replications without it behave exactly as before.

```bash
curl -X POST http://localhost:9671/replication/add -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":{"fromStorage":"minio1","toStorage":"minio2","fromBucket":"b","toBucket":"b"},"opts":{"since":"2026-10-17T00:00:00Z"}}'
```

Semantics:

- `LastModified` is the time of the object on the SOURCE storage. Objects older than the cutoff are skipped at listing time: no task is queued, no HEAD request is made, and they do not count in the object or byte totals, progress or ETA (those cover only the objects that pass the filter).
- Objects older than the cutoff are never copied by this replication and never deleted at the destination. Objects already at the destination are skipped by the usual ETag / source-ETag check.
- Live events (proxy, webhook, S3 notifications) are not filtered: new objects are always newer than the cutoff, and an overwrite or delete of an old object still replicates.
- Versioned buckets: a key is listed only if its latest version passes; each of its versions (and delete markers) is then kept only if its own `LastModified` is at or after the cutoff.
- Diff reports and diff-fix copies are separate from the replication and are not filtered.
- The cutoff cannot be changed after creation (there is no update call). To move it, delete the replication and create it again with the new value: objects already at the destination are skipped, so only the missing ones are copied.
- With the PostgreSQL control plane it is stored in `replication_status.since` (nullable, migration `0005_replication_since.sql`) and projected to the Redis status hash field `since`.

**Periodic rescan (`opts.rescanInterval`, `PUT /replication/rescan`, `PUT /replication/rescan-interval`).** Tutti only sees objects that go through its proxy. When something writes directly into the source S3, set a rescan interval: the worker then lists the source bucket again every N and copies the objects that are newer than the last successful scan and missing at the destination. Works with any S3 source; nothing changes on the writer side. See [Pick up files written directly to the source](./operations.md#pick-up-files-written-directly-to-the-source) for the semantics and limits.

- `POST /replication/add` accepts `opts.rescanInterval`, a protobuf Duration whose JSON form is a string such as `"3600s"`. Unset or `0` means off (the default). Allowed range is 15 minutes to 24 hours, anything else (including negative values) is `400`. The source storage must be S3. It is not part of the replication id, the scope lock or the overlap rule.
- `PUT /replication/rescan-interval` with `{"id":{...},"rescanInterval":"900s"}` changes it later; unset or `"0s"` turns it off, same validation. Turning it on for a replication that had it off also requests one catch-up scan of the whole source (or from `since`), because objects written directly in the meantime are unknown.
- `PUT /replication/rescan` with the replication id as body ("Sync now") starts a scan immediately, whether or not the interval is set. It fails with gRPC `FailedPrecondition` (HTTP `400` through the gateway, reason in `details`) when a scan is already running, the replication is paused or its initial copy is not finished, and with `NotFound` for an unknown replication.
- Both calls need an administrator (they are not on the viewer allow-list). With the PostgreSQL control plane they are audited as `RESCAN_INTERVAL` and `RESCAN`.
- `get`, `list` and the stream return `rescanInterval` (absent when off), `lastRescanAt` (START time of the last completed scan), `nextRescanAt` (absent when rescan is off, the replication is paused, its initial copy is not done or a scan is running), `rescanRunning`, `lastRescanObjects` (objects the last completed scan actually copied; int64 values are JSON strings) and `lastRescanError` (empty when the last scan succeeded). A scan never changes `isInitDone`, `initObjListed`, `initObjDone` or the init byte counters.
- With the PostgreSQL control plane the interval is stored in `replication_status.rescan_seconds` (nullable integer seconds, migration `0006_replication_rescan.sql`) and projected to the Redis status hash field `rescan_seconds`. Scan state is Redis runtime data (below), not policy data.
- Copies of a scan appear in `POST /replication/activity` with `flow` `"rescan"`.
- Prometheus (worker): `tutti_rescan_started_total`, `tutti_rescan_scans_total{result="success|failed_copies|aborted"}`, `tutti_rescan_objects_copied_total`, `tutti_rescan_last_duration_seconds`.

```bash
curl -X PUT http://localhost:9671/replication/rescan-interval -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":{"fromStorage":"minio1","toStorage":"minio2","fromBucket":"b","toBucket":"b"},"rescanInterval":"3600s"}'
curl -X PUT http://localhost:9671/replication/rescan -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"fromStorage":"minio1","toStorage":"minio2","fromBucket":"b","toBucket":"b"}'
```

**Overlap rule and 409.** Creating a replication, creating a group and updating a group all use the same rule:

| Case | Example | Result |
|---|---|---|
| Same destination | `A -> C` exists, add `B -> C` (or `A -> C` again) | rejected |
| Reverse direction | `A -> B` exists, add `B -> A` | rejected |
| Different source storage (same user) | `A -> B` on bucket `b1` exists, add `C -> D` on `b1`; or `A -> B` and `C -> D` both all-buckets | rejected: policy stores keep one source per bucket |
| Same source, different destination | `A -> B` exists, add `A -> C` | allowed |
| Chain, unrelated pairs | `A -> B` and `B -> C` (or `C -> D`) with disjoint buckets, or one all-buckets and one explicit list | allowed |

The different-source row applies only to two objects of the same user (both storage-level with an empty user, or the same real user; different real users stay independent) whose explicit bucket lists intersect or that are both all-buckets (a bucket policy takes precedence over an all-buckets one in the proxy, so all-vs-explicit is fine); a chain on the same bucket is therefore also rejected. The error text is `storage-level replications must share one source storage per bucket: bucket "b1" is already replicated from "A" by <kind> "<id>"` (`replications of user "alice" must share ...` for a real user). Other rejections apply only when the bucket sets also intersect: an explicit bucket list intersects when at least one name is shared, and "all buckets" intersects everything. Objects of two different real users conflict only when their explicit bucket lists intersect, because "all buckets" of a user means all buckets of that user. A storage-level object (no user) covers every bucket of the storage, so it conflicts with any object on the same edges. A protection group contributes every ordered pair between its members. The check runs before anything is created, under one global lock, so a rejected request leaves no policy, key, group or queue entry behind and two concurrent creations cannot both succeed.

A rejected request returns HTTP `409` (gRPC `AlreadyExists`); the human text naming the conflicting object and bucket is in the error details `reason`. `tuttictl` prints that text and exits non-zero. One source to many destinations is expressed as several replications (`A -> B`, `A -> C`) or as one protection group.

**Data behavior** (measured): an object with the same name but different content is overwritten at the destination by the source version; objects that exist only on the destination are never touched; identical objects are skipped.

---

## 5. S3 Users & Access Policies

Decoupled identity management. S3 users sign in to the Tutti S3 Proxy. Their requests are signed toward member storages using the storage's **connection account** or an explicit per-storage **override**.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/tutti/users` | List all S3 users, policies, and accessible storages |
| `POST` | `/tutti/users` | Create an S3 user identity with initial keys & policy |
| `PUT` | `/tutti/users/credentials` | Set per-storage credential override |
| `PUT` | `/tutti/users/credentials/delete` | Remove credential override |
| `PUT` | `/tutti/users/delete` | Safely remove an S3 user, keys, and policies |
| `POST` | `/tutti/users/test` | Test connectivity for user credentials |
| `GET` | `/tutti/access-policies` | List access policies |
| `POST` | `/tutti/access-policies` | Set access policy (permission & bucket scopes) |
| `PUT` | `/tutti/access-policies/delete` | Remove access policy |

### Example: Create S3 User with Bucket Scope
```bash
curl -X POST http://localhost:9671/tutti/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "data-ingest",
    "createAccessKey": true,
    "description": "ETL ingest service account",
    "policy": {
      "permission": "read-write",
      "buckets": ["analytics-raw", "incoming-*"]
    }
  }'
```
**Response**:
```json
{
  "user": {
    "name": "data-ingest",
    "complete": true,
    "policy": {
      "hasPolicy": true,
      "permission": "read-write",
      "buckets": ["analytics-raw", "incoming-*"]
    }
  },
  "accessKey": {
    "accessKeyId": "TUT9A8B7C6D5E4F3G2H1",
    "secretAccessKey": "Kj9#mQ2$pL8vWx7zYt3rBn1sDf6hJ0kL"
  }
}
```

---

## 6. Dynamic Proxy Access Keys

Proxy access keys authenticate S3 client requests (AWS CLI, SDKs, `mc`) against the Tutti Proxy.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/tutti/proxy-accounts` | List all proxy access keys |
| `POST` | `/tutti/proxy-accounts` | Generate a new access key and secret for a user |
| `PUT` | `/tutti/proxy-accounts/rotate` | Rotate secret with an active overlap period |
| `PUT` | `/tutti/proxy-accounts/status` | Enable or disable an access key |
| `PUT` | `/tutti/proxy-accounts/delete` | Permanently revoke an access key |

---

## 7. Proxy Fleet, Monitoring & Circuit Breakers

Enables real-time fleet health observation, circuit breaker status, and traffic metrics.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/tutti/proxies` | List all running proxies, circuit breaker states, and rates |
| `GET` | `/tutti/stats/history?range=15m` | Time-series traffic metrics (`5m`, `15m`, `1h`, `6h`) |
| `GET` | `/tutti/worker-stats` | Replication throughput of the live workers (see below) |
| `GET` | `/tutti/proxy-config` | Cluster-wide proxy fleet configuration |
| `PUT` | `/tutti/proxy-config` | Update cluster-wide proxy configuration |

### Example: Query Live Proxy Health & Circuit Breakers
```bash
curl http://localhost:9671/tutti/proxies -H "Authorization: Bearer $TOKEN"
```
**Response**:
```json
{
  "proxies": [
    {
      "appId": "proxy-us-east-1",
      "address": "http://10.0.1.10:9669",
      "startedAt": "2026-11-11T08:00:00Z",
      "updatedAt": "2026-11-11T12:00:02Z",
      "storages": [
        {
          "name": "minio1",
          "available": true,
          "consecutiveFailures": 0,
          "lastError": ""
        },
        {
          "name": "minio2",
          "available": false,
          "consecutiveFailures": 4,
          "lastError": "dial tcp 10.0.2.15:9000: i/o timeout"
        }
      ],
      "rates": {
        "requestsPerSec": 240.5,
        "latencyAvgMs": 8.4,
        "latencyP95Ms": 32.1,
        "bytesInPerSec": 10485760,
        "bytesOutPerSec": 52428800
      }
    }
  ]
}
```

---

## 8. Audit Logging API

Retrieves immutable control-plane and security audit logs stored in PostgreSQL.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/tutti/audit-log` | Query paginated audit events with filters |

**Query Parameters**:
- `limit`: Number of entries to return (default: `50`, max: `500`).
- `offset`: Pagination offset.
- `action`: Filter by action (e.g. `USER_CREATE`, `STORAGE_ROTATE_KEY`, `SWITCHOVER`).
- `user`: Filter by operator username.
- `since`: RFC 3339 timestamp.

```bash
curl "http://localhost:9671/tutti/audit-log?limit=10&action=SWITCHOVER" \
  -H "Authorization: Bearer $TOKEN"
```


## Worker throughput and tuning

`GET /tutti/worker-stats` (read-only roles allowed) returns the snapshots that each worker publishes to Redis every 5 s (key `tutti:worker:stats:<appID>`, TTL 20 s; older snapshots are ignored) and their sums:

```json
{
  "workers": [{ "workerId": "w1", "startedAt": "...", "updatedAt": "...", "concurrency": 10, "bulkConcurrency": 2, "liveConcurrency": 8,
    "inFlight": 3, "copiedObjectsTotal": 120, "copiedBytesTotal": 5000000, "failedTotal": 1,
    "objectsPerSec1m": 1.5, "bytesPerSec1m": 100000, "objectsPerSec5m": 1.2, "bytesPerSec5m": 90000, "avgCopySeconds1m": 2.1,
    "bufferTotalBytes": 268435456, "bufferUsedBytes": 33554432, "bufferWaiters": 0, "bufferWaitSecondsTotal": 0.4 }],
  "totals": { "inFlight": 3, "concurrency": 10, "objectsPerSec1m": 1.5, "bytesPerSec1m": 100000, "objectsPerSec5m": 1.2, "bytesPerSec5m": 90000,
    "bufferTotalBytes": 268435456, "bufferUsedBytes": 33554432, "bufferWaiters": 0,
    "copiedObjectsTotal": 120, "copiedBytesTotal": 5000000, "failedTotal": 1 }
}
```

`GET/PUT /tutti/system-settings` `worker` block gained `bulkConcurrencyPercent` (0 or 5..95; 0 = auto; worker restart), `multipartThreads` (0 or 1..16; 0 = auto, 1; live) and `multipartPartSizeMib` (0 or 5..512; 0 = auto, 16; live) and `copyEngine` (`""`, `legacy` or `pipeline`; live), `resumeMultipart` (bool, default false; live) and `largeObjectStreams` (0 or 1..64; 0 = auto, 2 per CPU clamped to 4..16; live; concurrent copies of objects >= 128 MiB per destination storage). Out-of-range values return `InvalidArgument`; an omitted field keeps the stored value and an explicit `0` resets that knob to auto (`bandwidthLimitBytes`: unlimited). `concurrency` and `bufferBudgetBytes` also accept `0` = auto (derived from the detected hardware); a non-zero `bufferBudgetBytes` below 16 MiB is rejected and the budget is never unlimited. A partial `worker` body such as `{"worker":{"multipartThreads":4}}` changes only that field.

The response additionally carries read-only fields describing the worker process that answered: `hardware` (`cpu`, `cpuFractional`, `memoryBytes`, `source` = `host|cgroup-v1|cgroup-v2`, `memoryLimited`), `recommended` and `effective` (worker settings: `concurrency`, `bulkConcurrencyPercent`, `bufferBudgetBytes`, `multipartThreads`, `multipartPartSizeMib`, `largeObjectStreams`) and `tuningSources` (the same six names, each `auto` or `explicit`). See [Hardware-aware defaults](./configuration.md#hardware-aware-defaults).
