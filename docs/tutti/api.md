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

`GET/PUT /tutti/system-settings` `worker` block gained `bulkConcurrencyPercent` (0 or 5..95, default 20; worker restart), `multipartThreads` (0 or 1..16, default 1; live) and `multipartPartSizeMib` (0 or 5..512, default 16; live). Out-of-range values return `InvalidArgument`; an omitted or `0` field keeps the stored value.
