# Performance and Redis assessment (2026-11-10)

## What was measured

The detached performance worktree at `/tmp/claude-1000/-home-hellc-ReplicaT4/ebf294dd-87df-4dc5-b7d6-289b454a8387/scratchpad/perf/` contains MinIO Warp JSON results, a summary table, proxy CPU profiles and Redis MONITOR captures. It used one local MinIO backend, one local proxy (`:9979`), and Redis on `:16379` with benchmark DBs 10–13. Its `run.py` ran 15-second Warp samples (plus setup) at concurrency 1, 16 and 64. The source worktree is at commit `2c20181`, older than this report's main tree. Results are an indicative local baseline, not a production capacity or regression claim. The raw files remain in that worktree under `res/` and `prof/`.

| Operation | Size, concurrency | Direct MinIO | Proxy | Proxy/direct | Proxy p50 / p99 |
| --- | --- | ---: | ---: | ---: | ---: |
| PUT | 4 KiB, 16 | 1,243 objects/s | 500 objects/s | 40% | 26.2 / 60.5 ms |
| GET | 4 KiB, 16 | 5,795 objects/s | 1,530 objects/s | 26% | 9.5 / 29.3 ms |
| HEAD | 4 KiB, 16 | 4,477 objects/s | 1,225 objects/s | 27% | 12.3 / 48.6 ms |
| PUT | 1 MiB, 16 | 245 MiB/s | 133 MiB/s | 54% | 99.2 / 120.3 ms |
| GET | 1 MiB, 16 | 1,554 MiB/s | 652 MiB/s | 42% | 19.4 / 55.7 ms |
| GET | 64 MiB, 16 | 2,333 MiB/s | 1,389 MiB/s | 60% | 695.3 / 1,220.1 ms |

The Warp summary's 64 MiB throughput is local memory/network throughput, not a disk durability measurement. Some weighted percentiles in the generated table are nonmonotonic; use the raw Warp JSON for a precise latency comparison. DELETE-only Warp reported zero objects/s, so those rows are excluded. The separate local A/B experiment (`res/ab.txt`) compared a baseline proxy and an experimental transport patch; it is not a controlled enough result to merge the patch without a fresh run on current code.

The proxy CPU profile for 4 KiB GET showed `internal/runtime/syscall/linux.Syscall6` at 49.2% of sampled CPU; the 1 MiB PUT profile showed 23.9% in that syscall and 16.0% in SHA-256. This suggests network/syscall and hashing costs, but the profile does not isolate an individual function as a safe optimization target. The benchmark MONITOR files recorded 259 commands in plain mode and 1,114/1,264 in async/sync modes. These include background queue workers and periodic status activity; they are **not** Redis commands per request. A useful follow-up is a quiet, isolated, timestamped one-request trace with the queue workers disabled or separately accounted for.

### Fresh paired GET baseline on current proxy code

On 2026-11-10, `tools/perf/paired-get.sh` ran three 10-second pairs against direct local MinIO (`:19001`) and an isolated proxy (`:29979`). The proxy binary was built from commit `6145c6d` with synchronous replication and failover disabled, using Redis `:26379` DBs 10–13. The harness ran from working tree HEAD `a42fc23` with 15 modified or untracked files; the intervening changes did not target the isolated GET path. It seeded 256 distinct 4 KiB objects in its own bucket, alternated direct/proxy order, used concurrency 16, reported zero Warp errors, and removed the bucket afterward. The Warp image was pinned to digest `sha256:72ae1b02216b51bd102b73a6923b597c092a5acc7a655c936c2f31bee897a7c5`.

| Pair | Direct GET/s | Proxy GET/s | Proxy/direct |
| --- | ---: | ---: | ---: |
| 1 (direct first) | 2,897 | 1,085 | 37.5% |
| 2 (proxy first) | 1,915 | 1,035 | 54.0% |
| 3 (direct first) | 3,253 | 1,214 | 37.3% |

The median paired ratio was 37.5%; direct throughput ranged from 1,915 to 3,253 GET/s. The range is wide enough that this is a direction and scale of local proxy overhead, not a precise speedup target. UI validation was finishing with low CPU load; no Docker builds or integration tests ran during these samples. The backend and proxy shared one host, and the test had only 256 hot objects, so cache, loopback networking and host scheduling limit generalization. Raw Warp JSON, the summary TSV, and recorded environment are in `/tmp/tutti-perf-current/results-20261110b/`. The earlier `/tmp/tutti-perf-current/results-20261110/` attempt stopped before measurement because its proxy process exited; exclude it.

### Repeat the baseline

