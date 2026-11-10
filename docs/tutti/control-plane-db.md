# Control-plane database (PostgreSQL), phase 6, part 1

Status: package, schema, outbox publisher and migration tool are implemented and tested; **not wired** into
`service/worker` or `service/proxy` yet (section 6 lists the exact steps). Plan reference: [scale-plan.md](./scale-plan.md), Phase 6.

Code: `pkg/controlplane` (models, repository interfaces, `controlPlane` config), `pkg/controlplane/pg` (PostgreSQL
implementation, migrations, audit log, outbox, publisher), `pkg/controlplane/migrate` (Redis to PostgreSQL copy and
verification), `pkg/controlplane/pgtest` (test PostgreSQL), `tools/tutti-migrate-pg` (command).

## 1. Inventory: what is stored in Redis today

Redis roles (`pkg/config`): **config** (`redis.configDB`) holds the control plane, **meta** (`redis.metaDB`) holds
version vectors, uploads, sessions and the dynamic credentials hash, **queue** the asynq queues, **lock** the locks.
"Writers" below are the code that mutates the object; every consumer polls or reads Redis directly.

### 1.1 Config (source of truth moves to PostgreSQL)

| Object | Redis key(s) | Go store (file) | Writers | Readers | Consistency need today |
|---|---|---|---|---|---|
| Runtime storages | HASH `tutti:storages` (name -> JSON, credentials sealed), STRING `tutti:storages:version` | `objstore.credsSvc` Create/Update/Delete/RotateConnectionAccount (`pkg/objstore/managed.go`) | worker API | worker + proxy poll the version (<= 2 s), rebuild their storage view | create-if-absent Lua + version INCR in one script; update/delete under the per-storage Redis lock `tutti:storage:lock:<name>`; the reference check on delete is check-then-act (LOGIC-06) |
| Tutti user records (S3 identities) | HASH `tutti:s3users` (user -> `{createdAt}`), STRING `tutti:s3users:version` | `objstore` AddIdentity/RemoveIdentity (`connection.go`) | worker API (`pkg/s3users`) | worker + proxy poll the version | create-if-absent Lua + INCR; delete in MULTI |
| Sealed dynamic credentials | HASH `chorus:dynamic_creds` (**meta role**): fields `version` (counter), `salt` (`<parallelism>:<b64>`), `S3:<storage>:<user>` / `SWIFT:...` (AES-GCM sealed JSON) | `objstore.credsSvc` SetS3Credentials/Delete (`credentials.go`) | worker API | worker + proxy poll `version` | MULTI HSET + HINCRBY; salt HSETNX (the salt must never change, sealed values depend on it) |
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

One migration, `pkg/controlplane/pg/migrations/0001_init.sql`, embedded with `embed`, applied by `pg.Migrate`
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
by `BeforeID` (for the UI). Login time updates are deliberately not audited.

## 4. Change propagation (outbox)

Every mutation also inserts an `outbox` row holding **idempotent Redis operations** that project the committed state
into the keys the services already read (section 1.1): hash/set writes in the exact existing formats, the
`tutti:storages:version` / `tutti:s3users:version` counters, the `version` field of `chorus:dynamic_creds`, and for
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
                 --pg-dsn postgres://tutti:pw@db:5432/tutti?sslmode=verify-full  schema | migrate | verify
```

`--redis-url` is the **config** Redis (`redis.configDB`); `--redis-meta-url` the Redis with `chorus:dynamic_creds`
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

### Wiring steps (next work item)

1. **Config**: add `ControlPlane controlplane.Config` to `config.Common` (worker and proxy); validate at start.
2. **Factory**: `controlplane.Open(cfg)` returns `nil` for `redis`; for `postgres` it returns the `pg.Store` and, in the
   **worker only**, starts `Publisher.Run` (any number of workers; the advisory lock makes one of them active). Expose
   `outbox_pending`, `outbox_oldest_seconds`, `outbox_failures_total` metrics and add them to the Phase 7 alerts.
3. **Adapters, one family at a time**, in `service/worker/server.go`: each Redis store gets a small interface in its
   own package (`accesspolicy.Repository` List/Get/Set/Delete, `proxyaccount` records, `failover.Store` methods,
   `auth.UserStore`, `objstore` managed storages / identities / dynamic creds, `store` routing and replication stores)
   with the current Redis code as one implementation and a thin PostgreSQL adapter (type conversion only) as the other.
   Order of risk: access policies, failover policies/events, proxy accounts, console users, storages + identities +
   dynamic creds, routing, replication.
4. **Transactions**: replace the check-then-act sequences by `Store.Tx` - storage delete with the reference check
   (LOGIC-06; keep the Redis lock until the check is inside the transaction), replication create (policy + status +
   generation + destination routing block), failover switch (event + routing + policy pause), user delete (identity,
   credentials, policy, accounts).
5. **Actor and request id**: the gRPC/HTTP interceptor (after the auth principal and trace id are known) calls
   `controlplane.WithActor`; add an `AuditLog` RPC + UI page on `Audit.List`.
6. **Proxy**: no change is needed for the first rollout, it keeps reading the projected Redis keys; the proxy needs no
   PostgreSQL connection. A later step can let proxies build their snapshot from PostgreSQL directly.
7. **Fallback reader** for `backend: postgres`: reads try PostgreSQL and fall back to the Redis cache when PostgreSQL is
   unavailable (stale-but-served, flagged in `/ready` and the UI), mutations return 503 (`ErrReadOnly` /
   `ErrUnavailable`). `controlPlane.readOnly: true` forces this mode for maintenance.

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

### Rollback

While `backend: postgres` is on, Redis is a warm projection, so rollback is: stop writes, wait for `Pending == 0`
(or run `tutti-migrate-pg verify`, exit 0), set `backend: redis`, restart the workers. Not covered: protection groups
(always authoritative in Redis), switch state and `failover_pending` (never left Redis). To rebuild a lost Redis from PostgreSQL a `republish` command (enqueue an outbox snapshot of all rows) is a follow-up;
`migrate.ImportOptions.EmitOutbox` already does it for freshly imported records (used by the projection test).

### Read-only fallback

`Store.SetReadOnly(true)` / `controlPlane.readOnly` rejects all mutations with `ErrReadOnly` and keeps reads working;
the publisher keeps draining. Operationally: when PostgreSQL is lost, the proxies are unaffected (they read the Redis
projection, and the Phase 1 snapshot keeps the last known data), the console shows the config read-only, S3 traffic and
replication continue; failover decisions that must write routing wait for PostgreSQL (or an operator uses `readOnly:
false` after restoring a backup). Backup/restore drills belong to Phase 7.

## 7. Open points

- `pkg/protectiongroups` keeps promotion state in the group document and reads it from Lua; splitting desired state
  (PostgreSQL) from promotion state (Redis) is needed before groups can be authoritative in PostgreSQL.
- The replication switch state machine stays in Redis (workers drive it with locks and queues).
- The importer does not delete PostgreSQL-only records (`extra` in the report) and does not copy Redis version
  counters (the publisher recreates them).
- Console `last-login` is mirrored to Redis best effort.
