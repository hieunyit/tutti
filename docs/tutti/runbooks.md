# Runbooks

Step-by-step procedures for operating Tutti. Mechanisms are described in [operations](./operations.md); API paths in [api](./api.md); config keys in [configuration](./configuration.md).

Status labels: **Available now** means the procedure works with the current code. **Planned** means the referenced capability is not implemented yet (see the [scale plan](./scale-plan.md)).

## Conventions

```bash
export API=http://localhost:9671          # worker REST API (api.httpPort)
# Only when api.auth.enabled is true:
export TOKEN=$(curl -s -X POST $API/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<password>"}' | jq -r .token)
AUTH=(-H "Authorization: Bearer $TOKEN")  # when api.auth.enabled is false, use AUTH=() instead
```

`chorctl` reads its address from `-a` (default `http://localhost:9671`) and the token from `--token`, `TUTTI_TOKEN`, or the file saved by `chorctl login`.

Verbs follow [api.md](./api.md): reads are `GET` or `POST` (for example `POST /routing`), changes are `PUT` or `POST` as listed there. `PUT /routing/update` and the failover event and policy routes are admin only.

Durations in JSON bodies are protobuf strings such as `"3600s"`. Forms like `"1h"` are not accepted.

---

## (a) A storage is down and automatic failover happened

