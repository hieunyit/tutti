# Plan: many storages, many proxies, multi-step failover

Status: proposal (2026-11-10). Goal: run Tutti with N storages, a fleet of proxies and repeated failovers
without manual per-user/per-storage work and without losing copies after a failover.

## 0. Where we are and what blocks scale

| Area | Today | Problem at scale |
|---|---|---|
| Credentials | one key pair per (user, storage), created by hand on the storage first | users × storages key pairs; adding a storage or user means touching every pair |
| Replication | one replication per (user or bucket, source, target) | failover only works where a replication was created in advance |
| Failover | moves routing to the first in-sync target, pauses replication from the failed storage | after failover nothing replicates from the new primary: copy count drops to 1, second failover needs pre-made policies and replications, failback is manual |
| Proxies | stateless, share Redis; health and some settings come from each proxy's YAML | settings drift between proxies; each proxy decides "storage down" alone (a partitioned proxy can trigger failover) |
| State | everything in Redis | see section 1 |

## 1. Is keeping everything in Redis OK?

Not for everything. Redis holds three kinds of state today, and they need different guarantees:

| Kind | Examples | Needs | Verdict |
|---|---|---|---|
| Control-plane config (source of truth) | storages, users, proxy keys, access policies, routing, replication and failover policies, failover events, console users | durability, transactions across objects, audit history, backups, schema migrations | **Move out of Redis** (PostgreSQL), keep Redis only as a cache/notification bus |
| Coordination and queues | object locks, asynq replication queues, version vectors, failover lock | low latency, atomic ops | **Keep in Redis**, made highly available |
| Ephemeral / derived | sessions, proxy status, stats history, health snapshots | speed, can be rebuilt | **Keep in Redis** (TTL, may be lost) |

Why the config should not live only in Redis:
- AOF `everysec` can lose the last second; a lost routing or failover event during an incident is the worst case.
- No cross-object transactions (we rely on Lua and WATCH per key; e.g. "delete storage if unreferenced" has a race).
- No history/audit of who changed routing or credentials.
- Memory-bound: per-object metadata and queues grow with data, and config shares that memory (with `noeviction`, a full Redis also blocks config changes).
- One Redis outage stops writes on every proxy.

Redis is still the right tool for locks, queues and fast shared state, as long as it is HA.

## 2. Phases

### Phase 1: Foundations (low risk, do first)
1. Redis HA: Sentinel (or managed Redis with failover), AOF + periodic RDB snapshots, tested restore (extend `tools/redis`), memory alerts, separate logical DBs or instances for config vs queues/metadata.
2. Proxies keep a **last-known-good config snapshot in memory** (routing, accounts, policies, storages): a Redis blip must not break reads or routing; writes that need the event log still fail closed (503) as today.
3. Readiness endpoint per proxy (`/ready`: config loaded, Redis reachable, storages known) for the load balancer; graceful drain on shutdown.

### Phase 2: Storage connection accounts (fixes the credentials matrix)

#### 2.1 Model (backend done)
1. Each storage gets one **connection account** (a dedicated S3 user on that storage, e.g. `tutti-svc`), stored sealed
   with the master key. Proxy, worker (replication, diff, migration) and health checks use it by default.
2. Tutti users become **identities + permissions** only: name, access policy (permission + bucket globs), proxy access keys.
   Creating a user no longer asks for storage keys.
3. Optional **per-user override** on a storage (advanced: tenant isolation, quota or billing per user on the storage side):
   if set, that user's traffic to that storage is signed with the override instead of the connection account. This is today's model.
4. Credential resolution for a request of user U to storage S: override(U, S) if present → connection account of S → error
   `storage S has no credentials for user U` (503, logged, counted in stats).
5. Migration: config-defined per-user credentials become overrides and keep working; a storage without a connection account
   keeps the current per-user behaviour; the UI marks it "legacy credentials".
6. Isolation: default deny for shared-account users (done). A user on a connection account without an access policy gets
   403 / empty bucket list; users and user-level replications of such users require and are scoped by the policy globs
   (`auth.defaultPolicy`, see storage-accounts.md).

