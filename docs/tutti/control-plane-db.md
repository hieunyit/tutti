# Control-plane database (PostgreSQL), phase 6

Status: **Implemented and wired** (`pkg/controlplane`, `pkg/controlplane/pg`, `pkg/controlplane/migrate`, `tools/tutti-migrate-pg`,
`service/worker`, `pkg/api`, and UI Audit Log page). Plan reference: [scale-plan.md](./scale-plan.md), Phase 6.

Code: `pkg/controlplane` (models, repository interfaces, `controlPlane` config), `pkg/controlplane/pg` (PostgreSQL
implementation, migrations, audit log, outbox, publisher, `EnqueueSnapshot`), `pkg/controlplane/migrate` (Redis to PostgreSQL copy,
verification, and `Republish`), `pkg/controlplane/pgtest` (test PostgreSQL), `tools/tutti-migrate-pg` (command: `schema | migrate | verify | republish`).

## 1. Inventory: what is stored in Redis today

Redis roles (`pkg/config`): **config** (`redis.configDB`) holds the control plane, **meta** (`redis.metaDB`) holds
version vectors, uploads, sessions and the dynamic credentials hash, **queue** the asynq queues, **lock** the locks.
"Writers" below are the code that mutates the object; every consumer polls or reads Redis directly.

### 1.1 Config (source of truth moves to PostgreSQL)

