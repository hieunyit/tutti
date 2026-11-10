# Redis operations

Redis is a required, durable dependency of Tutti. Object bodies are not stored in Redis, but routing, credentials, users, replication queues, version metadata and locks are. Read [performance and Redis](./performance-and-redis.md) for the measured baseline and [runbooks](./runbooks.md) (d) for incident steps.

## What Tutti stores in Redis

Logical DB numbers come from `redis.metaDB`, `queueDB`, `lockDB` and `configDB` (defaults 0, 1, 2, 3; source `pkg/config/config.go`, clients built in `service/*/server.go`). Only the DB number is configurable; the key names are fixed.

### Configuration (config DB, default 3)

Source of truth for administrative state. Loss means reconfiguration, and some of it is audit-relevant.

| Key | Type | Content |
|---|---|---|
| routing and replication records | hashes/keys written by the policy service | user and bucket routing, replication definitions and status |
| `tutti:failover-policies` | hash | failover policy per source storage |
| `tutti:failover:user:<user>`, `tutti:failover:bucket:<user>:<bucket>` | string (JSON) | failover events not yet acknowledged |
| `tutti:access-policies` | hash | access policy per user |
| `tutti:proxy-accounts` | hash | proxy access keys; secrets sealed with the master key |
| `tutti:storages`, `tutti:storages:version` | hash, string | managed storage definitions (never secrets) |
| `tutti:console-users`, `tutti:console-users:last-login` | hash | console users (bcrypt hashes) and last login |

### Coordination and queues (queue DB 1, lock DB 2)

| DB | Key pattern | Content | Loss impact |
|---|---|---|---|
| 1 (queue) | `asynq:*` (asynq library) | replication and migration task queues, retry and archive sets | pending replication work is lost; source and target diverge until a diff |
| 2 (lock) | `tutti:failover-lock:<storage>` (TTL 30 s) and short-lived object, bucket, user and replication-status locks | coordination between proxies and workers | stale locks delay work until TTL; lost locks during work can cause duplicates |

Locks are expected to expire on their own. Do not treat a lock as data.

### Metadata and runtime state (meta DB, default 0)

| Key | Content | Loss impact |
|---|---|---|
| version vectors, migration progress (`meta`, migration list state stores) | per-object and per-bucket versions, last processed listing position | replication state can be wrong; a full scan may be needed |
| `chorus:dynamic_creds` (hash) | sealed storage credentials per user and storage, plus the `salt` used for the key | **not disposable**: lost secrets must be re-entered; the salt is needed with the master password |
| `tutti:session:*` | console sessions (TTL) | users sign in again |
| `tutti:authfail:*`, `tutti:authknown:*` | login throttling counters (15 min) and known (user, IP) pairs (30 days) | throttling resets |
| rate-limit counters | per-storage request limits | limits reset |

Proxy health and history are ephemeral, in the config DB:

| Key | TTL | Content |
|---|---|---|
| `tutti:proxy-status:<app_id>` | 3 × `status.interval` (default 6 s) | live status per proxy |
| `tutti:proxy-stats-history:<app_id>` | about 6 h | traffic samples for the UI |

Note: the performance report lists "managed storage and key secrets" under the config DB. In code, the sealed per-user credentials (`chorus:dynamic_creds`) are in the meta DB; managed storage definitions are in the config DB. Back up both DBs anyway.

## Persistence

The demo compose file (`docker-compose/docker-compose.yml`) starts Redis with:

```bash
redis-server --appendonly yes --appendfsync everysec \
  --maxmemory ${REDIS_MAXMEMORY:-512mb} --maxmemory-policy noeviction
```

| Setting | Meaning | Recommendation |
|---|---|---|
| `appendonly yes`, `appendfsync everysec` | write-ahead log, fsync once per second | keep. A host crash can lose about one second of acknowledged writes. |
| RDB snapshots (`save`) | point-in-time file used by the backup script | the compose command does not set `save`; it uses the image default. Check with `CONFIG GET save` and set it explicitly in production. |
| `maxmemory-policy noeviction` | writes fail at the limit instead of deleting keys | keep. Eviction would silently delete queue tasks, version metadata or credentials. |
| `maxmemory` | limit before writes fail | set it from measured usage, below. The demo value is a guardrail, not a sizing. |

`CONFIG SET` changes are not persisted unless you also use `CONFIG REWRITE` or change the start command.

