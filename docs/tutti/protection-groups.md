# Protection groups

A protection group keeps one S3 scope (a user, or an explicit list of that user's buckets) on several
storages. One member is the **primary**: routing points to it and the group maintains replications from it
to every other member. When the primary fails, or on an operator switchover, the group promotes another
member and re-points the replications, so the copy count is restored instead of dropping to one.

Implementation: `pkg/protectiongroups` (model, store, controller, write fence, quorum, resync),
`pkg/policy/protection_groups.go` (routing/replication backend), API `proto/tutti/protection_groups.proto`.
The controller runs in every worker (one reconcile per group per second, serialized by a Redis lock per group).

## Model

| Field | Meaning |
|---|---|
| `id`, `user` | Group ID and the Tutti user whose data it protects. `user` is optional: empty means a storage-level group (every bucket of the members, connection accounts, no identity); if set it must exist (an identity, or a user with credentials on a member) and is never chosen implicitly |
| `buckets` | Exact bucket names; empty = all buckets of the user (or of the storage when `user` is empty). Scopes of two groups may not overlap |
| `members` | Storages (S3, at least two). The primary must currently be the routing target of the scope |
| `copies` | Wanted healthy copies (2..members) |
| `mode` | `manual` (switchover only) or `auto` (automatic failover by quorum) |
| `after`, `electorate`, `maxEventLag`, `maxSwitchesPerHour` | Automatic failover: outage duration, explicit proxy voters (at least three), allowed event lag, rate limit |
| `primary`, `generation`, `state`, `frozen`, `target`, `verified`, `quarantined`, `history` | Server-owned state |

A group shares the overlap rule with replications (see [API](../tutti/api.md), "Storage-level replication"); a group over members {A, B, C} counts every ordered pair, so it conflicts with a standalone `C -> A`, and (storage-level, all buckets) with a standalone all-buckets `A -> D` because the group can switch its source, but not with `A -> D` on explicit buckets while the group covers all buckets:

| Case | Example | Result |
|---|---|---|
| Same destination | `A -> C` exists, add `B -> C` (or `A -> C` again) | rejected |
| Reverse direction | `A -> B` exists, add `B -> A` | rejected |
| Different source storage (storage-level only) | `A -> B` on bucket `b1` exists, add `C -> D` on `b1`; or `A -> B` and `C -> D` both all-buckets | rejected: policy stores keep one source per bucket |
| Same source, different destination | `A -> B` exists, add `A -> C` | allowed |
| Chain, unrelated pairs | `A -> B` and `B -> C` (or `C -> D`) with disjoint buckets, or one all-buckets and one explicit list | allowed |

The different-source row applies only to two storage-level objects (empty user) whose explicit bucket lists intersect or that are both all-buckets (a bucket policy takes precedence over an all-buckets one in the proxy, so all-vs-explicit is fine); a chain on the same bucket is therefore also rejected. The error text is `storage-level replications must share one source storage per bucket: bucket "b1" is already replicated from "A" by <kind> "<id>"`. Other rejections apply only when the bucket sets also intersect: an explicit bucket list intersects when at least one name is shared, and "all buckets" intersects everything. Objects of two different real users conflict only when their explicit bucket lists intersect, because "all buckets" of a user means all buckets of that user. A storage-level object (no user) covers every bucket of the storage, so it conflicts with any object on the same edges. A protection group contributes every ordered pair between its members. The check runs before anything is created, under one global lock, so a rejected request leaves no policy, key, group or queue entry behind and two concurrent creations cannot both succeed.

Creating or updating a group that would overlap fails with `AlreadyExists` (HTTP 409) naming the object and bucket, and creates nothing. One source to many destinations is several replications or one group. The group's internal replications run without a user, using each member's connection account; the former internal `$service` identity was removed.

Scope and members are immutable; detach (`Delete`, ready groups only) and create again to change them.

A storage is managed either by protection groups or by legacy failover policies, never both: creating or
enabling a legacy failover policy whose source or a target is a group member is refused, and so is creating a
group over a storage used by an enabled legacy policy. The legacy failover controller additionally skips every
scope a group manages (resolved from the version-checked snapshot or from Redis) and every scope whose
protection cannot be determined.

### Users on a shared connection account

A group never extends to other identities. A user whose requests to a storage use its shared connection account
(credential mode `connection`) always keeps its own routing and replications. When that user's routed storage
is a member of another user's bucket group and the bucket is one of the group's buckets (the same physical
bucket), its writes are refused with 403 (reads are allowed): such writes would bypass the group's fence,
replication and promotion. Whole-user groups of other users never apply (their bucket set is not known to the
proxy). An unreadable scope index fails the request (never "unprotected"). The worker applies the same rule (same
credential-mode predicate) to bucket replications of other identities whose destination is such a bucket: their
tasks are refused without retry.

### PostgreSQL control plane

Redis is authoritative for every group it holds and is written first (WATCH/CAS). PostgreSQL receives the spec
plus a recovery seed (primary, generation, quarantined members, history) afterwards, always stored recovery-safe
(`frozen`, `initializing`, no verified/seeded marks). A failed PostgreSQL write is retried by the next read
(`Store.Get`/`List` re-push a differing copy); a promotion's routing/topology change is recorded in
`tutti:protection:cp-commit-pending` in the promotion's Redis transaction and applied to PostgreSQL by the
reconciler. Deletes tombstone the group in `tutti:protection:deleted`, so a leftover PostgreSQL row never
resurrects it. A group missing from Redis but present in PostgreSQL (Redis lost) is restored frozen and
re-initializing (re-seeded and re-verified before writes resume); `Store.Recover` does this for all groups.
With `controlPlane.readOnly`, spec changes (create, update, delete) are refused; promotions continue.

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
fence (`Store.Middleware`, `Store.WorkerMiddleware`).

The scope index `tutti:protection:scope` (hash: `u:<user>` / `b:<user>/<bucket>` → group ID,
`s:<storage>/<bucket>` → group ID for every member of a bucket group, `o:<group>` → user, plus a `_v` format
marker, currently `2`; an older format is rebuilt by `EnsureIndex`) is rebuilt from the group documents in the same transaction as every group create/delete. When its
content changes, that transaction also bumps `tutti:cfg:version` and publishes `tutti:cfg:changed`.

* **Unprotected scope (proxy, policy snapshot enabled):** the proxy snapshot caches the index per config version
  (one HGETALL after each change). Every write reads the version before using the snapshot, so a write after a
  group create/delete always sees the new index. Such writes are admitted from memory: no Redis command, and no
  503 when Lua/Redis scripting fails (so `replication.onEventError: log` behaves as without groups).
* **Protected scope, snapshot disabled, or a cached index without `_v`:** one Lua script, fail-closed:
  1. one HMGET on the index; a missing index is rebuilt before deciding (never admits unfenced);
  2. the group document is read (only this one); the write is refused (503) when the group is frozen or the
     request was routed to a storage that is not the current primary (stale routing view);
  3. a writer record `<owner>:<generation>:<instance>:<admittedUnixMs>` is added to
     `tutti:protection:writers:<group>` (`:workers` for workers) and the process lease is refreshed.

Redis commands per write, independent of the number of groups (`TestAdmitRedisCommandsDoNotGrowWithGroups`):
protected scope EVALSHA + 4 (HMGET, HGET, HSET, SET); unprotected scope 0 with the snapshot
(`TestSnapshot_ProtectionGroupEnforcedOnNextWrite`), EVALSHA + 1 without it. Workers always use the script.

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
* A write admitted as unprotected just before a group is created carries no writer record. It is only a
  concern if it is still in flight when the group first freezes, i.e. after the whole catch-up phase.
* Redis Cluster is unsupported (the fence script touches keys derived from the group ID).
* Equal MD5-form ETags are trusted as equal content: a tenant able to craft MD5 collisions could keep a
  divergent object on a replica of its own data.
* e2e: `test/resilience/protection_groups_test.go` (three MinIO storages, three-proxy electorate: failover with
  copies restored; promotion completes after a proxy dies mid-write) and
  `test/resilience/protection_shared_account_test.go` (shared connection account users never inherit a group;
  legacy policy/group overlap refused; legacy failover skips group scopes with the snapshot disabled).
