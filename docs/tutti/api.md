# Resilience API, CLI, Redis keys and UI

Tutti adds the gRPC service `chorus.Resilience` to the worker management API (package name unchanged from Chorus, so existing clients stay wire compatible). Definition: [proto/chorus/tutti.proto](../../proto/chorus/tutti.proto); REST mapping: [proto/http.yaml](../../proto/http.yaml); handlers: `pkg/api/resilience_handlers.go`; generated OpenAPI: [proto/gen/openapi/chorus/chorus.swagger.json](../../proto/gen/openapi/chorus/chorus.swagger.json).

The API is served by the **worker** and requires `api.enabled: true` in the worker config (gRPC on `api.grpcPort`, default 9670; REST on `api.httpPort`, default 9671; see [service/worker/config.yaml](../../service/worker/config.yaml)). The examples below assume REST on `localhost:9671`.

REST JSON follows protobuf JSON mapping: camelCase field names, durations as strings (`"30s"`), timestamps as RFC 3339, and unset fields emitted with zero values (`null` for an unset optional).

## Service

| RPC | REST | Description |
|---|---|---|
| `GetProxyStatus(Empty) -> ProxyStatusList` | `GET /tutti/proxies` | Live proxy instances: breaker state per storage and effective settings |
| `GetStatsHistory(StatsHistoryRequest) -> StatsHistory` | `GET /tutti/stats/history?range=5m\|15m\|1h\|6h` | Traffic rates over time, summed across proxies (viewer-readable) |
| `ListFailoverPolicies(Empty) -> FailoverPolicyList` | `GET /tutti/failover/policies` | All policies sorted by source |
| `SetFailoverPolicy(FailoverPolicy) -> Empty` | `POST /tutti/failover/policies` | Create or replace the policy of a source |
| `DeleteFailoverPolicy(FailoverPolicySource) -> Empty` | `PUT /tutti/failover/policies/delete` | Remove a policy (not found if missing) |
| `ListFailoverEvents(Empty) -> FailoverEventList` | `GET /tutti/failover/events` | Performed failovers not yet cleared |
| `DeleteFailoverEvent(FailoverEventID) -> Empty` | `PUT /tutti/failover/events/delete` | Clear the record of a failover (not found if missing) |

Errors: invalid arguments (unknown storage, enabled policy without targets, target equal to source, duplicate or empty target, negative `after`/`maxEventLag`, missing `source`/`user`) are rejected as invalid argument; deleting a missing policy or event returns not found.

## Messages and examples

### GetProxyStatus

```shell
curl -s http://localhost:9671/tutti/proxies
```

```json
{
  "proxies": [
    {
      "appId": "3f6c0a52-...",
      "address": "http://localhost:9669",
      "startedAt": "2026-11-09T10:00:00Z",
      "updatedAt": "2026-11-09T10:05:10Z",
      "storages": [
        {
          "name": "main",
          "available": false,
          "consecutiveFailures": 5,
          "lastError": "dial tcp 10.0.0.5:9000: i/o timeout",
          "lastChange": "2026-11-09T10:04:02Z"
        },
        { "name": "f1", "available": true, "consecutiveFailures": 0, "lastError": "", "lastChange": "2026-11-09T10:00:00Z" }
      ],
      "settings": {
        "replication": {
          "onEventError": "fail",
          "eventRetryBudget": "2s",
          "sync": { "enabled": true, "writeQuorum": 0, "timeout": "30s", "onQuorumFailure": "fail" }
        },
        "health": { "failureThreshold": 3, "openDuration": "30s" },
        "failover": { "enabled": true, "checkInterval": "10s", "after": "60s", "maxEventLag": 0 },
        "requireSignedContentType": false
      },
      "rates": {
        "windowSeconds": 10,
        "requestsPerSec": 42.5,
        "requestsByClass": { "read": 30.1, "write": 12.4 },
        "requestsByStorage": { "main": 40, "f1": 2.5 },
        "bytesInPerSec": 1048576, "bytesOutPerSec": 524288,
        "status4xxPerSec": 0.3, "status5xxPerSec": 0, "failFastPerSec": 0, "accessDeniedPerSec": 0,
        "latencyAvgMs": 12.4, "latencyP95Ms": 48.1
      },
      "stats": { "...": "cumulative counters since startedAt (unchanged)" }
    }
  ],
  "totalRates": { "requestsPerSec": 42.5, "...": "rates summed across the listed proxies" }
}
```