**Status: Available now** (automatic failover requires `failover.enabled: true` on proxies and an enabled policy; see [failover](./operations.md#automatic-failover)).

Goal: confirm what moved, what is still at risk, and what to do before failback.

1. **Read the failover events.** Each event is one scope (user, or user + bucket) that moved.

   ```bash
   chorctl failover events
   curl -s "${AUTH[@]}" $API/tutti/failover/events
   ```

   Record `user`, `bucket` (empty for user-level), `from`, `to`, `time` and `eventLag`. `eventLag` is the number of replication events that were not yet applied on the target when routing moved. Those objects may be missing or stale on the new primary.

2. **Confirm the storage state on every proxy.**

   ```bash
   chorctl proxies            # STORAGES column: name=up or name=DOWN, FAILOVER column
   curl -s "${AUTH[@]}" $API/tutti/proxies
   ```

   Look at `storages[].available`, `consecutiveFailures` and `lastError`. Proxies disagree when only some of them saw the outage; that is normal for a few seconds.

3. **Confirm routing now points to the target.**

   ```bash
   curl -s -X POST "${AUTH[@]}" $API/routing -H 'Content-Type: application/json' -d '{}'
   ```

   Check `userRoutings` and `bucketRoutings` for the users and buckets from step 1.

4. **Check the copy count.** Failover pauses the replication from the failed storage to the new primary. It does **not** create replication from the new primary to other storages, so the scope may now have a single copy until you add one.

   ```bash
   chorctl repl              # PAUSED column: old -> new should be true
   ```

   If a third storage exists, create replication from the new primary:

   ```bash
   chorctl repl add -f <new-primary> -t <third> -u <user> [-b <bucket>]
   ```

   Wait for `PROGRESS` to reach 100% and `EVENTS` done to equal total.

5. **Check the logs and metrics.** Look for the warning `failover: routing switched to replica and replication from the failed storage paused; failback is manual`. With `metrics.enabled`, `failover_total{from,to}` increments per scope.

6. **Decide on failback** (runbook b) only after the old storage is repaired and healthy. Do not acknowledge the event before failback is complete, because acknowledging only removes the record and does not change routing.

---

## (b) Manual failback (return routing to the original storage)

**Status: Available now, with manual steps.** Failback is never automatic. There is no single failback command. The steps below use existing replication and routing APIs. `PUT /routing/update` changes only the routing target; it does not drain queues or create replication, so the order matters.

Preconditions:

- The old storage is healthy on every proxy (`chorctl proxies` shows `up`).
- The workers can reach the old storage with the same credentials.
- You have the `eventLag` value from the failover event. It says how many writes may be missing on the target.

Steps:

1. **Remove the paused forward replication** (old -> new, the one failover paused). Record its `EVENTS` value first.

   ```bash
   chorctl repl                                                         # note EVENTS done/total
   chorctl repl delete -f main -t f1 -u user1 [-b photos]               # old=main, new=f1 in this example
   ```

   Deleting archives the replication status and purges its queue. Work that was still queued for the old storage is discarded here, which is why the diff in step 4 matters.

2. **Create reverse replication**, so the old storage receives everything written during the outage:

   ```bash
   chorctl repl add -f f1 -t main -u user1 [-b photos]
   chorctl repl                                                         # wait for PROGRESS 100%, EVENTS done == total
   ```

3. **Verify the old storage has caught up.** `chorctl repl` shows the queue age in `LAG`. That is the age of the oldest queued task, not a count. The failover policy's `maxEventLag` counts unprocessed events, which you can estimate as `EVENTS` total minus done.

4. **Check consistency** against the current primary as source of truth:

   ```bash
   chorctl diff check f1:photos main:photos --user user1
   chorctl diff report
   chorctl diff fix --source f1:photos main:photos      # only after reviewing the report
   ```

   Objects that existed only on the old storage (writes that had not reached the target at failover time, see `eventLag`) appear in the report. Decide whether they should be copied back before you switch routing.

5. **Freeze writes and switch routing**, with a short block so no client write lands on the wrong side:

   ```bash
   curl -s -X PUT "${AUTH[@]}" $API/routing/block -H 'Content-Type: application/json' \
     -d '{"user":"user1","bucket":"photos"}'
   chorctl repl                                    # EVENTS done == total again, nothing new queued
   curl -s -X PUT "${AUTH[@]}" $API/routing/update -H 'Content-Type: application/json' \
     -d '{"user":"user1","bucket":"photos","toStorage":"main"}'
   curl -s -X PUT "${AUTH[@]}" $API/routing/unblock -H 'Content-Type: application/json' \
     -d '{"user":"user1","bucket":"photos"}'
   ```

   While a user or bucket is blocked, its S3 requests fail with `404 NoSuchBucket` ("Bucket already used as replication destination"). That message is the existing one for blocked routing; it is a block, not a missing bucket. For a user-level scope, omit `bucket` in every call. Failover also moves explicit bucket routings that pointed at the failed storage, so repeat the routing update for each of them.

   Do not use the `chorctl repl switch` commands for this. Their help text describes bucket-level replications, and they also change replication direction, which this runbook handles explicitly.

6. **Restore forward replication immediately after the switch**, otherwise writes on the old storage are not copied to the failed-over storage:

   ```bash
   chorctl repl add -f main -t f1 -u user1 [-b photos]
   ```

   The new replication runs an initial copy. Wait for it to finish with `chorctl repl`.

7. **Remove the reverse replication** once its queue is drained (`EVENTS` done == total):

   ```bash
   chorctl repl delete -f f1 -t main -u user1 [-b photos]
   ```

8. **Acknowledge the failover event** (bucket `""` or omitted for a user-level event):

   ```bash
   curl -s -X PUT "${AUTH[@]}" $API/tutti/failover/events/delete -H 'Content-Type: application/json' \
     -d '{"user":"user1","bucket":"photos"}'
   ```

   Equivalent: `redis-cli -n <configDB> del tutti:failover:bucket:user1:photos` (user-level: `tutti:failover:user:<user>`). Prefer the API.

9. **Verify.** `chorctl failover events` no longer lists the scope, `POST /routing` shows the original storage, and `chorctl repl` shows both directions as expected. Repeat for every scope in the failover event list.

---

## (c) Failover did not trigger

**Status: Available now.** Use this checklist in order. Each item names the condition that the code checks before promoting a target (see [failover](./operations.md#automatic-failover)).

| # | Check | How | Expected | If not |
|---|---|---|---|---|
| 1 | Proxy runs the controller | `curl -s "${AUTH[@]}" $API/tutti/proxies` → `settings.failover.enabled` on each proxy | `true` on proxies that should act | set `failover.enabled: true` in the proxy config and restart it (the file is read at startup) |
| 2 | Policy exists and is enabled | `curl -s "${AUTH[@]}" $API/tutti/failover/policies` | entry for the failed storage, `enabled: true`, non-empty `targets` | create or update it with `POST /tutti/failover/policies` |
| 3 | Target is in the policy | same output | the replica you expect is in `targets` in order | only listed targets are considered |
| 4 | Replication exists from the failed storage to the target, for the scope | `chorctl repl` | from = failed storage, to = target, same bucket name for bucket-level | create it with `chorctl repl add` (bucket-level targets need the same destination bucket name) |
| 5 | Replication not paused | `chorctl repl` → `PAUSED` | `false` | `chorctl repl resume ...` (an operator pause blocks promotion) |
| 6 | Initial migration finished | `chorctl repl` → `PROGRESS` | 100% | wait; a replication still copying is not promoted |
| 7 | Event lag within limit | `chorctl repl` → `EVENTS` (done/total) | unprocessed count ≤ `maxEventLag` (policy value, or `failover.maxEventLag` if unset) | wait for the queue to drain, or raise `maxEventLag` deliberately |
| 8 | Target healthy | `chorctl proxies` → target `up` on every proxy | `up` | fix the target first; a target with an open breaker is skipped |
| 9 | Source breaker open long enough | proxy status `available: false`; metric `storage_available{storage}` = 0 | open for at least `after` (policy value or `failover.after`) | breakers are per proxy; if only one proxy sees the outage, its view counts |
| 10 | Scope routed to the failed storage | `POST /routing` (`{}`) | the user or bucket routes to the failed storage (explicit or implicit via `main`) | a user routed elsewhere is not in scope |
| 11 | No other controller holds the lock | Redis `tutti:failover-lock:<storage>` in the lock DB, TTL 30 s | absent, or held briefly | another proxy is acting; check again after 30 s |
| 12 | Logs | proxy logs: warning with the reasons per scope | the reason tells you which check failed | the log names the failing candidate check (lag, paused, breaker, missing replication) |

After a fix, failover re-evaluates on the next `checkInterval` (`failover.checkInterval`). The event is recorded only if promotion succeeds.

---

## (d) Redis is down or full

**Status: Available now.** Redis is a required dependency for proxies and workers ([redis operations](./redis-operations.md)).

### Symptoms

- Client writes return `503 ServiceUnavailable` with "The change was applied but could not be recorded for replication" (`onEventError: fail`). The data is on the routed storage; the client retries (see [operations](./operations.md#replication-event-recording-and-503-semantics)).
- Requests that need routing, policy or credential lookups fail while Redis is unreachable. These lookups read Redis on every request today.
- Redis returns `OOM command not allowed when used memory > 'maxmemory'` when full (`noeviction`).
- Workers log queue or lock errors.

### Check

```bash
REDIS="redis-cli -h <host> -p 6379"            # add -a / --tls as your deployment needs
$REDIS PING
$REDIS INFO memory | grep -E 'used_memory_human|maxmemory_human|mem_fragmentation_ratio'
$REDIS CONFIG GET maxmemory-policy             # must be noeviction
$REDIS INFO persistence | grep -E 'aof_last_write_status|rdb_last_bgsave_status|aof_rewrite_in_progress'
$REDIS INFO keyspace                           # keys per logical DB (0 meta, 1 queue, 2 lock, 3 config)
$REDIS INFO stats | grep rejected_connections
```

### Recover from memory full

1. **Raise the limit temporarily** (runtime only; persist it in the Redis config or compose file):

   ```bash
   $REDIS CONFIG SET maxmemory 1gb
   ```

2. **Find the growth.** Queue backlog in DB 1 (workers down or failing tasks) is the usual cause. Sample large keys without blocking the server:

   ```bash
   $REDIS --bigkeys
   $REDIS MEMORY USAGE <key>
   ```

3. **Fix the cause**, not the symptom: restart or scale workers so the queue drains; fix the failing storage behind retried tasks.
4. **Do not** set the policy to an eviction policy, do not `FLUSHDB`/`FLUSHALL`, and do not delete `asynq:*` keys by hand. Deleting queued tasks loses replication work. If tasks were dropped, run a diff (`chorctl diff check`) on the affected buckets afterwards.

### Recover from Redis down

1. Restart Redis. With AOF enabled, check the load: `INFO persistence` → `aof_last_write_status:ok`, and `INFO keyspace` shows keys in DB 0, 1 and 3.
2. Confirm proxies publish again: `chorctl proxies` lists every instance (status keys have TTL 3 × `status.interval`).
3. Check the event-recording 503s from the outage window. Clients retry, which records the event again.
4. Because AOF `everysec` can lose about one second of acknowledged writes on a crash, run a diff on buckets written during the crash window.

---

## (e) Rotate a proxy access key, with a grace period

**Status: Available now.** Keys created with `POST /tutti/users` (`createAccessKey`) are stored in the same proxy-account store, so the same calls apply.

1. Find the key:

   ```bash
   curl -s "${AUTH[@]}" $API/tutti/proxy-accounts
   ```

2. Rotate, keeping the old secret for a grace period (default 1 h; `"0s"` means immediate invalidation):

   ```bash
   curl -s -X PUT "${AUTH[@]}" $API/tutti/proxy-accounts/rotate -H 'Content-Type: application/json' \
     -d '{"accessKeyId":"TUTABCDEFGHIJKLMNOPQ","gracePeriod":"3600s"}'
   ```

   The response contains the new secret once. Store it in your secret manager. `previousSecretExpiresAt` shows when the old secret stops working.

3. Give the new secret to the client and verify it, for example with `mc alias set` and `mc ls`.
4. Check that the old secret is no longer needed (client logs, `GET /tutti/proxy-accounts`). Then either wait for the grace period, or rotate again with `"gracePeriod":"0s"`.

If a key is compromised:

```bash
curl -s -X PUT "${AUTH[@]}" $API/tutti/proxy-accounts/status -H 'Content-Type: application/json' \
  -d '{"accessKeyId":"TUT...","status":"disabled"}'      # rejected like an unknown key
curl -s -X PUT "${AUTH[@]}" $API/tutti/proxy-accounts/rotate -H 'Content-Type: application/json' \
  -d '{"accessKeyId":"TUT...","gracePeriod":"0s"}'
```

Changes reach all proxies within about 5 seconds (proxies cache accounts for that long). Keys from `auth.custom` in the proxy config are not managed by this API; change them in the config and restart.

---

## (f) A console admin is locked out

**Status: Available now.** Login throttling is described in [api](./api.md#console-users-api). Summary of the current rules:

- Failed logins are counted per (username, client IP) and per username. The pair limit is 10 failures; the counter expires 15 minutes after the last failure, and a successful login clears the pair counter.
- Above 100 failures for a username, unknown client IPs are delayed. Above 1000, unknown IPs are refused. A (username, IP) pair that logged in successfully in the last 30 days is not blocked by the username-wide count.
- A throttled login returns HTTP 429 with `Retry-After`. Invalid credentials return HTTP 401 with `remainingAttempts`.

Procedure:

1. **Stop retrying.** Each failed attempt extends the window. Wait 15 minutes from the last failure.
2. **Sign in from another address or as a config admin.** Config-defined admins (`api.auth.users`) are not stored in Redis, but they use the same throttling, per IP.
3. **Reset the managed user** as a config admin:

   ```bash
   curl -s -X PUT "${AUTH[@]}" $API/tutti/console-users/password -H 'Content-Type: application/json' \
     -d '{"username":"ops","generatePassword":true}'     # shown once; user must change it at next login
   ```

4. **Clear throttle counters only if you must** (keys are hashed, so you cannot pick one user by name). Counters live in the meta DB (`redis.metaDB`, default 0):

   ```bash
   redis-cli -n 0 --scan --pattern 'tutti:authfail:*' | xargs -r redis-cli -n 0 del
   ```

   This clears all counters for all users for the next 15 minutes of attempts. Do it only during an incident, and expect the brute-force protection to restart from zero.

5. **Delete a managed user record** (config DB, default 3) only if it is broken and cannot be reset. Direct Redis edits skip the API's last-admin checks:

   ```bash
   redis-cli -n 3 HDEL tutti:console-users ops
   ```

6. **If no admin can sign in at all**, add an admin under `api.auth.users` in the worker config and restart the worker:

   ```bash
   htpasswd -bnBC 12 "" '<new-password-at-least-12-chars>' | tr -d ':\n' | sed 's/^\$2y/$2b/'
   ```

   Put the output in `passwordHash`. The `$2y$` prefix that htpasswd produces is rewritten to `$2b$` to match the documented format; verify by signing in.

7. Afterwards, change the password, check `GET /tutti/console-users`, and make sure at least one config admin remains.

---

## (g) Add a storage at runtime, and add credentials

**Status: Available now** for managed storages with dynamic credentials. Config-defined storages still need a config change and restart.

Prerequisites: `storage.dynamicCredentials.enabled: true` and a `masterPassword` on the worker **and** all proxies (check `GET /tutti/storages` → `managedStoragesEnabled`).

1. **Test the storage and one credential before saving.** It does not save anything.

   ```bash
   curl -s -X POST "${AUTH[@]}" $API/tutti/storages/test-credential -H 'Content-Type: application/json' -d '{
     "storage": {"name": "archive", "address": "https://archive.example.internal:9000",
                 "provider": "Other", "isSecure": true, "httpTimeout": "60s", "healthCheckInterval": "10s"},
     "credential": {"user": "user1", "accessKeyId": "KEY", "secretAccessKey": "SECRET"}
   }'
   ```

   Endpoint URLs cannot contain credentials, query strings, fragments or a path. `provider` is `Ceph`, `Minio` or `Other`.

2. **Create the storage**, with credentials for the existing users you want on it:

   ```bash
   curl -s -X POST "${AUTH[@]}" $API/tutti/storages -H 'Content-Type: application/json' -d '{
     "storage": {"name": "archive", "address": "https://archive.example.internal:9000",
                 "provider": "Other", "isSecure": true, "httpTimeout": "60s", "healthCheckInterval": "10s"},
     "credentials": [{"user": "user1", "accessKeyId": "KEY", "secretAccessKey": "SECRET"}]
   }'
   ```

   The API verifies again on the server. Proxies and workers pick the change up within about 2 seconds under healthy Redis connectivity, without restart.

3. **Confirm propagation.** `chorctl proxies` lists the new storage on each proxy, and `GET /tutti/storages` shows it with `references: []`.

4. **Give each existing dynamic user credentials on the new storage**, then check `complete`:

   ```bash
   curl -s -X PUT "${AUTH[@]}" $API/tutti/users/credentials -H 'Content-Type: application/json' -d '{
     "user": "user1",
     "credential": {"storage": "archive", "accessKeyId": "KEY", "secretAccessKey": "SECRET"},
     "verify": true
   }'
   curl -s "${AUTH[@]}" $API/tutti/users | jq '.users[] | {name, complete}'
   ```

   Users whose credentials come from the config file cannot be changed through the API (`defined in config`). Add them to the config and restart.

5. **Test the stored credential**: `curl -s -X POST "${AUTH[@]}" $API/tutti/users/test -H 'Content-Type: application/json' -d '{"user":"user1","storage":"archive"}'`.

6. **Add replication and failover target if wanted.** A storage does nothing until replication exists. For failover, add it to a policy's `targets`:

   ```bash
   chorctl repl add -f main -t archive -u user1 [-b photos]
   curl -s -X POST "${AUTH[@]}" $API/tutti/failover/policies -H 'Content-Type: application/json' \
     -d '{"source":"main","enabled":true,"targets":["f1","archive"]}'
   ```

   Replacing a policy with `POST` overwrites the whole policy, so send the full target list.

Removing a storage: the API refuses deletion while anything references it (replication, routing, failover policy or event, or diff report), and the main storage can never be deleted. Remove the references first, then `PUT /tutti/storages/delete` with `{"name":"archive"}` and the name typed as confirmation in the UI.

---

## (h) Rotate the master password: not supported

**Status: Not supported.** Tutti has no command or API to change the dynamic credentials master password in place.

### Why

- The encryption key is derived from the master password and a salt stored in Redis (hash `chorus:dynamic_creds`, field `salt`, in the meta DB). A different password derives a different key.
- Sealed secrets stored with the old key cannot be decrypted with the new one. Dynamic user storage credentials are then unusable (expected behaviour; confirm the exact error in staging). Proxy accounts that cannot be decrypted are skipped, with the warning "skipped accounts that cannot be decrypted (wrong master password?)".
- No API returns stored secrets (`ListS3Users` and `GET /tutti/proxy-accounts` never return them). The only copy of the plaintext is the one you hold.
- Config-defined credentials (in YAML) do not use the master key and are not affected.

### Safe procedure (manual, rehearse in staging first)

There is no tested re-seal tool. The steps below are the only sequence the code supports.

1. **Keep the current password.** Changing it is the last resort.
2. **Back up Redis** (`tools/redis/backup-rdb.sh`) and store the current master password separately.
3. **Inventory the secrets you hold** for each dynamic user and storage. Plan to re-issue every proxy account: its secret cannot be recovered.
4. **Open a maintenance window and stop all workers and proxies.** Components must use one password at a time; mixed keys make reads fail.
5. **Set the new password on every worker and proxy** (same value, for example via `CFG_STORAGE_DYNAMICCREDENTIALS_MASTERPASSWORD`). Minimum length is 12 characters.
6. **Start one worker**, then re-write every dynamic user credential with `PUT /tutti/users/credentials` (it re-seals with the new key).
7. **Recreate proxy accounts** with `POST /tutti/proxy-accounts`, then delete the old, unreadable records (`PUT /tutti/proxy-accounts/delete`). Verify deletion works on a staging copy first. Give clients the new keys.
8. **Start the remaining components** and check `GET /tutti/proxy-accounts`, `GET /tutti/users` (`complete`), and a test request through the proxy.

If you cannot re-enter the secrets, the affected users and accounts cannot be used again, and recovery means creating new ones.

---

## See also

- [Storage accounts](./storage-accounts.md): preparing storage credentials.
- [Redis operations](./redis-operations.md): Redis keys, persistence, Sentinel, backup and restore.
- [Operations](./operations.md) and [API](./api.md).