### Sizing `maxmemory`

Measured on the demo: about 2.5 MiB used with the 512 MiB limit. That tells you nothing about production. Use this process:

1. Load a representative dataset (objects, versions, users, policies, queued events) and read `used_memory` (`INFO memory`).
2. Project linearly with object count, replication count and expected queue depth (queue backlog is the largest variable).
3. Set `maxmemory` to the projection plus headroom for fork copy-on-write during RDB snapshots and AOF rewrites under write load. Our recommendation is a limit well below host RAM (for example half), and alerts well before the limit (see monitoring).

An unset limit risks host OOM. A limit that is too small causes write failures (`OOM command not allowed`), which surface as 503s to clients.

## High availability with Redis Sentinel

**Status: available now in configuration and client code.** Tutti builds a Sentinel-aware client when `redis.sentinel.masterName` is set (`pkg/util/redis.go`). The Helm chart has an external-Redis example for Sentinel (`deploy/chorus/examples/values-external-redis.yaml`). The repository's tests only cover parsing of these keys; no test starts a Sentinel topology. Verify failover behaviour in your environment before relying on it.

Tutti keys:

| Key | Env override | Meaning |
|---|---|---|
| `redis.addresses` | none | sentinel addresses when `masterName` is set |
| `redis.sentinel.masterName` | `CFG_REDIS_SENTINEL_MASTERNAME` | name of the monitored master (empty = no Sentinel) |
| `redis.sentinel.user`, `redis.sentinel.password` | `CFG_REDIS_SENTINEL_USER`, `CFG_REDIS_SENTINEL_PASSWORD` | credentials for the sentinels (ACL) |
| `redis.user`, `redis.password` | `CFG_REDIS_USER`, `CFG_REDIS_PASSWORD` | credentials for the master and replicas |
| `redis.tls.enabled`, `redis.tls.insecure` | | TLS for all Redis and sentinel connections |
| `redis.metaDB`, `queueDB`, `lockDB`, `configDB` | | logical DBs, unchanged with Sentinel |

Example worker or proxy config:

```yaml
redis:
  addresses:
    - sentinel-1.example.internal:26379
    - sentinel-2.example.internal:26379
    - sentinel-3.example.internal:26379
  sentinel:
    masterName: tutti
    user: ""
    password: "<sentinel-password>"
  user: ""
  password: "<redis-master-password>"
  metaDB: 0
  queueDB: 1
  lockDB: 2
  configDB: 3
```

Important:

- **Give at least two sentinel addresses.** The queue client (asynq) uses standalone mode when exactly one address is configured, even if `masterName` is set, so a single sentinel address breaks the queues. Three sentinels are the usual minimum for quorum.
- `masterName` must match the name in the sentinel `monitor` line.
- Sentinel failover uses asynchronous replication. Writes acknowledged by the old master but not yet replicated can be lost on failover. This is the same kind of loss as AOF `everysec`, and it is not covered by Sentinel.

Example topology (one primary, two replicas, three sentinels; adapt host names and timeouts):

```
primary    10.0.1.10:6379   appendonly yes, appendfsync everysec, maxmemory-policy noeviction
replica-1  10.0.1.11:6379   replicaof 10.0.1.10 6379
replica-2  10.0.1.12:6379   replicaof 10.0.1.10 6379
sentinel   10.0.1.21:26379 ... .22 ... .23 (same config on each)

# sentinel.conf
sentinel monitor tutti 10.0.1.10 6379 2
sentinel auth-pass tutti <redis-master-password>      # if the master requires a password
sentinel down-after-milliseconds tutti 5000
sentinel failover-timeout tutti 60000
```

Put the replicas on separate hosts or zones from the primary, and keep `maxmemory` identical on all data nodes (a promoted replica with a smaller limit can start refusing writes).

Not an option today:

- **Redis Cluster.** Not supported. Tutti uses four logical DBs. With more than one address and no `masterName`, the client becomes a cluster client, and cluster mode has only DB 0, so the four DBs would share one keyspace. Tutti's multi-key scripts and transactions were not validated against cross-slot behaviour, so cluster mode is treated as unsupported. Planned change: PERF-4 in the [scale plan](./scale-plan.md#4-external-review-2026-11-10-verified-findings) (separate instances for config, queues and locks, and key prefixes instead of DB numbers).

## Backup and restore