#### 2.2 Request flow (client never sees storage credentials) (backend done)
```
client --SigV4 with Tutti key (TUT…)--> proxy
proxy: 1) find key (static / dynamic), reject disabled or unknown (403 InvalidAccessKeyId)
       2) verify signature (header, presigned, streaming)
       3) resolve user U, check access policy (permission class + bucket glob) → 403 AccessDenied
       4) route: storage S = routing(U, bucket)
       5) re-sign with credentials of (U, S) per 2.1.4 and forward
```
Consequences: revoking a client = disable/rotate its Tutti key; adding a storage or failing over never changes client config.
`auth.useStorage` (clients using storage keys directly) stays only for backward compatibility and is not offered in the UI.

#### 2.3 Permissions the connection account needs (backend done)
Every member of a protection group can become primary after failover, so **every storage needs read and write**.
Required set depends on enabled features:

| Needed for | S3 actions | Required when |
|---|---|---|
| Health check, list buckets, user-level replication | `s3:ListAllMyBuckets`, `s3:GetBucketLocation` | always |
| Bucket listing, replication scans | `s3:ListBucket`, `s3:ListBucketVersions` (if versioning) | always |
| Client bucket create/delete, replication creates target buckets | `s3:CreateBucket`, `s3:DeleteBucket` | always (unless buckets are pre-created; then the check marks it "not allowed: bucket creation through Tutti disabled") |
| Object read/write/delete, replication | `s3:GetObject`, `s3:PutObject`, `s3:DeleteObject` | always |
| Versioned objects | `s3:GetObjectVersion`, `s3:DeleteObjectVersion`, `s3:GetBucketVersioning`, `s3:PutBucketVersioning` | storage provider supports versioning and versioned buckets are used |
| Multipart uploads | `s3:ListBucketMultipartUploads`, `s3:ListMultipartUploadParts`, `s3:AbortMultipartUpload` (Put covers parts) | always |
| Tag sync | `s3:GetObjectTagging`, `s3:PutObjectTagging`, `s3:GetBucketTagging`, `s3:PutBucketTagging` | `features.tagging` |
| ACL sync | `s3:GetObjectAcl`, `s3:PutObjectAcl`, `s3:GetBucketAcl`, `s3:PutBucketAcl` | `features.acl` |
| Lifecycle sync | `s3:GetLifecycleConfiguration`, `s3:PutLifecycleConfiguration` | `features.lifecycle` |
| Bucket policy sync | `s3:GetBucketPolicy`, `s3:PutBucketPolicy`, `s3:DeleteBucketPolicy` | `features.policy` |

Scope: `Resource` may be limited to a bucket prefix (e.g. `arn:aws:s3:::app-*` and `arn:aws:s3:::app-*/*`) when Tutti manages
only part of the storage; the access policies of Tutti users must then stay inside that prefix (the UI warns otherwise).
Never use the root account (`minioadmin`, Ceph admin users): the check in 2.4 flags it as a warning.