Each proxy publishes its document every `status.interval` (default 2 s, see [configuration](./configuration.md#status)) with a TTL of 3 intervals; a proxy that stopped publishing is not listed. Values are the effective ones. That is one Redis `SET` of a few KB per interval per proxy.

`rates` are computed by the proxy itself from its cumulative counters over a sliding window of about 10 s (samples every second): no client-side diffing needed, and the values are available ~2 s after proxy start (`windowSeconds` is 0 until two samples exist). Latencies are in milliseconds; `latencyP95Ms` is estimated from histogram bucket deltas (linear interpolation, capped at the last bound, 10 s). `totalRates` sums the proxies: `latencyAvgMs` is request-weighted, `latencyP95Ms` is the maximum over proxies (an upper bound; percentiles cannot be merged exactly). `stats` (cumulative) is kept for compatibility.

Failover readiness: `settings.failover` carries per proxy whether the failover controller runs (`enabled`), its `checkInterval`, `after` and `maxEventLag`. Policies only trigger on proxies with `enabled: true`; the UI uses this to warn "policies will not trigger".

### GetStatsHistory

```shell
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:9671/tutti/stats/history?range=15m"
```

```json
{
  "range": "15m",
  "bucket": "10s",
  "points": [
    { "time": "2026-11-10T04:00:00Z", "rates": { "requestsPerSec": 42.5, "latencyAvgMs": 12.4, "latencyP95Ms": 48.1, "...": "same fields as proxy rates" } }
  ],
  "proxies": [
    { "appId": "3f6c0a52-...", "time": "2026-11-10T04:00:07Z", "rates": { "requestsPerSec": 42.5 } }
  ]
}
```

`range` is `5m`, `15m` (default), `1h` or `6h`; anything else is an invalid argument. Bucket size: 10 s for 5m/15m, 30 s for 1h, 2 min for 6h. Each proxy appends a compact sample (its 10 s-window rates) every 10 s to the Redis list `tutti:proxy-stats-history:<app_id>` (LPUSH + LTRIM to 2160 entries = 6 h, EXPIRE ~6 h, so history of a dead proxy vanishes after ~6 h); the endpoint reads them, so history survives UI reloads. In a downsampled bucket the samples of one proxy are averaged first, then proxies are summed. Aggregation rules: rates are summed across proxies, `latencyAvgMs` is the request-weighted average, `latencyP95Ms` is the maximum across proxies and samples (upper bound). Buckets without any sample are omitted (gaps mean no proxy reported). `proxies` holds the latest sample of each proxy within the range.

### Policies

`FailoverPolicy`: `source` (string, required), `enabled` (bool), `targets` (ordered string list), `after` (duration, unset/0 = proxy default), `maxEventLag` (optional int, unset = proxy default).

```shell
# create or replace
curl -s -X POST http://localhost:9671/tutti/failover/policies \
  -H 'Content-Type: application/json' \
  -d '{"source":"main","enabled":true,"targets":["f1","f2"],"after":"60s","maxEventLag":0}'

# list
curl -s http://localhost:9671/tutti/failover/policies
# {"policies":[{"source":"main","enabled":true,"targets":["f1","f2"],"after":"60s","maxEventLag":0}]}

# delete
curl -s -X PUT http://localhost:9671/tutti/failover/policies/delete \
  -H 'Content-Type: application/json' -d '{"source":"main"}'
```

A policy is effective only when the proxies run with `failover.enabled: true` (see [configuration](./configuration.md#automatic-failover)). `source` and `targets` must be storages configured in the worker.

### Events

`FailoverEvent`: `user`, `bucket` (empty for user-level), `from`, `to`, `time`, `eventLag`.

```shell
curl -s http://localhost:9671/tutti/failover/events
# {"events":[{"user":"user1","bucket":"","from":"main","to":"f1","time":"2026-11-09T10:06:00Z","eventLag":0}]}

# acknowledge after manual failback (bucket "" for a user-level event)
curl -s -X PUT http://localhost:9671/tutti/failover/events/delete \
  -H 'Content-Type: application/json' -d '{"user":"user1","bucket":""}'
```

Clearing an event does not change routing; it only removes the record.

### Atomic routing update

`chorus.Policy/UpdateRouting` (`PUT /routing/update`, admin only) changes the target storage of an existing user or
bucket routing in one Redis script, so requests never fall back to the main storage in between (unlike delete + add).
It validates like `AddRouting` (target storage and user exist, no conflicting replication), returns `NotFound` if the
routing does not exist, and keeps a routing block untouched.

```bash
curl -X PUT $API/routing/update -d '{"user":"user1","bucket":"photos","toStorage":"f1"}'
```

### Dynamic proxy accounts

`CreateProxyAccount`, `RotateProxyAccountSecret`, `SetProxyAccountStatus`, `DeleteProxyAccount` (all admin only,
`/tutti/proxy-accounts[/rotate|/status|/delete]`) manage runtime proxy accounts; `ListProxyAccounts` shows them with
source `dynamic`. `ListProxyAccounts` also returns `dynamicAccounts: { "available": bool, "reason": string }`: `available` is true only if `storage.dynamicCredentials` is enabled with a `masterPassword` and encryption on (otherwise `CreateProxyAccount` fails with failed precondition and `reason` says why); the UI uses it to disable the create form. Viewer-readable. See [dynamic-creds.md](../dynamic-creds.md#runtime-managed-proxy-accounts).

## S3 users API

An S3 user (for example `user1`) is an identity that has credentials on each storage (from `storage.storages.<name>.credentials` in the worker config, or dynamic credentials), plus optional proxy access keys and an access policy. All RPCs are on `chorus.Resilience`; `ListS3Users` is viewer-readable, everything else is admin only. Secrets are never returned except the secret of a newly created access key, once.

| RPC | REST | Description |
|---|---|---|
| `ListS3Users(Empty) -> S3UserList` | `GET /tutti/users` | Union of the users of all storages |
| `CreateS3User(CreateS3UserRequest) -> CreateS3UserResponse` | `POST /tutti/users` | Create a user with dynamic credentials |
| `UpdateS3UserCredentials(UpdateS3UserCredentialsRequest) -> Empty` | `PUT /tutti/users/credentials` | Replace the credentials on one storage |
| `DeleteS3User(DeleteS3UserRequest) -> DeleteS3UserResponse` | `PUT /tutti/users/delete` | Remove dynamic credentials, dynamic access keys and policy |
| `TestS3UserCredentials(TestS3UserCredentialsRequest) -> TestS3UserCredentialsResponse` | `POST /tutti/users/test` | `ListBuckets` with given or stored credentials |

```bash
# list (no secrets)
curl $API/tutti/users
# {"users":[{"name":"user1",
#    "storages":[{"storage":"main","configured":true,"source":"config","accessKeyId":"testKey1"},
#                {"storage":"f1","configured":true,"source":"dynamic","accessKeyId":"AKIA..."}],
#    "complete":true,
#    "accessKeys":{"static":1,"dynamicActive":1,"dynamicDisabled":0},
#    "policy":{"hasPolicy":true,"permission":"read-only","buckets":["photos-*"]}}],
#  "mainStorage":"main","storages":["f1","main"],"dynamicCredentialsEnabled":true}

# create, verify credentials first, create the first proxy access key, set a policy
curl -X POST $API/tutti/users -d '{"name":"user3",
  "credentials":[{"storage":"main","accessKeyId":"minioadmin","secretAccessKey":"minioadmin"},
                 {"storage":"f1","accessKeyId":"minioadmin","secretAccessKey":"minioadmin"}],
  "verify":true,"createAccessKey":true,"description":"first key",
  "policy":{"permission":"read-write","buckets":["*"]}}'
# {"user":{...},"accessKey":{"accessKeyId":"TUT...","secretAccessKey":"...","previousSecretExpiresAt":null},
#  "verification":[{"storage":"main","ok":true,"error":"","latencyMs":"7"}, ...]}

# replace the credentials on one storage; test stored or given credentials
curl -X PUT  $API/tutti/users/credentials -d '{"user":"user3","credential":{"storage":"f1","accessKeyId":"k","secretAccessKey":"s"},"verify":true}'
curl -X POST $API/tutti/users/test -d '{"user":"user3","storage":"f1"}'     # stored credentials
# {"ok":true,"error":"","latencyMs":"6"}

# delete
curl -X PUT $API/tutti/users/delete -d '{"name":"user3","force":true}'
# {"deletedStorages":["f1","main"],"deletedAccessKeys":1,"deletedPolicy":true}
```

- `complete` is true when the user has credentials on every storage (needed for replication and failover). `source` is `config` or `dynamic`. Only S3 storages are supported for create and update.
- User names: 1-64 characters, letters, digits, `.`, `_`, `-`, starting with a letter or digit. Existing users (on any storage, from config or dynamic) are rejected with already exists.
- `CreateS3User` requires dynamic credentials (`storage.dynamicCredentials.enabled` with a `masterPassword`), otherwise failed precondition with the reason. The main storage credentials are required. With `verify: true` every credential is checked with `ListBuckets` before anything is saved; failures are returned as invalid argument listing each storage and error. Order of steps: verify, credentials, policy, access key; a failing step rolls back the previous ones (best effort). `createAccessKey` needs the sealing key (same requirement as `CreateProxyAccount`) and is checked up front.
- `UpdateS3UserCredentials` fails with failed precondition `defined in config` for config-sourced credentials.
- `DeleteS3User` is refused with failed precondition, listing every reason, if: credentials come from the worker config (never deleted by the API, `force` does not help); replications, routings or failover events reference the user (`force` does not help, delete them first); or the user has active proxy access keys (only this is overridden by `force`; disabled keys never block). Deletion order: access keys, access policy, then credentials, so a retry after a partial failure finishes the job. Proxies and workers drop deleted credentials on their next poll (`storage.dynamicCredentials.pollInterval`); the worker handling the request drops them immediately.
- Failure examples: `{"code":9,...,"details":[{"reason":"FailedPrecondition: s3 user \"user3\" cannot be deleted: 1 replication(s) exist (main->f1): delete them first"}]}`.

## Console users API

Console users sign in to the management API and UI. Users defined in the worker config (`api.auth.users`) are read-only; managed users are stored in Redis and managed here. All RPCs are admin only and fail with failed precondition (`sign-in is disabled`) when `api.auth.enabled` is false.

| RPC | REST | Description |
|---|---|---|
| `ListConsoleUsers(Empty) -> ConsoleUserList` | `GET /tutti/console-users` | Config and managed users, never hashes |
| `CreateConsoleUser(CreateConsoleUserRequest) -> CreateConsoleUserResponse` | `POST /tutti/console-users` | `username`, `role`, `password` or `generatePassword` |
| `UpdateConsoleUser(UpdateConsoleUserRequest) -> Empty` | `PUT /tutti/console-users` | `role`, `disabled` (unset keeps) |
| `ResetConsoleUserPassword(ResetConsoleUserPasswordRequest) -> ResetConsoleUserPasswordResponse` | `PUT /tutti/console-users/password` | Set or generate a password, `mustChangePassword`, revokes sessions |
| `DeleteConsoleUser(ConsoleUserName) -> Empty` | `PUT /tutti/console-users/delete` | Delete a managed user |
| (HTTP only) | `POST /auth/password` | Any signed-in managed user: `{"currentPassword","newPassword"}` -> 204 |
| (HTTP only) | `GET /auth/me` | Now also returns `source` (`config`/`managed`) and `mustChangePassword` |

```bash
curl -X POST $API/tutti/console-users -d '{"username":"ops","role":"viewer","generatePassword":true}'
# {"user":{"username":"ops","role":"viewer","source":"managed","disabled":false,"mustChangePassword":true,
#          "createdAt":"...","updatedAt":"...","lastLoginAt":null,"passwordChangedAt":"..."},
#  "generatedPassword":"..."}            # returned once
curl -X PUT $API/tutti/console-users -d '{"username":"ops","disabled":true}'
curl -X PUT $API/tutti/console-users/password -d '{"username":"ops","generatePassword":true}'
curl -X PUT $API/tutti/console-users/delete -d '{"username":"ops"}'
curl -X POST $API/auth/password -H "Authorization: Bearer $TOKEN" -d '{"currentPassword":"...","newPassword":"at-least-12-chars"}'
```

Rules:

- Password policy: 12-72 bytes (72 is the bcrypt limit), not equal to the username. Hashed with bcrypt cost 12.
- Roles: `admin` or `viewer`. Usernames: 1-64 characters, letters, digits, `.`, `_`, `@`, `-`. A name defined in the config cannot be created (already exists); if a stored record has a config user's name, config wins.
- You cannot delete, disable or demote yourself. The last enabled admin cannot be removed, disabled or demoted (config admins count as enabled admins). Config users cannot be modified (failed precondition `defined in config`).
- Sessions are re-resolved on every request: a disabled or deleted user is rejected immediately, a role change applies immediately, and a password change or reset invalidates all sessions created before it (`POST /auth/password` keeps the calling session). `mustChangePassword` is informational for the UI; it is not enforced server side.
- Login throttling (per user+IP and per user) also applies to the current-password check of `POST /auth/password`.
- `lastLoginAt` is recorded best effort.
- Viewers can call `ListS3Users` only; the viewer allow-list in `pkg/auth/interceptor.go` fails closed for every other new RPC.

## chorctl

`chorctl` (see [tools/chorctl](../../tools/chorctl)) talks to the worker REST API (`-a/--address`, default `http://localhost:9671`). Tutti adds read-only commands:

```shell
chorctl proxies               # APP ID, ADDRESS, UPDATED, STORAGES (name=up|DOWN), SYNC, FAILOVER
chorctl failover policies     # SOURCE, ENABLED, TARGETS, AFTER, MAX EVENT LAG ("default" when unset)
chorctl failover events       # USER, BUCKET, FROM, TO, TIME, EVENT LAG
```

Policies and events are written via REST/gRPC or the UI; `chorctl` has no set/delete command for them. Replication and routing commands (`chorctl repl ...`, `chorctl route ...`) are unchanged.

## Redis keys

| Key | DB | Type | Written by | Content |
|---|---|---|---|---|
| `tutti:proxy-status:<app_id>` | config DB (`redis.configDB`) | string (JSON), TTL 3x `status.interval` | each proxy every `status.interval` (2 s) | breaker state per storage, effective settings, cumulative stats, current rates (`pkg/proxystatus`) |
| `tutti:proxy-stats-history:<app_id>` | config DB | list (JSON samples, newest first), max 2160, TTL ~6 h | each proxy every 10 s | `{t, r: rates}` samples for `GetStatsHistory` |
| `tutti:failover-policies` | config DB | hash: field = source storage, value = JSON policy | Resilience API | `{source, enabled, targets, after, maxEventLag}`; `after` is stored in nanoseconds (Go `time.Duration` JSON) |
| `tutti:failover:user:<user>` | config DB | string (JSON) | failover controller | `{user, bucket, from, to, time, eventLag}` |
| `tutti:failover:bucket:<user>:<bucket>` | config DB | string (JSON) | failover controller | same, for a bucket scope |
| `tutti:console-users` | config DB | hash: field = username, value = JSON record | Console users API | `{username, role, passwordHash (bcrypt), disabled, createdAt, updatedAt, passwordChangedAt, mustChangePassword}` |
| `tutti:console-users:last-login` | config DB | hash: field = username, value = RFC 3339 | worker at login | last login time (best effort) |
| `tutti:failover-lock:<storage>` | lock DB (`redis.lockDB`) | lock, TTL 30 s | failover controller | held by the one proxy performing the failover |

Inspect: `redis-cli --scan --pattern 'tutti:*'`, `redis-cli get tutti:failover:user:user1`, `redis-cli hgetall tutti:failover-policies`. Prefer the API for changes to policies.

## Metrics

Exposed when `metrics.enabled` is true (Prometheus, same endpoint as the other proxy metrics):

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `storage_available` | gauge | `storage` | 1 breaker closed, 0 open |
| `storage_breaker_transitions_total` | counter | `storage`, `to` (`open`/`closed`) | breaker transitions |
| `failover_total` | counter | `from`, `to` | automatic failovers (one per scope) |

Source: `pkg/metrics/service.go`.

## UI

The web UI ([ui/](../../ui)) has two Tutti pages, backed by the REST paths above:

- `/health` (Storage health): one card per live proxy with each storage's availability, consecutive failures, last error and last change, plus the effective settings (sync, write quorum, timeout, failover, health thresholds, signed Content-Type). Refreshes automatically every 10 s.
- `/failover`: failover policy table with an editor (source, ordered targets with add/remove/reorder, after, max event lag, enabled; client-side validation mirrors the API) and the list of failover events with an acknowledge action. Shows a warning when some proxies run with `failover.enabled` off.

Source: `ui/src/pages/Tutti*`, `ui/src/services/TuttiService.ts`, `ui/src/router/routes.ts`.
