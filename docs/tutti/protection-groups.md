# Protection groups

A protection group keeps one S3 scope (a user, or an explicit list of that user's buckets) on several
storages. One member is the **primary**: routing points to it and the group maintains replications from it
to every other member. When the primary fails, or on an operator switchover, the group promotes another
member and re-points the replications, so the copy count is restored instead of dropping to one.

Implementation: `pkg/protectiongroups` (model, store, controller, write fence, quorum, resync),
`pkg/policy/protection_groups.go` (routing/replication backend), API `proto/chorus/protection_groups.proto`.
The controller runs in every worker (one reconcile per group per second, serialized by a Redis lock per group).

## Model

| Field | Meaning |
|---|---|
| `id`, `user` | Group ID and the Tutti user whose data it protects |
| `buckets` | Exact bucket names; empty = all buckets of the user. Scopes of two groups may not overlap |
| `members` | Storages (S3, at least two). The primary must currently be the routing target of the scope |
| `copies` | Wanted healthy copies (2..members) |
| `mode` | `manual` (switchover only) or `auto` (automatic failover by quorum) |
| `after`, `electorate`, `maxEventLag`, `maxSwitchesPerHour` | Automatic failover: outage duration, explicit proxy voters (at least three), allowed event lag, rate limit |
| `primary`, `generation`, `state`, `frozen`, `target`, `verified`, `quarantined`, `history` | Server-owned state |

Scope and members are immutable; detach (`Delete`, ready groups only) and create again to change them.

## States

```
initializing --(all available members initialized, lag 0)--> freeze --> promoting (resync) --> ready
ready --(manual switchover)--> waiting --(target initialized)--> draining (frozen) --> promoting --> initializing (new primary)
ready --(auto: quorum says primary down)--> draining (frozen) --> promoting --> initializing (new primary)
ready --(recovered member caught up but not verified)--> initializing (frozen) --> promoting --> ready
```

* **Catch-up before freeze.** In `initializing` the bulk initial migration and the event backlog run while
  writes are admitted (to the primary only; replication captures them). The group freezes only once every
  available, non-quarantined member is initialized with zero lag. The frozen window is then the drain of
  in-flight writes plus the final resync of members that are already caught up.
* **Resync** (`S3Resyncer`) makes a member exactly equal to the primary under the freeze: one merge-join pass
  over both bucket listings. Per key it compares size, then the ETag when it decides equality (equal MD5-form
  ETags; two different single-part ETags), and reads both sides (SHA-256, once each) only when inconclusive,
  for example multipart objects with different part layouts. Copies are verified from the digest streamed during
  the copy. Target-only objects are deleted only after both listings were consumed in ascending order. For a
  caught-up member the cost is two listings. Versioned buckets are refused. Object tags, ACLs and bucket
  configuration are not reconciled.
* **Automatic failover** needs a strict majority of the explicit electorate to report the primary down for
  `after` (fresh reports only; missing voters never shrink the denominator), the global automatic freeze off
  (`SetAutomaticFreeze`), and the hourly limit not reached. A promotion with event lag > 0 quarantines every
  other member: quarantined members are never overwritten automatically.
* After a promotion the generation increases; Commit replaces routing, removes the old replications and bumps
  their replication generations in one Redis transaction, so stale replication tasks are dropped.

## Write fence

Every proxy write (any method but GET/HEAD) and every worker replication task of a protected scope passes the
fence (`Store.Middleware`, `Store.WorkerMiddleware`), one Lua script per write:

1. `tutti:protection:scope` (hash: `u:<user>` / `b:<user>/<bucket>` → group ID, plus a `_v` marker) finds the
   group with one HMGET.
   The index is rebuilt from the group documents in the same transaction as every group create/delete; if it is
   missing (groups written by an older version) admission rebuilds it before deciding, never admits unfenced.
2. The group document is read and decoded (only this one); the write is refused (503) when the group is frozen
   or the request was routed to a storage that is not the current primary (stale routing view).
3. A writer record `<owner>:<generation>:<instance>:<admittedUnixMs>` is added to
   `tutti:protection:writers:<group>` (`:workers` for workers) and the process lease is refreshed.

Redis commands per write, independent of the number of groups (`TestAdmitRedisCommandsDoNotGrowWithGroups`):
protected scope EVALSHA + 4 (HMGET, HGET, HSET, SET); unprotected scope EVALSHA + 1 (HMGET). A Redis outage fails writes closed, including unprotected scopes.

### Writer leases and crashed proxies

A frozen group promotes only when no writer record remains. Records are removed when the write returns; a
failed removal (Redis blip) is retried by the lease loop. To avoid a crashed process blocking promotion forever:

* Each proxy/worker holds a lease `tutti:protection:lease:<proxy|worker>:<app_id>` = random instance nonce,
  TTL `protectionGroups.writerLease` (proxy config, default 30s; workers use 30s), refreshed every TTL/6.
* The controller reaps a record only when its lease is expired (or held by a newer instance of the same ID)
  **and** the proxy is absent from fresh status reports, or its fresh report shows a process start after the
  write was admitted (it restarted). A slow but alive proxy keeps its lease and keeps blocking promotion.
* Self-fencing: a process that cannot refresh its lease for TTL/2 cancels its in-flight protected writes,
  before the lease can expire and the record be reaped. Workers have no status reports; their lease alone
  decides, with the same self-fencing.
* Bound: after a proxy crash, promotion waits at most `writerLease` + status expiry (3 × `status.interval`,
  and the 10s report freshness window) + one reconcile interval. With an alive proxy the wait is bounded only
  by its slowest in-flight write (the proxy has no write timeout for large uploads).
* Records written by an older version (`<owner>:<generation>`) are never reaped automatically.

Operator override: `ListWriters` (`GET /tutti/protection-groups/{id}/writers`) and `ClearWriter`
(`POST /tutti/protection-groups/clear-writer`, requires a frozen group and `externallyFenced: true`, attesting
that the process and its backend writes are stopped).

## API

| RPC | REST |
|---|---|
| `List` / `Get` (status with members, healthy copies, active writers) | `GET /tutti/protection-groups`, `GET /tutti/protection-groups/{id}` |
| `Create` (`adoptExisting` adopts matching replications) | `POST /tutti/protection-groups` |
| `Update` (copies, mode, after, maxEventLag, electorate, maxSwitchesPerHour) | `PUT /tutti/protection-groups` |
| `Delete` (ready groups only) | `POST /tutti/protection-groups/delete` |
| `Switchover` / `AbortSwitch` | `POST /tutti/protection-groups/switchover`, `.../abort-switch` |
| `SetAutomaticFreeze` / `GetAutomaticFreeze` | `PUT /tutti/protection-groups/automatic-freeze`, `GET .../automatic-freeze/status` |
| `ListWriters` / `ClearWriter` | see above |

Replications, routings and failover policies inside a group's scope can only be changed through the group.

## Limits and open points

* While a group is `initializing` (after creation or a promotion, until every available member caught up)
  automatic failover is not evaluated.
* A frozen group waits for writes in flight to a blackholed primary until the storage HTTP timeout ends them.
* Redis Cluster is unsupported (the fence script touches keys derived from the group ID).
* Equal MD5-form ETags are trusted as equal content: a tenant able to craft MD5 collisions could keep a
  divergent object on a replica of its own data.
* e2e: `test/resilience/protection_groups_test.go` (three MinIO storages, three-proxy electorate: failover with
  copies restored; promotion completes after a proxy dies mid-write).