MinIO example (`tutti-rw.json`, all features, whole storage):
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["s3:ListAllMyBuckets", "s3:GetBucketLocation"], "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:CreateBucket", "s3:DeleteBucket", "s3:ListBucket", "s3:ListBucketVersions",
                "s3:ListBucketMultipartUploads", "s3:GetBucketVersioning", "s3:PutBucketVersioning",
                "s3:GetBucketTagging", "s3:PutBucketTagging", "s3:GetBucketAcl", "s3:PutBucketAcl",
                "s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration",
                "s3:GetBucketPolicy", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy"],
     "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion",
                "s3:ListMultipartUploadParts", "s3:AbortMultipartUpload",
                "s3:GetObjectTagging", "s3:PutObjectTagging", "s3:GetObjectAcl", "s3:PutObjectAcl"],
     "Resource": ["arn:aws:s3:::*/*"]}
  ]
}
```
```bash
mc admin user add myminio tutti-svc '<secret>'
mc admin policy create myminio tutti-rw tutti-rw.json
mc admin policy attach myminio tutti-rw --user tutti-svc
```
Ceph RGW: `radosgw-admin user create --uid=tutti-svc --display-name="Tutti"`; buckets created through Tutti are owned by it;
existing buckets of other owners need a bucket policy granting `tutti-svc` the actions above, or
`radosgw-admin bucket link --bucket=<b> --uid=tutti-svc` (changes ownership: confirm with the storage owner).
The docs page ships these files (`docs/tutti/storage-accounts.md`) and the UI offers "Copy policy" for the enabled features.

#### 2.4 "Test connection": exact checks (on add, on edit, and on demand) (backend done)
Run in order with the entered settings and connection account; each step reports **pass / fail / warning / skipped**, the API
call, latency and, on failure, the exact S3 error code and the missing permission. A fail blocks saving unless the admin
ticks "save anyway" (recorded in the audit log). All probes use a temporary bucket `tutti-probe-<random>` (or, if bucket
creation is not allowed, the bucket the admin selects) and clean up afterwards; cleanup failures are reported.

| # | Check | Call | Pass when | On fail show |
|---|---|---|---|---|
| 1 | DNS + TCP | resolve host, connect to port | connected within `httpTimeout` | "Cannot reach host:port (DNS / refused / timeout)" |
| 2 | TLS (if https) | TLS handshake | valid chain for the host name; expiry > 14 days (else warning) | certificate error text, expiry date, "custom CA required" hint |
| 3 | S3 endpoint | unauthenticated `GET /` | any S3 XML error/response (not HTML, not a redirect) | "Endpoint does not look like S3 (got <status>/<content-type>)" |
| 4 | Clock skew | `Date` header of the response | |skew| < 5 min (warning > 1 min) | "Clock differs by Xs: SigV4 will fail" |
| 5 | Authentication | `ListBuckets` signed with the account | 200 | `InvalidAccessKeyId` / `SignatureDoesNotMatch` / `AccessDenied` mapped to a sentence |
| 6 | Root account | compare owner/display name, provider admin API if known | not the root/admin user | warning "Use a dedicated account" |
| 7 | Region | `GetBucketLocation` on probe or an existing bucket | matches `defaultRegion` (or storage reports none) | "Storage reports region X" with a one-click fix |
| 8 | Create bucket | `CreateBucket tutti-probe-…` | 200 | missing `s3:CreateBucket`; if intentionally disabled, mark skipped and ask for an existing bucket |
| 9 | Write | `PutObject probe/obj` (small) | 200 | missing `s3:PutObject` |
| 10 | Read | `GetObject` + `HeadObject` | content matches | missing `s3:GetObject` |
| 11 | List | `ListObjectsV2` on the probe bucket | object listed | missing `s3:ListBucket` |
| 12 | Multipart | create upload, put 1 part (5 MiB), list parts, abort | all succeed | missing multipart action named by the failing call |
| 13 | Versioning (if provider supports it and versioned buckets are in use) | `GetBucketVersioning`, `PutBucketVersioning Enabled` on probe, write 2 versions, `ListObjectVersions`, delete a version | all succeed | missing versioning action; "provider does not support versioning" if `NotImplemented` |
| 14 | Feature checks (only enabled features) | tagging / ACL / lifecycle / policy Get+Put on the probe bucket/object | all succeed | missing permission per feature; option to disable the feature for this storage |
| 15 | Delete | `DeleteObject`, `DeleteBucket` of the probe | 200/204 | missing `s3:DeleteObject` / `s3:DeleteBucket`; probe left behind is listed |
| 16 | Existing buckets | for each bucket referenced by Tutti users' policies, routings or protection groups on this storage: `HeadBucket` + `ListObjectsV2 max-keys=1` | accessible | "Account cannot access bucket B (owned by another account): add a bucket policy" with a copyable policy snippet |
| 17 | Throughput sanity (optional, on demand) | 3 × PUT/GET of 1 MiB | report MiB/s and p95 | warning only |

Results are stored with the storage (last check time, per-step outcome) and shown on the Storage detail Settings tab;
the health checker keeps re-running steps 1–5 periodically and alerts when one starts failing (e.g. the account was
disabled or its key rotated on the storage side).

#### 2.5 UI changes
- Add storage: Connection (endpoint, provider, region, TLS, timeouts) → Connection account (access key, secret, "Copy policy"
  for the enabled features) → Test (the table above, live, with retry per step) → Save.
- Storage detail → Settings: same form, "Rotate connection account" (enter the new key, test, switch atomically, old key
  kept until all proxies confirm), last test results.
- Create user: name, permission, buckets, first access key. No storage keys. Advanced section "Per-storage credentials
  override" collapsed by default.
- Users list: column "Credentials" shows `connection account` or `override on <storages>`.

#### 2.6 Later
Auto-provision per-user accounts through the MinIO admin API / Ceph RGW admin ops for storages that support it
(needs an admin account per storage; opt-in).

### Phase 3: Protection groups (replaces hand-made replication + failover pairs)
A **protection group** = scope (user, bucket list or all buckets) + ordered set of storages + `copies` wanted + `maxEventLag` + failover mode (auto/manual).
1. Tutti keeps a **primary** (where routing points) and creates/maintains replications from the primary to every other healthy member automatically.
2. **Failover** = pick the first in-sync member (lag ≤ maxEventLag), switch routing, then **re-point replications from the new primary to the remaining members**, so the copy count is restored instead of dropping to 1.
3. **Repeated failover** works without extra policies: the group always knows its members.
4. **Failback / switchover** (planned, from the UI): resync the recovered storage from the current primary (reverse replication), wait until the replication's events are done (`EVENTS` done == total in `chorctl repl`), short write freeze, switch routing, restore the original direction.
5. Group status in UI: primary, members with lag, copies healthy vs wanted, RPO estimate.
6. Existing replications and failover policies keep working; a migration wizard can turn them into groups.

### Phase 4: Failover safety across many proxies
1. **Quorum on "storage down"**: a storage counts as down for failover only when a majority of reporting proxies (and/or the worker's own probe) see it down for `after`; one partitioned proxy cannot trigger failover.
2. **Fencing**: every routing switch increments a generation; proxies reject writes routed with an older generation, so a proxy with a stale view cannot keep writing to the old primary.
3. Rate limits on automatic failover (max N switches per group per hour) and a global "freeze automatic failover" switch for incidents.
4. Chaos tests: partition one proxy, kill primary during load, double failure, Redis failover during a switch.

### Phase 5: Fleet configuration for proxies (Done)
1. Move proxy behaviour settings (failover enabled, sync replication + quorum, health thresholds, readiness/drain timings, and the single auth option `requireSignedContentType`; other auth settings stay in YAML) from YAML to central config (`tutti:proxy-config`) with **proxy groups** (`group` in proxy YAML) and per-group overrides; YAML keeps bootstrap settings and acts as the baseline when no central override is set.
2. Proxies apply changes live without restart (`service/proxy/server.go`, `pkg/health/breaker.go`, `service/proxy/failover/failover.go`, `service/proxy/router/server.go`, `service/proxy/auth/middleware.go`, `service/proxy/ready.go`); non-hot-reloadable settings (`readHeaderTimeout`, `protectionGroups.writerLease`) report `pendingRestart: true` with `pendingRestartReasons`.
3. Safety: updates are optimistic (`expected_version`, `FailedPrecondition` on a stale version), audited (actor, before/after: structured log line plus the capped Redis list `tutti:proxy-config:audit`) and validated with the same validators as the YAML config; a proxy that receives an invalid document or invalid effective settings keeps its last good configuration and reports `configError` in its status.
4. The UI `Proxies` page shows each proxy's group, applied config version vs cluster version, pending restart status, and a Fleet Configuration editor; covered by E2E test with two proxies in two groups (`test/resilience/proxy_fleet_config_test.go`).

### Phase 6: Control-plane database (Done)
1. PostgreSQL for control-plane config (storages, users, keys metadata, policies, routing, replications, protection group specs, failover policies/events, console users, storage tests) with embedded migrations (`pkg/controlplane/pg`) and `Store.Tx` multi-object transactions; secrets stay sealed with the master key.
2. **Audit log** table (who, request id, what, before/after with secrets redacted as `***`) written atomically in the same transaction; exposed via `Resilience.ListAuditLog` and the Console **Audit log** page.
3. Change propagation: transactional outbox (`pg.Publisher`) projects every change into Redis keys and `tutti:cfg:version` / `tutti:cfg:changed` pub/sub so proxies keep reading Redis unchanged; exports `controlplane_outbox_*` metrics.
4. Migration & recovery CLI `tutti-migrate-pg` (`schema | migrate | verify | republish`), plus read-only fallback to the Redis projection when PostgreSQL is unreachable (with 503 on mutations). See [control-plane-db.md](./control-plane-db.md).

### Phase 7: Operations
Alerts (storage down, copies below wanted, lag over RPO, Redis memory, proxy drift/version skew), runbooks for failover/failback, backup/restore drill for both PostgreSQL and Redis, capacity guide.

## 3. Order and rough size

| Phase | Depends on | Size | Value |
|---|---|---|---|
| 1 Foundations | – | S | survives Redis blips, LB-ready |
| 2 Connection accounts | – | M | removes the credentials matrix |
| 3 Protection groups | 2 (simpler with one account per storage) | L | real multi-storage failover with copies kept |
| 4 Failover safety | 3 | M | safe with many proxies |
| 5 Fleet config | 1 | M | no drift between proxies |
| 6 Control-plane DB | 1 | L | durability, audit, transactions |
| 7 Operations | all | S–M | production readiness |

Recommended sequence: 1 → 2 → 3 → 4, then 5 and 6 in parallel, 7 continuously.

## 4. External review (2026-11-10): verified findings

Each item was checked against `main`. Items only present on upstream branches are listed but not planned.

| ID | Finding | Verdict on main | Action |
|---|---|---|---|
| VULN-01 | per-username failure budget (`userFailKey`, 100 / 15 min) is never reset and blocks even correct passwords → anyone can lock out `admin` | **Confirmed** (`pkg/auth/service.go`) | **Done** — the per-user budget no longer blocks (user, IP) pairs with a recent successful login; correct logins from known IPs still pass; progressive delay instead of a hard block for unknown IPs; success clears the pair budget |
| VULN-02 | `/chorus.Webhook/*` is public; on a shared port anyone reaching the API can inject fake bucket events | **Confirmed** (`pkg/auth/interceptor.go`) | **Done** — optional `api.webhook.token` (Bearer / `X-Tutti-Webhook-Token`, constant-time compare); startup warning + UI warning when webhooks share the main port without a token |
| VULN-03 | lower-case `authorization` / `x-amz-date` not stripped when forwarding | **Not an issue**: `net/http` canonicalises incoming header names before `processHeaders` | Optional hardening with `strings.EqualFold` |
| VULN-04 | `ExtractReqBody` reads XML bodies without a limit | **Confirmed** (DeleteObjects, bucket XML) | **Done** — `http.MaxBytesReader` (S3 limit: 1000 keys ≈ 2 MiB; use 4 MiB), 400 `MalformedXML`/`EntityTooLarge` |
| VULN-05 | Argon2 parallelism from `runtime.NumCPU()` | **Not on main** (fixed parallelism 4); upstream `feat/proxy-alias-credentials` only | Do not merge that branch as is |
| LOGIC-01 | `WriteHeader(500)` after the body started | **Confirmed**, harmless log but the client may not notice truncation | **Done** —  abort the connection (`panic(http.ErrAbortHandler)`) so the client sees a broken response |
| LOGIC-02 | response headers copied with `Set(k, v[0])`, hop-by-hop headers not removed | **Confirmed** | **Done** —  copy all values, drop hop-by-hop headers (RFC 7230) |
| LOGIC-03 | lock refresh failure only logged; work continues after the lock may be lost | **Confirmed** (`pkg/store/lock.go`) | **Done** —  on refresh failure / lock lost, cancel the work context and return an error so the task retries |
| LOGIC-04 | sync-replication budget timeout (`DeadlineExceeded` of our own context) is counted as a storage failure for every pending destination | **Confirmed** | **Done** —  our own budget timeout is neutral for the breaker; only storage errors count |
| LOGIC-05 | PUT ignores `x-amz-version-id`, worker syncs "latest" | **Confirmed**, inherited from Chorus design | Plan: investigate versioned-bucket replication end to end (capture version id, worker support, ordering); not a quick fix |
| LOGIC-06 | runtime storage delete is check-then-act without a lock | **Confirmed** | **Done** — a Redis lock per storage name taken by storage update/delete and by replication / routing / failover-policy creation for the storages they reference; re-check references inside the lock |
| LOGIC-07, BUG-01 | goroutine leak / wrong error variable in consistency check | **Not on main** (upstream `consistency-check` draft) | Fix before ever merging that branch |
| BUG-02 | storage reference check does 2 Redis calls per user | **Confirmed** | **Done** — pipeline per batch of users |
| BUG-03 | `HKEYS` on the dynamic credentials hash | **Confirmed** (`pkg/objstore/managed.go`) | **Done** — `HSCAN` |
| BUG-04 | ListBuckets filtering fails on gzip-encoded upstream responses | **Plausible** (only if the client sent `Accept-Encoding: gzip` and the storage compresses) | **Done** — drop `Accept-Encoding` on the forwarded ListBuckets when a filter applies |
| PERF-1 | forwarding client uses `http.DefaultTransport` (`MaxIdleConnsPerHost = 2`) | **Confirmed** (`pkg/s3client/client.go`) | **Done** —  one tuned `http.Transport` per storage (MaxIdleConns 1024, MaxIdleConnsPerHost 512, IdleConnTimeout 90 s, TLS session cache), configurable; measure with warp before/after |
| PERF-2 | extra `StatObject` (HEAD) after every PUT | **Confirmed** | **Done** —  use the request size (`Content-Length`, or `x-amz-decoded-content-length` for aws-chunked); HEAD only when unknown |
| PERF-3 | ~8 Redis ops per S3 request in the policy middleware | **Confirmed** (see `performance-and-redis.md`) | Plan, Phase 1: in-memory policy/routing snapshot with versioned invalidation (pub/sub); must keep switch/failover correctness (generation checks) |
| PERF-4 | 4 logical DBs on one Redis; Redis Cluster supports only DB 0 | **Confirmed** | Plan, Phase 1/6: separate instances (config / queues+locks / metadata) and key prefixes instead of DB numbers, so each can be scaled or clustered |
| PERF-5 | synchronous replication latency wall (lock + read + write per destination per PUT) | **Confirmed by design** | Document the trade-off; Phase 3 protection groups use async by default with quorum only where required; consider streaming the client body to destinations directly (tee) instead of re-reading from the primary |

## 5. Review of protection groups (2026-11-10)

External static review of the Phase 3 / Phase 4 implementation (`pkg/protectiongroups`, see
[Protection groups](./protection-groups.md)), verified against the code, then fixed.

| ID | Finding | Verdict | Status |
|---|---|---|---|
| NEW-01 | admission Lua script runs `HVALS tutti:protection:groups` and decodes every group on every proxy write | **Confirmed** (`fence.go` admit script): O(groups) Redis CPU per write, and Lua blocks Redis for its duration | **Done** — scope index `tutti:protection:scope` (`u:<user>` / `b:<user>/<bucket>` → id) rebuilt in the same transaction as group create/delete; admission = one HMGET (+ HGET of that one group, HSET writer record, SET lease when protected). Missing index is rebuilt before deciding (never admits unfenced). Test counts commands per admit: constant for 1/50/500 groups |
| NEW-02 | writer records never expire; a crashed proxy blocks promotion forever until `ClearWriter` | **Confirmed** (by design, `fence.go` comment): crashed proxy, or even a live proxy whose release `HDEL` failed once, starved promotion permanently | **Done** — per-process lease (instance nonce, TTL `protectionGroups.writerLease`, default 30s, refreshed every TTL/6); a record is reaped only when its lease expired/was replaced **and** the proxy is absent from fresh status reports (or reports a later restart); self-fencing cancels in-flight protected writes after TTL/2 without refresh; failed releases are retried. Promotion after a crash waits ≤ lease + status expiry + 1 reconcile; alive slow writers still block. `ClearWriter` kept |
| NEW-03 | resync streams and SHA-256-hashes every object on both sides, twice per object, while frozen | **Confirmed** (`resync.go` source loop and destination loop both call `verifyResyncObject`): 4 full reads per object under the write freeze, plus a HEAD per object | **Done** — one merge-join pass over both listings; size → ETag (when it decides) → hash only if inconclusive (multipart layouts), each side read at most once; copies verified from the streamed digest. Plus catch-up before freeze (below) |

Additional findings fixed in the same pass:

| Finding | Impact | Fix |
|---|---|---|
| Groups stayed **frozen during the whole initial migration** after creation and after every promotion (`Create` and `Commit` set `frozen`, and `initializing` waited for full catch-up while frozen) | After a failover, writes were refused until the new primary had re-copied everything to the other members: an outage proportional to data size | `initializing` now unfreezes while members catch up and freezes only for the final drain + resync of caught-up members |
| `Reconcile` error path saved the in-memory group with `LastError`, persisting half-applied transitions (e.g. `ready`/unfrozen when the final `Commit` failed) | Group marked ready without a successful commit | Error path reloads the stored group and records only `LastError`; the final transition is built on a copy |
| Unbounded `history` in the group document | Grows the document decoded by every admission | Keep the newest `max(100, maxSwitchesPerHour)` events |
| Worker tasks of a replication from a demoted primary were retried forever by the fence | Queue noise, retries until max retry | Stale-route refusals are `SkipRetry` (their generation was already bumped) |
| `proto/http.yaml` had a second block of selectors for RPCs that do not exist (`ListProtectionGroups`, …) | `buf generate` fails (`HTTP rules without a matching selector`) | Removed the dangling block |
| Resync could delete target objects based on an out-of-order listing | Replica object loss before the error is detected | Deletions are applied only after both listings were consumed in ascending order |

Follow-up (same day): the fence made every write pay 2–3 Redis commands and return 503 when Lua failed, even
for unprotected scopes (`TestRedisCommandsPerRequest`, `TestEventRecordingFailure_*` failed). **Done** — the
proxy policy snapshot caches the scope index; group create/delete bump `tutti:cfg:version` in the same
transaction, and writes validate the version first. Unprotected writes: 0 fence commands (PUT 0.94 commands
per request in total), no dependency on Lua; protected scopes keep the fail-closed script path.

Verification: unit tests in `pkg/protectiongroups` (fence/index/lease/reaper, controller with fake backend,
resync metadata-first), `-race`; e2e `test/resilience/protection_groups_test.go`: 3 MinIO storages, group
primary A blackholed → automatic failover to B by a three-proxy electorate, C keeps receiving copies; a proxy
partitioned from Redis mid-write → switchover completes ~4s after the partition (lease 3s).

Remaining risks / next steps:
- A frozen group still waits for in-flight writes to a blackholed primary until the storage HTTP timeout; a
  proxy-side abort of protected writes when its group freezes would bound this further.
- Self-fencing cancels the request context; a backend that already received the whole body may still commit
  shortly after. The TTL/2 margin covers normal completion times, not arbitrarily slow storages.
- Workers have no status reports: their lease alone decides reaping.
- No automatic failover while a group is `initializing` (catch-up after a previous promotion).
- Equal MD5-form ETags are trusted as equal content (MD5 collisions crafted by the tenant itself).
