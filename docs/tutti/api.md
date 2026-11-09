# Resilience API, CLI, Redis keys and UI

Tutti adds the gRPC service `chorus.Resilience` to the worker management API (package name unchanged from Chorus, so existing clients stay wire compatible). Definition: [proto/chorus/tutti.proto](../../proto/chorus/tutti.proto); REST mapping: [proto/http.yaml](../../proto/http.yaml); handlers: `pkg/api/resilience_handlers.go`; generated OpenAPI: [proto/gen/openapi/chorus/chorus.swagger.json](../../proto/gen/openapi/chorus/chorus.swagger.json).

The API is served by the **worker** and requires `api.enabled: true` in the worker config (gRPC on `api.grpcPort`, default 9670; REST on `api.httpPort`, default 9671; see [service/worker/config.yaml](../../service/worker/config.yaml)). The examples below assume REST on `localhost:9671`.

REST JSON follows protobuf JSON mapping: camelCase field names, durations as strings (`"30s"`), timestamps as RFC 3339, and unset fields emitted with zero values (`null` for an unset optional).

## Service

| RPC | REST | Description |
|---|---|---|
| `GetProxyStatus(Empty) -> ProxyStatusList` | `GET /tutti/proxies` | Live proxy instances: breaker state per storage and effective settings |
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
      }
    }
  ]
}
```

Each proxy publishes its document every 10 s with a 30 s TTL; a proxy that stopped publishing is not listed. Values are the effective ones.

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
source `dynamic`. See [dynamic-creds.md](../dynamic-creds.md#runtime-managed-proxy-accounts).

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
| `tutti:proxy-status:<app_id>` | config DB (`redis.configDB`) | string (JSON), TTL 30 s | each proxy every 10 s | breaker state per storage, effective settings (`pkg/proxystatus`) |
| `tutti:failover-policies` | config DB | hash: field = source storage, value = JSON policy | Resilience API | `{source, enabled, targets, after, maxEventLag}`; `after` is stored in nanoseconds (Go `time.Duration` JSON) |
| `tutti:failover:user:<user>` | config DB | string (JSON) | failover controller | `{user, bucket, from, to, time, eventLag}` |
| `tutti:failover:bucket:<user>:<bucket>` | config DB | string (JSON) | failover controller | same, for a bucket scope |
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