Tools in `tools/redis/`. Both scripts are for the Docker demo and do not restart or flush Redis.

| Script | Use |
|---|---|
| `tools/redis/backup-rdb.sh <redis-container> <output.rdb>` | `BGSAVE` in the running container, copies the completed RDB out (mode 600), checks it with `redis-check-rdb`. Default container name in the script is `tutti-redis-1`. Refuses to overwrite an existing output. |
| `tools/redis/check-restore.sh <snapshot.rdb> [redis-image]` | loads the RDB in a disposable Redis container (no network, read-only mount, AOF and saves off) and prints keys per DB. It does **not** restore into the live Redis. |

Drill (recommended cadence; this is our recommendation, not a schedule the repo enforces):

1. Take a snapshot: `tools/redis/backup-rdb.sh tutti-redis-1 /secure/backups/tutti-$(date +%F).rdb`.
2. Verify it loads: `tools/redis/check-restore.sh /secure/backups/tutti-<date>.rdb`. Expect keys in DB 0, 1 and 3 (DB 2 holds short-lived locks and can be empty).
3. Copy the snapshot off the host. The single Docker volume is not a backup.
4. Back up the **master password** separately from the snapshot. Without it, sealed credentials and proxy keys cannot be used after a restore. Protect both like credentials.

Restore into a live environment (verify each step in staging first):

1. Stop all workers and proxies, so no writes or task consumers run.
2. Stop Redis. With AOF enabled, Redis loads the AOF in preference to the RDB at startup. Move the AOF data aside (Redis 7 and later keep it in the `appendonlydir` directory inside the data directory) so the RDB is used.
3. Place the snapshot as `dump.rdb` in the data directory. Start Redis with `--appendonly no`, check `INFO keyspace`, then `CONFIG SET appendonly yes` to recreate the AOF, and `CONFIG REWRITE` if the setting must persist.
4. Start the workers and proxies with the same master password. Check `GET /tutti/proxy-accounts`, `GET /tutti/users` (`complete`), `chorctl repl` and `chorctl proxies`.
5. A snapshot older than the last acknowledged writes loses those writes. Run `chorctl diff check` on buckets that changed after the snapshot, and restore from object storage state where needed.

## Monitoring

Use `INFO` (or an exporter such as redis_exporter) and the Tutti API. The thresholds below are starting points, not measured limits.

| Signal | How | Alert (suggested) |
|---|---|---|
| memory | `used_memory` / `maxmemory` | warn at 70%, page at 85% |
| evictions | `evicted_keys` | any value above 0 (policy was changed) |
| rejected writes | client errors `OOM command not allowed`; proxy 503 rate | any sustained rate |
| persistence | `rdb_last_bgsave_status`, `aof_last_write_status`, `aof_rewrite_in_progress` | any `err`; rewrite running longer than expected |
| fork cost | `latest_fork_usec` | rising trend (copy-on-write pressure) |
| disk | free space on the AOF volume, AOF file size | less than 20% free |
| latency | `LATENCY LATEST`, `SLOWLOG GET 10` | sustained slow commands on DB 1 or 0 |
| connections | `rejected_connections`, `connected_clients` | any rejection |
| keyspace | `INFO keyspace` per DB | unexpected growth in DB 1 (queue) |
| queue depth | Asynq inspector or counts of `asynq:*` keys in DB 1 | growth over time; a non-draining queue means failing workers |
| Tutti | `chorctl proxies` (count of live proxies), `chorctl repl` (`EVENTS` done vs total), `failover_total`, `storage_available` | fewer proxies than expected; events not draining; any failover |

Commands for a quick check:

```bash
redis-cli INFO memory | grep -E 'used_memory_human|maxmemory_human|evicted_keys'
redis-cli INFO persistence | grep -E 'rdb_last_bgsave_status|aof_last_write_status|aof_rewrite_in_progress'
redis-cli INFO keyspace
redis-cli SLOWLOG GET 10
```

Check exact key names for queue depth on your asynq version before writing an alert on them.

## Related

- [Runbooks](./runbooks.md) (d): Redis down or full.
- [Operations](./operations.md): what Tutti does when Redis rejects writes.
- [Performance and Redis](./performance-and-redis.md): measurements and priorities.
- [Scale plan](./scale-plan.md) Phase 1 (Redis HA, last-known-good snapshot) and PERF-4 (separate instances): planned.