For a repeatable read-only GET comparison, use `tools/perf/paired-get.sh` with `DIRECT_ENDPOINT`, `PROXY_ENDPOINT`, `S3_ACCESS_KEY`, and `S3_SECRET_KEY` set. It seeds and cleans up a dedicated bucket, then retains JSON and summary files in the chosen results directory. For broader PUT/GET/HEAD coverage, run a separate benchmark stack with a dedicated bucket and Redis DBs, then use the detached harness, for example `python3 run.py direct http://127.0.0.1:19001 quick` and `python3 run.py proxy-plain http://127.0.0.1:9979 quick` from the perf worktree. `run.py` calls `docker run --network host minio/warp` and assumes `minioadmin` demo credentials. Do not point its PUT/DELETE workloads at production buckets. Preserve raw JSON, source commit, host CPU/RAM, backend type, proxy config, Redis config, and concurrent load for each comparison. The existing result files provide no repeated-run variance or production-like WAN data.

## Redis commands per request after the policy snapshot

PERF-3 confirmed: before the change the proxy policy middleware read routing, routing blocks, zero-downtime switches, replication policies and generations for **every** request in one MULTI/EXEC of 12 commands (14 sent to Redis with MULTI and EXEC). The proxy now keeps the resolved data per (user, bucket) in memory (design: [configuration.md](./configuration.md#policy-snapshot)).

Measured with `TestSnapshot_RedisCommandsPerRequest` (`pkg/policy/snapshot_test.go`: a counting go-redis hook against miniredis, 100 requests each, policy context only, steady state, data already loaded):

| Request class | Before | After |
|---|---:|---:|
| GET, HEAD, LIST (bucket) | 14 | **0** |
| ListBuckets (no bucket) | 14 | **0** |
| PUT, DELETE, POST, multipart, bucket/ACL/tag writes | 14 | **1** (`GET tutti:cfg:version`) |
| first request of a (user, bucket) after any routing/replication/switch change | 14 | 1 + 14 (reload, once per proxy and pair, concurrent requests share one load) |

The version read for writes is what keeps writes correct: a write can never use routing, switch or generation data older than the last announced change. A write on the proxy additionally issues the commands it always did to record the replication event (version vector update and task enqueue), which are not part of this table. 

Whole proxy, measured end to end with `test/resilience/redis_commands_test.go` (real proxy and worker over MinIO, miniredis `CommandCount`, idle worker activity subtracted, 200 requests each, bucket without replication, 4-byte object): snapshot disabled GET 14.0, HEAD 14.0, LIST 14.0, PUT 14.0 commands per request; snapshot enabled **GET 0.0, HEAD 0.0, LIST 0.0, PUT 1.0**. No other component issues Redis commands on the read path with the default config (rate limiting is off by default; auth keys and access policies are cached with a TTL). A PUT to a bucket with replication adds the event recording commands.

Costs of the mechanism: one `GET tutti:cfg:version` per proxy per second (`snapshot.pollInterval`), one subscriber connection per proxy, and for writers (worker API, failover controller) one extra `EVAL` (INCR + PUBLISH) per change to routing/replication/switch data, inside the same transaction when the change is a transaction. Replication status, version vectors and queues do not invalidate the snapshot.

What is cached, and why it is safe:

| Data | Source | Cached | Why safe |
|---|---|---|---|
| user and bucket routing, routing blocks | `p:route_*` | yes | writes validate the version per request; reads lag a change by one notification (ms) or one poll |
| zero-downtime switch active flag and info | `p:repl_*_switch_*` | yes | switch creation changes routing, switch info and policy in one MULTI/EXEC that also bumps the version, so a validated write sees all or nothing |
| replication policies | `p:repl_*_from`, `p:repl_*_to` | yes | same; replications whose source is not the routed storage are dropped as before |
| replication generations | `p:repl_*_gen` | yes | an old generation would make the worker drop the event, so writes always validate the version before using it |
| replication status (paused, archived), version vectors, queues, locks | other keys | **no** | not read by the policy middleware; the event-recording path keeps reading them from Redis as before |

If Redis is unreachable, GET/HEAD/LIST continue from the last data (`policy_snapshot_stale_served_total`), a request for a (user, bucket) never seen before and every write answer `503`. A proxy whose last successful Redis contact is older than `ready.maxStaleness` reports not ready. Changes written to Redis by other tools (redis-cli, restore) are picked up within `snapshot.fullReload` (30 s).

## Is storing everything in Redis a problem?

Tutti does **not** put S3 object bodies in Redis. Object bytes live in configured S3 backends. Redis stores control-plane records and coordination state. Four logical DB numbers (`metaDB: 0`, `queueDB: 1`, `lockDB: 2`, `configDB: 3`) are on the **same Redis instance** in the demo; they separate keys but share memory, persistence, process, and failure domain. The data model document still calls `metaDB` `appDB` in one example; `pkg/config/config.yaml` is authoritative.

| Data | Redis location | Loss impact |
| --- | --- | --- |
| Object and bucket version vectors, migration progress | Meta DB | May require reconciliation/rescan; unsafe to assume prior replication state remains correct. |
| Asynq task queues and pending replication work | Queue DB | Pending work may be lost; source and target objects can diverge until recovered by a full scan. |
| Distributed locks | Lock DB | Ephemeral; stale locks may delay work, lost locks during active work can cause duplicate processing. |
| Routing/replication/failover policies, managed storages, access keys, users and access policies | Config DB | Administrative configuration and credentials can be lost. Managed storage and key secrets are sealed; restore also requires the external master key. |
| Proxy sessions and live status | Meta/config DB | Sessions and status can be recreated; status keys have short TTLs. |
| Traffic history | Config DB | Six-hour sample lists are capped and expire; loss affects historical charts only. |
| S3 object bodies | S3 backends | Redis backup does not protect the underlying object bytes. |

Redis is therefore a **critical durable dependency**, even though some records are transient. AOF with `appendfsync everysec` can lose the latest acknowledged Redis writes on an abrupt host failure (typically around one second, potentially more); it is not an off-host backup. Losing queue or version metadata can undermine replication guarantees even if the S3 object body survives. The demo uses one Redis container and one named volume, so a host/volume failure remains a single point of failure.

The initial demo inspection found `maxmemory:0` (unbounded), AOF enabled with `appendfsync:everysec`, `noeviction`, approximately 2.90 MiB used memory, and keys in DB 0, 1 and 3. The hardened demo was then rebuilt with a pinned Redis image digest and a 512 MiB memory limit. A subsequent live `CONFIG GET` confirmed `maxmemory:536870912`, `maxmemory-policy:noeviction`, `appendonly:yes`, and `appendfsync:everysec`; `used_memory` was 2,575,048 bytes. These are measurements of the small local demo only, and 512 MiB is a demo guardrail rather than a production sizing recommendation. `noeviction` is appropriate for queue and control data: eviction would silently remove needed state. With a finite memory limit, Redis will reject writes at the limit; monitor memory and alert well before that point. Size for dataset growth plus clients, replication buffers, and AOF/RDB copy-on-write. An unbounded limit risks host OOM; an undersized limit causes write failures and proxy/worker disruption.

## Deployment and recovery

1. Use a persistent Redis volume, AOF enabled with a documented fsync policy, and off-host protected backups. Back up all logical DBs together. The single Docker volume is not a backup. Pin the Redis image version and test upgrades and restores before production use.
2. Keep `noeviction`. Set a memory limit based on observed peak plus substantial headroom, then monitor `used_memory`, `maxmemory`, `evicted_keys`, `rejected_connections`, AOF write/rewrite status, disk free space, queue depth, and replication lag.
3. For availability, use a Redis deployment with failover and persistent replicas, and configure Tutti's Redis addresses/Sentinel settings accordingly. HA does not replace backups; replication can carry accidental deletion or corruption to replicas. Verify that the chosen topology supports the four configured logical DBs. Redis Cluster uses only DB 0, so four DB defaults cannot be used unchanged with Cluster.
4. Back up the external dynamic-credentials master password/key separately from the RDB/AOF. Without it, restored sealed access keys and managed storage credentials cannot be used. Protect backups like credentials.
5. Recovery should stop proxy and worker writes, restore Redis and master key together, restart components, then validate policies, task queues and S3 consistency. If the Redis snapshot predates acknowledged object writes, run a full source/target reconciliation before treating replicas as current.

The demo Docker scripts `tools/redis/backup-rdb.sh <container> <output.rdb>` and `tools/redis/check-restore.sh <output.rdb> [image]` create an RDB snapshot and load it in a disposable container without restarting or flushing the live Redis. The backup uses `BGSAVE`, copies the completed snapshot, checks RDB structure, and writes mode-600 output. The restore check starts Redis with AOF and further saves disabled, a read-only RDB mount, and no host network, then prints the restored DB key counts. It does not validate application-level semantics or replace an off-host backup schedule. An RDB omits writes after its snapshot point; preserve AOF as well if the recovery point objective demands it.

Validation on 2026-11-10: `tools/redis/backup-rdb.sh tutti-runtime-review-redis /tmp/tutti-redis-review-backup-1794268118.rdb` produced a 6,778-byte RDB. `tools/redis/check-restore.sh ... <same Redis image>` loaded it in a disposable Redis container and reported keys in DB 0, 1 and 3. A later backup of the hardened demo, `/tmp/tutti-redis-post-hardening.rdb`, was 19,290 bytes (mode 600) and was independently loaded in a disposable container with keys in DB 0, 1 and 3. Neither Redis instance was restarted or flushed.

## Priorities

1. Establish off-host Redis backups and scheduled restore drills, plus protected master-key backup. This is the highest-impact gap for correctness and recoverability.
2. Extend the fresh small-object GET baseline to repeat HEAD/PUT and larger objects on a production-representative backend. Small-object GET/HEAD and PUT overhead are the clearest measured performance opportunities; profile again before changing transport, routing, or Redis query paths.
3. Add Redis memory/queue/lag dashboards and alerts; capacity-plan with realistic numbers of objects, policies and queued events.
4. Treat any transport patch from the detached worktree as experimental until a current-code correctness and A/B performance run confirms it.