| Object | Redis key(s) | Go store (file) | Writers | Readers | Consistency need today |
|---|---|---|---|---|---|
| Runtime storages | HASH `tutti:storages` (name -> JSON, credentials sealed), STRING `tutti:storages:version` | `objstore.credsSvc` Create/Update/Delete/RotateConnectionAccount (`pkg/objstore/managed.go`) | worker API | worker + proxy poll the version (<= 2 s), rebuild their storage view | create-if-absent Lua + version INCR in one script; update/delete under the per-storage Redis lock `tutti:storage:lock:<name>`; the reference check on delete is check-then-act (LOGIC-06) |
| Tutti user records (S3 identities) | HASH `tutti:s3users` (user -> `{createdAt}`), STRING `tutti:s3users:version` | `objstore` AddIdentity/RemoveIdentity (`connection.go`) | worker API (`pkg/s3users`) | worker + proxy poll the version | create-if-absent Lua + INCR; delete in MULTI |
| Sealed dynamic credentials | HASH `tutti:dynamic_creds` (**meta role**): fields `version` (counter), `salt` (`<parallelism>:<b64>`), `S3:<storage>:<user>` / `SWIFT:...` (AES-GCM sealed JSON) | `objstore.credsSvc` SetS3Credentials/Delete (`credentials.go`) | worker API | worker + proxy poll `version` | MULTI HSET + HINCRBY; salt HSETNX (the salt must never change, sealed values depend on it) |
| Proxy access keys | HASH `tutti:proxy-accounts` (access key id -> JSON, secrets sealed) | `proxyaccount.Store` (`pkg/proxyaccount/store.go`) | worker API (`resilience_handlers.go`) | proxy `Cache` reloads every 5 s (no version key) | HSETNX create; Lua compare-and-set (`luaCAS`) for rotate / status |
| Access policies | HASH `tutti:access-policies` (user -> JSON) | `accesspolicy.Store` | worker API | proxy `Cache` every 5 s; protection group credential validator | plain HSET/HDEL |
| Console users | HASH `tutti:console-users` (user -> JSON incl. bcrypt hash), HASH `tutti:console-users:last-login` | `auth.UserStore` (`pkg/auth/userstore.go`) | worker API / login | worker auth | HSETNX create; Lua `consoleMutation` = CAS on the old value + "keep one enabled admin" check over the whole hash |
| Routing, user level | HASH `p:route_user:` (user -> storage), SET `p:route_block_user:` | `store.UserRoutingStore` (`routing_stores.go`) | policy service, failover controller, protection group promotion, switch handlers | proxy via the policy snapshot (`tutti:cfg:version`) | MULTI/EXEC with the version bump; `luaUpdateRoute` = update only if present |
| Routing, bucket level | HASH `p:route_bucket:<user>` (bucket -> storage), SET `p:route_block_bucket:<user>` | `store.BucketRoutingStore` | same | same | same |
| Replication policies | user: HASH `p:repl_user_from:` (user -> from), SET `p:repl_user_to:<user>`; bucket: HASH `p:repl_bucket_from:<user>`, SET `p:repl_bucket_to:<user>:<bucket>` (`<toStorage>:<toBucket>`), SET `p:repl_bucket_to_dest_index:<user>` | `store.UserReplicationPolicyStore`, `BucketReplicationPolicyStore` (`replication_stores.go`) | policy service | proxy (snapshot), worker | single source per scope; remove script drops the source when the last destination goes; rules (no user+bucket mix, destination unique) are enforced in Go |
| Replication generations (fencing tokens) | HASH `p:repl_user_gen:<user>` (to -> n), HASH `p:repl_bucket_gen:<user>:<bucket>` (`to:toBucket` -> n) | `store.ReplicationGenerationStore` | policy service (create and delete increment) | proxy (same MULTI as the policy lookup), workers drop stale tasks | never deleted, must be monotonic |
| Replication status, **config fields** | HASH `p:repl_status_user:<user>:<from>:<to>` / `p:repl_status_bucket:<user>:<from>:<to>:<fb>:<tb>`; indexes SET `p:repl_user_status_index:` (JSON members), SET `p:repl_bucket_status_index:<user>` | `store.UserReplicationStatusStore`, `BucketReplicationStatusStore` | policy service | UI, workers | kept after the policy is removed (archived) |
| Failover policies | HASH `tutti:failover-policies` (source -> JSON) | `failover.Store` (`pkg/failover/store.go`) | worker API | proxy failover controller | plain HSET/HDEL |
| Failover events | STRING `tutti:failover:user:<u>`, `tutti:failover:bucket:<u>:<b>` | `failover.Store` | failover controller | UI, failback | plain SET/DEL |
| Protection groups | HASH `tutti:protection:groups` (id -> JSON incl. promotion state), HASH `tutti:protection:scope` (scope index) | `protectiongroups.Store` (read only for this work) | worker API, promotion controller | proxy admission Lua scripts (`fence.go`), controllers | WATCH-based CAS on `Revision`; the index is rebuilt in the same transaction |
| Storage connection tests | HASH `tutti:storage-tests` (storage -> JSON) | `objstore` SaveStorageTest | worker API | UI | plain HSET |

Replication status field split: **config** = `created_at`, `archived`, `archived_at`, `agent_url`, `event_source`
(PostgreSQL). **runtime** = `failover_pending` (a pause owned by an unfinished failover), queue pause state and the
queue statistics (asynq); they stay in Redis. Protection groups mix desired state (members, copies, mode) and
promotion state (primary, generation, state, frozen, history, seeded); they are copied for audit and backup but stay
authoritative in Redis until `pkg/protectiongroups` is split (section 7).

### 1.2 Runtime and coordination (stays in Redis)

| What | Keys | Why it stays |
|---|---|---|
| Locks | `tutti:storage:lock:*`, object / bucket / user / replication-status lockers (lock role) | low latency, TTL semantics |
| Replication queues | asynq keys (queue role) | queue semantics |
| Replication switch state machine | HASH `p:repl_switch_user:<user>`, `p:repl_switch_bucket:<user>:<bucket>`, history lists, SETs `p:repl_user_switch_zd_active`, `p:repl_bucket_switch_zd_active:<user>` | multi-step state driven by workers with locks and queues; its durable outcome (the new route) is written to routing, which is in PostgreSQL |
| Version vectors, uploads, migration progress | `v:*`, upload keys (meta role) | high volume, rebuildable |
| Protection runtime | `tutti:protection:writers:*`, `tutti:protection:lease:*`, `tutti:protection:automatic-frozen` | leases and counters with TTL |
| Proxy status, stats history, health snapshots, sessions, rate limit counters | `proxystatus`, `trafficstats`, `ratelimit`, session keys | ephemeral, may be lost |
| Snapshot version and channel | `tutti:cfg:version`, pub/sub `tutti:cfg:changed` | the notification bus; written by the outbox publisher after this phase |
| Last console login | `tutti:console-users:last-login` | moved to a column but still mirrored (best effort) for rollback |

