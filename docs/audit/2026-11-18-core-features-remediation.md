# Core features remediation

This change addresses the 14 findings from the review of `feat/core-features`
at `6ba2153`, relative to main at `b3dec72`.

| Finding | Resolution |
| --- | --- |
| R1: marker-only hard deletion | Require a successful versioning check with status Enabled before creating a marker. |
| R2: lifecycle overwrite race | Delete the observed immutable version; refuse unsafe buckets or keys whose deletion could reveal an older value. |
| R3: prefix changed after preview | Preserve literal prefix bytes in both create and update requests. |
| R4: filters lost in Redis fallback | Decode projected JSON arrays when reading status and fail on malformed protection fields. |
| R5: synchronous path bypassed policies | Resolve the policy in the proxy's syncer, including deletion restrictions and delay. |
| R6: stale writes bypassed delete delay | Persist the deletion deadline alongside version removal and enforce it on every deleting path. |
| R7: lifecycle leases did not renew | Use owner tokens, separate lease expiry, renewal and fenced completion. |
| R8: fixed deployment secrets | Remove the tracked Secret manifest and generate unique credentials directly into a create-only Kubernetes request. |
| R9: Redis-only updates were no-ops | Persist related fields atomically; support clearing arrays and reject missing policies. |
| R10: partial lifecycle failure was hidden | Return failures while retaining successful deletion counts and audits. |
| R11: missing lifecycle create button | Use the PageShell actions slot. |
| R12: audit response ordering | Clear the next drawer and ignore responses from superseded requests. |
| R13: CORS origin handling | Configure exact production origins and parse IPs for development defaults. |
| R14: retry did not enqueue work | Re-enqueue persisted, generation-bound delete events with explicit outcome correlation and rollback on enqueue failure. |

The final CLI checks also found four new Policy RPCs missing from tuttictl's
REST route map. Their mappings now match the generated HTTP gateway. Existing
route-completeness and gateway-dispatch tests cover this correction.

## Operational changes

Read [lifecycle operations](../tutti/lifecycle-operations.md) before upgrading.
Drain old lifecycle workers before enabling the new executors: old binaries do
not honor the new lease/deletion protocol. Automatic expiration now requires
enabled versioning and immutable version IDs. It refuses keys with older live
versions. Dry Run remains an estimate and can exceed safe deletions.

Drain old replication workers and proxies before relying on the corrected
delete modes, filters or deadlines. Keep destination versioning enabled while
using marker-only propagation: S3 versioning checks and DELETE are separate
requests, so an administrator suspending versioning concurrently can still race
with deletion. Intentional policy skips satisfy synchronous quorum; an actual
deferred delete does not count as a completed synchronous delete.

Apply database migrations 0011 (lifecycle leases) and 0012 (delete event
generation). Legacy delete events without a recorded generation cannot be
safely replayed and are rejected rather than retried under a new replication.

For a cross-origin management UI, set `api.corsAllowedOrigins` to exact origins,
including scheme and port. Empty configuration permits localhost and private IP
origins for development.

Kubernetes deployments now apply `01-configmaps.yaml` and use
`sh create-secrets.sh` once for a new installation. The script refuses to
overwrite an existing secret. If credentials from the removed manifest were
deployed, rotate them deliberately, accounting for the master encryption key.

The feature branch is integrated into main using a squash commit so the removed
Secret document is not introduced into main's commit history.

## Validation

Regression coverage includes conditional version targeting, lifecycle ownership
and fencing, partial failures, JSON fallback, Redis array resets, marker-only
checks, delayed deletes, source recreation, retry handoff and UI request races.

Validation completed:

- All package suites under `pkg/...` and `service/...` were exercised with the
  Go race detector. The first broad run overlapped the addition of router test
  imports and failed that test build; the final router run passed, as did the
  reruns of store, policy, replication and syncer on settled sources. No races
  were reported.
- `go vet -p 1 ./...` and `go vet -p 1 ./test/...` passed.
- UI: 134 files / 873 tests passed; ESLint and TypeScript checks passed.
- tuttictl: all module tests passed, including route completeness and dispatch.
- Embedded integration: synchronous PUT/DELETE, object CRUD, versioning, proxy
  CORS, user migration and user switch passed. CRUD/migration fixtures now opt
  into synchronous deletion explicitly instead of relying on the old default.
- PostgreSQL 16 container tests covered lease fencing, migrations and retry
  handoff. Real Asynq execution against an in-memory Redis broker covered
  retry deduplication and deferred deletion without consuming retry attempts.
- The Secret generator was checked offline for the complete required key set,
  unique credentials, matching DSN and create-only behavior. Removed credential
  values were absent from staged tracked text files; YAML and shell syntax
  checks passed.

The entire Docker end-to-end matrix and production throughput benchmarks were
not run locally. No destructive tests were run against production storage or a
live Kubernetes deployment.
