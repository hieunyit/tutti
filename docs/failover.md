# Automatic failover (optional, off by default)

## Enable
Proxy YAML (global safety switch and timing only):

```yaml
failover:
  enabled: false     # true starts the controller in every proxy
  checkInterval: 10s
  after: 60s         # default for policies that omit `after`
  maxEventLag: 0     # default for policies that omit `maxEventLag`
```

## Failover policy (per source storage, in Redis config DB)
`{source, enabled, targets: [ordered], after, maxEventLag}` (Go API: `pkg/failover.Store`:
`Set/Get/List/DeleteFailoverPolicy`, `ListFailoverEvents`, `DeleteFailoverEvent`; stored in hash `tutti:failover-policies`).
A source without an enabled policy with targets never fails over. Only listed targets may be promoted; the first
target (operator order) that passes all checks wins.

A target qualifies when: there is a replication source->target (bucket-level for bucket scopes, else user-level) with
the same bucket name; the target breaker is closed; initial migration is done; the replication is not paused/archived;
no replication switch is in progress; unreplicated events (queued event tasks) <= maxEventLag.

## What happens
The source breaker must be continuously open for `after`. Under a Redis lock (one proxy acts, conditions re-checked)
each user/bucket routed to the source (explicit routing or implicit main) is re-routed to the target and the replication
source->target is paused. An event is stored under `tutti:failover:user:<user>` / `tutti:failover:bucket:<user>:<bucket>`
(`redis-cli --scan --pattern 'tutti:failover:*'`, `redis-cli get <key>`), logged at warn and counted in
`failover_total{from,to}`. If nothing qualifies a warning with the reason is logged once per scope per outage.
Routing is read from Redis on every request (no cache), so all proxies follow immediately.
Requests routed to the target ignore replications that point at the target itself, so synchronous/async replication
does not try to copy to the paused/failed side.

## Manual failback (never automatic)
1. Repair the old storage. 2. Delete the paused old->new replication. 3. Create replication new->old and let it catch up.
4. Switch back with the replication switch APIs. 5. Delete the failover event (`DeleteFailoverEvent` or `redis-cli del`).