## 2. Schema

Two migrations (`0001_init.sql`, `0002_outbox_pending_key.sql`: the index the publisher's per-object ordering lookup uses) in
`pkg/controlplane/pg/migrations/`, embedded with `embed`, applied by `pg.Migrate`
(`schema_migrations(version, name, checksum, applied_at)`, one transaction per file, `pg_advisory_xact_lock` against
concurrent migrators, checksum drift and a database newer than the binary are errors).

| Table | Key | Content |
|---|---|---|
| `storages`, `s3_identities`, `access_policies`, `failover_policies`, `failover_events`, `storage_tests` | `k` | `doc jsonb` = the Redis JSON document, `version`, `created_at`, `updated_at` |
| `proxy_accounts` | `k` (access key id) | same + generated `user_name`, `status` |
| `console_users` | `k` | same + generated `role`, column `last_login_at` |
| `protection_groups` | `k` | same + generated `user_name` |
| `dynamic_credentials` | (`storage_type`, `storage`, `user_name`) | `sealed text` exactly as in Redis, `version` |
| `dynamic_credentials_meta` | `name` | the salt |
| `routes`, `route_blocks` | (`user_name`, `bucket`) | `bucket = ''` is user level; `storage`, `version` |
| `replication_policies` | (`user_name`, `from_bucket`, `to_storage`, `to_bucket`) | `from_storage`; `from_bucket = to_bucket = ''` is user level |
| `replication_status` | (`user_name`, `from_storage`, `to_storage`, `from_bucket`, `to_bucket`) | config fields, kept when archived |
| `replication_generations` | (`user_name`, `from_bucket`, `to_storage`, `to_bucket`) | `generation` |
| `audit_log` | `id` | `at`, `actor`, `request_id`, `action`, `object_type`, `object_id`, `before`, `after` (jsonb) |
| `outbox` | `id` | `object_key`, `ops jsonb`, `published_at`, `attempts`, `next_attempt_at`, `last_error` |

Secrets: the documents contain only what Redis contains, i.e. values **sealed** with the dynamic credentials master key
(or bcrypt hashes). PostgreSQL never sees a plaintext secret and the package never opens a sealed value, so the master
password stays outside the database. The salt is copied verbatim; **never** replace it (all sealed values depend on it).

## 3. Package and API layout

- `controlplane.Store` = `Repos()` + `Tx(ctx, func(Repos) error)` + `Close()`. `Repos` has one interface per family:
  `Storages`, `S3Identities`, `DynamicCredentials`, `ProxyAccounts`, `AccessPolicies`, `ConsoleUsers`, `Routing`,
  `Replications`, `FailoverPolicies`, `FailoverEvents`, `ProtectionGroups`, `StorageTests`, `Audit`.
- Document families share `KV[T]`: `Create` (HSETNX / create-if-absent), `Get`, `List` (sorted), `Put` (HSET),
  `Update(v, expectedVersion)` (the version replaces the Lua compare-and-set; `ErrVersionConflict`), `Mutate(key, fn)`
  (row lock, replaces the CAS retry loops of Rotate / SetStatus / consoleMutation), `Delete` (`dom.ErrNotFound`).
- `ConsoleUsers.MutateGuarded` keeps the last-admin check (advisory lock, serialized). `Routing`, `Replications` and
  `DynamicCredentials` mirror the Redis method sets (Set/Update/Block/Unblock/DeleteAllUser, Add/Remove/Archive/
  DeleteStatus/Generation/IncrementGeneration, Set/Get/Delete/Salt).
- Mapping from Redis store methods to repository methods is in the doc comments of `pkg/controlplane/repos.go`.
- Errors are the existing `dom` errors (`ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidArg`, `ErrFailedPrecondition`,
  `ErrRoutingBlock`) plus `controlplane.ErrVersionConflict` and `ErrReadOnly` (both wrap `ErrFailedPrecondition`).
- Context: `controlplane.WithActor(ctx, Actor{Name, RequestID})` is what the audit log records ("system" if absent).
- `pg.Open(ctx, controlplane.Config)` builds a pgx v5 pool (`maxConns`, `statementTimeout`, ...), `pg.Migrate`,
  `Store.SetReadOnly`, `pg.NewPublisher`.

### Audit log

Every mutating call writes one `audit_log` row **in the same transaction** as the change (and the outbox entry):
actor, request id, action (`create`, `update`, `delete`, `set`, `block`, `generation`, ...), object type and id, `before`
and `after` documents with secret fields replaced by `***` (`secret`, `previousSecret`, `connectionAccount`,
`previousAccount`, `passwordHash`, `sealed`, `salt`, ...). A failed or rolled back transaction leaves no audit row.
Re-applying an identical state (migration re-run) writes none. `Audit.List` filters by object, actor, time and pages
by `BeforeID` (for the UI); the `action` filter is applied in SQL, so limit and cursor paginate the filtered rows.
Login time updates and storage connection-test results (`storage_tests`, overwritten on every test) are deliberately
not audited (the latter still projects to Redis). The audit log carries before/after documents of every change, so
`ListAuditLog` is **admin-only**; a database outage returns 503.

## 4. Change propagation (outbox)

Every mutation also inserts an `outbox` row holding **idempotent Redis operations** that project the committed state
into the keys the services already read (section 1.1): hash/set writes in the exact existing formats, the
`tutti:storages:version` / `tutti:s3users:version` counters, the `version` field of `tutti:dynamic_creds`, and for
routing / replication `tutti:cfg:version` + `PUBLISH tutti:cfg:changed` (same script as `pkg/policy/change_notify.go`).
So **proxies and workers keep working unchanged**: they read Redis, which is now a cache of PostgreSQL.

`pg.Publisher.RunOnce/Run` applies the entries after commit, one MULTI/EXEC per entry:

- ordered per object: entries of one `object_key` are applied in id order; a failed head holds back its object only;
- one active publisher (advisory lock), safe to run in every worker;
- idempotent: data operations write absolute state (replication scopes are rebuilt from PostgreSQL, so order across
  objects does not matter); version counters are at-least-once, a repeat only triggers one more reload;
- retried with exponential backoff (`attempts`, `last_error`, `next_attempt_at`); Redis down does not fail the
  control-plane write; `Pending()` reports backlog and age for an alert; `Purge()` drops published rows after the
  retention (24 h default).

Tests prove the projection byte for byte: data written by the real Redis stores, migrated to PostgreSQL and
republished into an empty Redis gives identical keys, and the real stores read the republished data.

## 5. Migration tool

```
tutti-migrate-pg --redis-url redis://:pw@redis:6379/1 [--redis-meta-url redis://:pw@redis:6379/3] \
                 --pg-dsn postgres://tutti:pw@db:5432/tutti?sslmode=verify-full  schema | migrate | verify | republish
```

`--redis-url` is the **config** Redis (`redis.configDB`); `--redis-meta-url` the Redis with `tutti:dynamic_creds`
(`redis.metaDB`; defaults to `--redis-url`). Env: `TUTTI_REDIS_URL`, `TUTTI_REDIS_META_URL`, `TUTTI_PG_DSN`.
Flags: `--dry-run`, `--format text|json`, `--out report.json`, `--timeout`.

- `schema`: apply the SQL migrations.
- `migrate`: read Redis (read commands only), diff against PostgreSQL, write the differences with actor
  `tutti-migrate-pg`. **Idempotent**: identical records are not touched (no audit rows, no outbox), changed ones are
  overwritten with the Redis value, records only in PostgreSQL are kept and reported (`extra`). Imports do not create
  outbox entries (Redis already has the data). `--dry-run` prints the same report without writing (it works even without
  a database). The report has count and SHA-256 per family before and after; sealed values take part in the checksum
  but are never printed. Malformed Redis entries are skipped with a warning.
- `verify`: compares Redis and PostgreSQL both ways; exit code 0 = identical, **2 = mismatch** (the report lists family,
  key and `different` / `missing_in_target` / `missing_in_source`, no values), 1 = error.
- `republish`: rebuilds a lost or empty Redis projection from PostgreSQL. Procedure: (1) wait for the outbox to drain
  (unpublished changes are lag, not drift); (2) compare PostgreSQL and Redis; **refuse (exit 1) when Redis holds
  entries that differ from PostgreSQL or exist only in Redis, unless `--force`**: they may be newer than PostgreSQL
  (for example after restoring PostgreSQL from an old backup); always review `--dry-run` first; (3) enqueue a snapshot
  (`pg.Store.EnqueueSnapshot`, exclusive against in-flight writers, so it cannot be ordered before a change it did not
  see) and, with `--prune`, the removal of Redis-only entries (re-checked against PostgreSQL under the same lock);
  (4) wait until the publisher has applied everything enqueued (`--drain-timeout`, default 5m; a worker's publisher
  holding the lock is waited for, not an error); (5) only then verify. Without `--prune` the republish is **additive**
  (it overwrites and adds, Redis-only entries stay and are reported as a warning) and is not an exact rebuild; with
  `--prune --force` it is. It restores configuration keys, `tutti:dynamic_creds` (meta Redis) and version counters.
  **Protection groups are never republished**: Redis holds their runtime fields (promotion, fencing, scope index) in
  the same documents as the spec, so they are skipped (reported as a warning). Race note: changes written while the
  republish runs may show up as mismatches in the final check; run `verify` again.
- Not migrated by design: `tutti:cfg:version`, version counters, `failover_pending`, switch state, protection runtime,
  console login of deleted users.
- PostgreSQL timestamps have microsecond precision; Redis keeps nanoseconds (comparisons truncate).

## 6. Configuration and planned wiring

```yaml
controlPlane:
  backend: redis            # redis (default) | postgres
  readOnly: false           # postgres only: reject every mutation (read-only fallback)
  postgres:
    dsn: postgres://tutti:secret@db:5432/tutti?sslmode=verify-full
    maxConns: 10            # default 10
    minConns: 0
    connMaxLifetime: 30m
    connMaxIdleTime: 5m
    connectTimeout: 5s
    statementTimeout: 10s   # server side, 0 = server default
    autoMigrate: false      # apply embedded migrations on start; production: run `tutti-migrate-pg schema`
  outbox:
    pollInterval: 500ms
    batchSize: 200
    retryBackoff: 1s
    maxBackoff: 1m
    retention: 24h
```

Type: `controlplane.Config` (`pkg/controlplane/config.go`), `WithDefaults()` + `Validate()`. Env override should follow
the existing `TUTTI_` mapping (`controlPlane.postgres.dsn` -> `TUTTI_CONTROLPLANE_POSTGRES_DSN`).

### Wiring architecture (implemented in Phase 6 Part 2)

1. **Config**: `ControlPlane controlplane.Config` in `config.Common` (`pkg/config/config.go`); validated at start.
2. **Factory & Publisher**: when `controlPlane.backend: postgres`, `service/worker/server.go` opens `pg.Store` and starts
   `Publisher.Run` (with `MetaRedis: appRedis`, advisory lock so one worker is active at a time), exporting
   `controlplane_outbox_pending`, `controlplane_outbox_oldest_seconds`, and `controlplane_outbox_failures_total`.
3. **Dual-backend adapters**: all 7 configuration families (`accesspolicy.Store`, `failover.Store`, `proxyaccount.Store`,
   `auth.UserStore`, `objstore.credsSvc` for storages/identities/dynamic credentials/storage tests, `policy.Service` for
   routing & replication, and `protectiongroups.Store`) support `.WithControlPlane(cpStore)`.
4. **Transactions (`Store.Tx`)**: check-then-act sequences run atomically inside a PostgreSQL transaction when
   `backend: postgres` — runtime storage delete with reference check (`DeleteRuntimeStorage` + `CheckStorageReferencesInTx`),
   replication creation (`AddUserReplicationPolicy`, `AddBucketReplicationPolicy`), failover switch (`FailoverReplicationWithEvent`),
   and S3 user delete (`s3users.Manager.Delete`).
5. **Actor, request id & Audit Log**: `auth.Interceptor` and `api.UnaryServerInterceptor` populate `controlplane.WithActor(ctx, Actor{Name, RequestID})`.
   `Resilience.ListAuditLog` (`GET /resilience/audit-log`) and the Console UI **Audit log** page (`TuttiAuditLogPage.vue`)
   provide filtering by object type, object ID, actor, action, and cursor pagination (`beforeId`).
6. **Proxy**: reads the projected Redis keys unchanged; no PostgreSQL connection required on proxies.
7. **Fallback reader & 503 writes**: for `backend: postgres`, reads query PostgreSQL first and fall back to the warm Redis
   projection if PostgreSQL is unreachable; mutations return 503 (`codes.Unavailable` via `dom.ErrUnavailable` or `controlplane.ErrReadOnly`).

### Rollout

1. Deploy the build with `backend: redis` (no behaviour change). Create the database, run `tutti-migrate-pg schema`.
2. `migrate --dry-run`, then `migrate` while writers are still active, then `verify`; repeat `migrate` (idempotent) until
   the diff is only recent changes. Check warnings (malformed entries).
3. Cutover window (minutes, S3 traffic is not interrupted): freeze automatic failover (`tutti:protection:automatic-frozen`
   and failover policies disabled), stop admin/console writes, last `migrate`, `verify` must exit 0.
4. Roll the workers with `backend: postgres` (publisher starts, `Pending` must drain to 0). Proxies need no restart.
   Unfreeze failover; run `verify` again after the first real changes (Redis and PostgreSQL stay equal because the
   publisher projects every change).
5. Keep the backend `redis` code path for at least one release.

### Rollback & Redis Rebuild

While `backend: postgres` is on, Redis is a warm projection, so rollback is: stop writes, wait for `Pending == 0`
(or run `tutti-migrate-pg verify`, exit 0), set `backend: redis`, restart the workers. To rebuild a lost Redis from
PostgreSQL, run `tutti-migrate-pg republish` (see section 5 for `--force` / `--prune`).

### Failover with the PostgreSQL backend

Routing and failover events are authoritative in PostgreSQL, so an automatic failover must go through the control
plane. Proxies have no PostgreSQL connection, therefore the **proxy failover controller delegates**: when the workers
announce `backend: postgres` (Redis key `tutti:fo-exec:backend`, written by every worker on start) the proxy queues
the decision (`tutti:fo-exec:requests`) and waits (20 s) for a worker, which applies it with
`FailoverReplicationWithEvent` (routing + event in one PostgreSQL transaction, then the Redis side) and replies. The
proxy never writes routing or events to Redis itself in this mode. If no worker answers the controller logs it and
re-evaluates in the next cycle; requests older than 60 s are dropped. Run at least one worker; with the PostgreSQL
backend a failover needs PostgreSQL (and a worker) to be available.

`FailoverReplicationWithEvent`, the zero-downtime/downtime switch calls and `DeleteReplicationSwitch` span PostgreSQL
and Redis (pending flag, queue pause, switch record) and are **convergent**: each step is idempotent and repeating the
call completes whatever an earlier call left undone (a failed Redis step after the PostgreSQL commit; a failed
PostgreSQL step rolls the Redis marker back). A failed failover never leaves `failover_pending` set after a definite
PostgreSQL rejection (read-only, 503 before the commit); after an unknown commit outcome the flag stays and the retry
decides from the stored routing.

### Starting without PostgreSQL

The worker opens the control-plane pool lazily (`pg.OpenLazy`): if PostgreSQL is unreachable at start the worker still
starts, serves the Redis projection, rejects mutations with `ErrReadOnly` (503), logs the error and retries
(`Store.RunReadiness`, every 5 s, including `autoMigrate`) until the database answers. The gauge
`controlplane_postgres_ready` (1/0) exposes it; the outbox publisher stays idle until then.

### Polling cost

Workers poll managed storages, S3 users and dynamic credentials every ~2 s. With PostgreSQL they first read a digest
of the table (`KV.Fingerprint`: md5 of keys and versions, no documents, no secrets) and skip the full SELECT, the
decoding and the credential opening while it is unchanged. Console-user lookups (one per API request) are cached for
2 s per process and dropped on local writes: a disabled or deleted console user is rejected immediately by the
process that made the change and within 2 s by the others.

### Outbox publisher

One publisher runs at a time (session advisory lock on a dedicated connection). The lock is held while Redis is
written but **no database transaction stays open**: each entry is applied (10 s Redis timeout) and marked published
in its own statement; connections also carry `idle_in_transaction_session_timeout=60s`. Due entries are selected in
SQL (`next_attempt_at <= now()` on the database clock, and not behind an earlier entry of the same object that waits
for its retry), so poison entries cannot starve newer ones, while per-object order is kept. All replication entries of
a scope share one object key (`replication_scope/<user>/<fromBucket>`), writers of a scope serialize on an advisory
lock and the scope projection (absolute sets) is computed under it. Metrics and alerts: `controlplane_outbox_pending`,
`controlplane_outbox_oldest_seconds` (`TuttiControlPlaneOutboxStalled`), `controlplane_outbox_failures_total`.

### Read-only fallback

`Store.SetReadOnly(true)` / `controlPlane.readOnly` rejects all mutations with `ErrReadOnly` (mapped to HTTP/gRPC 503 Unavailable)
and keeps reads working; the publisher keeps draining. Operationally: when PostgreSQL is lost, the proxies are unaffected (they read the Redis
projection, and the Phase 1 snapshot keeps the last known data), the console shows the config read-only, S3 traffic and
replication continue; failover decisions that must write routing wait for PostgreSQL (or an operator uses `readOnly:
false` after restoring a backup).

## 7. Notes & Design Decisions

- `pkg/protectiongroups` splits desired configuration (`GroupSpec`, authoritative in PostgreSQL `protection_groups` when
  `backend: postgres`) from runtime promotion/fence state (`PromotionState` in Redis `tutti:protection:promotion` and
  projected into `tutti:protection:groups` so proxy admission Lua scripts run at line rate without PostgreSQL).
- The replication switch state machine stays in Redis (workers drive it with locks and queues).
- `controlplane.IsUnavailable` is an allow-list (connection errors, timeouts, closed pool/transaction, SQLSTATE classes
  08/57P/53300/57014/25P03): only those fall back to Redis on reads and map to 503 on writes; every other error is
  returned as it is.
- The importer does not delete PostgreSQL-only records (`extra` in the report) and does not copy Redis version
  counters (the publisher recreates them).
- Console `last-login` is mirrored to Redis best effort via outbox without writing an audit log entry.
