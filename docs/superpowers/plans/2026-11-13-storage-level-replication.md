# Storage-level Replication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a replication (and a protection group) copy storage A to storage B with no user, and stop any creation that would cover a bucket already covered by another replication or group.

**Architecture:** `user` becomes optional end to end; empty user = storage-level replication that uses each storage's connection account; the synthetic `$service` identity is deleted. One new pure package `pkg/replscope` decides conflicts (same destination or reverse direction, with intersecting buckets) and is called under one global scope lock by every creation path (replication add, group create/update).

**Tech Stack:** Go, Redis (asynq queues, p:/v: keys), Postgres control plane, protobuf via `make proto-gen` (`go tool buf generate`, works offline), Vue 3 + Naive UI + Vitest, `tuttictl` (Go, own module under tools/tuttictl).

**Spec:** `docs/superpowers/specs/2026-11-13-storage-level-replication-design.md` (section numbers below refer to it).

## Global Constraints

- Branch `feat/storage-level-replication`. Commit only your own paths: `git add <paths>` then `git commit -m "..." -- <paths>`. Never run `git add -A`, `git commit-tree`, `git update-ref`, `git rebase`, `git reset`, `git stash`, `git checkout -- .`. If `index.lock` appears wait and retry. `deploy/k8s/01-secrets-configmap.yaml` stays untracked. Commit message body ends with a blank line and `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.
- No schema change to Postgres (`user_name` is `NOT NULL`, empty string is valid) and no data migration (no data exists).
- The string `$service` and the constant `protectiongroups.ServiceUser` must not exist anywhere after Task 3 (`grep -rn 'ServiceUser\|\$service' --include=*.go pkg service tools` returns nothing except tests that assert absence).
- API never returns `$service`; an empty user is returned as `""`.
- A creation rejected by the overlap rule leaves NO policy, key, group or queue entry behind.
- Existing user-based replications keep working unchanged.
- Go gate for every Go task: `go build ./... && go vet <touched pkgs> && go test <touched pkgs>`. UI gate: from `ui/`, `corepack yarn lint:types`, `corepack yarn test:unit --run <scope>`, `corepack yarn lint:eslint <files>`, `npx prettier --check <files>`.
- Task reports to the controller: max 10 lines.

## Review Focus

- Replication ID round-trip with empty user in all four shapes (`""` and `"alice"` × user-level and bucket-level) including queue names and `UniversalIDFromString`.
- Two simultaneous creations over the same bucket: exactly one succeeds, the other gets `AlreadyExists`.
- Storage with no connection account used for a user-less replication: clear error naming the storage, not a generic 404.
- A group {A,B,C} (all ordered pairs count) vs a standalone `A→D`: allowed; vs `C→A`: rejected.
- `AvailableBuckets` for a user-less request marks buckets covered by groups and by user-level replications, not only by the same user.

## Dependency order

`T1 ∥ T2` → `T3` → `T4 ∥ T5` → `T6 ∥ T7` → `T8`. Same working tree; disjoint file ownership.

---

### Task 1: Replication ID and keys accept an empty user

**Files:**
- Modify: `pkg/entity/replication_id.go` (+ `policy.go` validators for `UserReplicationPolicy`/`BucketReplicationPolicy`), `pkg/store/replication_stores.go`, `pkg/store/replication_generation_store.go`, `pkg/store/routing_stores.go`, `pkg/meta/version_keys.go`, `pkg/tasks/queue.go` (queue name), `pkg/controlplane/pg/replication.go` (only if a key/column assumes non-empty user)
- Test: `pkg/entity/replication_id_test.go`, `pkg/store/*_test.go`, `pkg/meta/*_test.go`, `pkg/tasks/queue_test.go`

**Interfaces — Produces:**
- `func (r UniversalReplicationID) User() string` never panics; returns `""` for storage-level.
- `func (r UniversalReplicationID) IsStorageLevel() bool` (`User() == ""`).
- `IsEmpty()` is true only when from/to storage are both empty, no longer when user is empty.
- `AsString()` / `UniversalIDFromString(string) (UniversalReplicationID, error)` round-trip all four shapes; the user token may be empty (`":s1:s2"`, `":s1:s2:b1:b2"`).
- Policy `Validate()` accepts empty user for both policy types and still rejects empty from/to storage.
- Every key builder in the listed files accepts `""` as user and produces a key distinct from every key of a real user.

- [ ] **Step 1: Write failing tests** — `TestUniversalID_RoundTrip` (table: user ""/"alice" × user-level/bucket-level; asserts `UniversalIDFromString(id.AsString())` equals `id`, `IsStorageLevel` true only for ""), `TestUserMethodDoesNotPanic`, `TestPolicyValidate_EmptyUserAllowed`, `TestQueueNameForEmptyUserDistinct` (queue name for `""` differs from any `alice` name and parses back), key-builder tests asserting `p:repl_user_to:` + `""` and `v::s1:s2:b1:b2`-style keys are unique.
- [ ] **Step 2: Run** `go test ./pkg/entity/... ./pkg/store/... ./pkg/meta/... ./pkg/tasks/...` — Expected: FAIL (panic / validation).
- [ ] **Step 3: Implement** the Interfaces above. `UniversalIDFromString` must split on the field count (3 or 5 tokens), not on the first token being non-empty. Storage names containing `:` stay invalid (already enforced elsewhere; add a test).
- [ ] **Step 4: Run gate** — Expected: PASS; also `go test ./pkg/policy/... ./service/worker/...` still compile (they may fail only if they relied on the old panic; fix by updating those tests, not by weakening the new behavior).
- [ ] **Step 5: Commit** `feat(replication): allow an empty user in replication ids and keys`.

### Task 2: Shared conflict rule `pkg/replscope`

**Files:**
- Create: `pkg/replscope/scope.go`, `pkg/replscope/check.go`, `pkg/replscope/locker.go`, `pkg/replscope/scope_test.go`, `pkg/replscope/check_test.go`, `pkg/replscope/locker_test.go`

**Interfaces — Produces (exact):**
```go
type Edge struct{ From, To string }
type Scope struct {
    Edges      []Edge
    Buckets    []string // ignored when AllBuckets
    AllBuckets bool
}
type Owner struct {
    Kind  string // "group" | "replication"
    ID    string // group id, or UniversalReplicationID.AsString()
    Scope Scope
}
type ConflictError struct{ OwnerKind, OwnerID, Bucket string; Edge, OtherEdge Edge }
func (e *ConflictError) Error() string // "bucket %q on %s->%s is already covered by %s %q"; Unwrap() returns dom.ErrAlreadyExists
func GroupScope(members []string, buckets []string) Scope        // every ordered pair of distinct members; empty buckets => AllBuckets
func ReplicationScope(from, to string, buckets []string) Scope   // one edge; empty buckets => AllBuckets
func Conflicts(a, b Scope) (conflict bool, bucket string, ea, eb Edge)
func Check(proposed Scope, existing []Owner, ignoreID string) *ConflictError // nil when allowed; skips owner with ID == ignoreID
type Locker interface{ Lock(ctx context.Context) (unlock func(), err error) }
func NewRedisLocker(rdb redis.UniversalClient) Locker // one global key `lock:repl_scope`, TTL 30s, auto-renew not required
```
Rule (spec 4.4): conflict iff some edge pair has same `To`, or `a.From==b.To && a.To==b.From`, AND bucket sets intersect (AllBuckets intersects anything; both AllBuckets reports bucket `"*"`).

- [ ] **Step 1: Write failing tests** — truth table in `TestConflicts`: `A→B`/`A→B` conflict; `A→B`/`C→B` conflict; `A→B`/`B→A` conflict; `A→B`/`A→C` allowed; `A→B`/`B→C` allowed; `A→B`/`C→D` allowed; same edges but disjoint buckets allowed; AllBuckets vs `{"x"}` conflict; group {A,B,C} vs `A→D` allowed, vs `C→A` conflict, vs `A→B` conflict; `ConflictError` message names owner and bucket; `errors.Is(err, dom.ErrAlreadyExists)`; `TestLockerMutualExclusion` (two goroutines, miniredis or the repo's redis test helper: second `Lock` blocks until first unlock; ctx cancel returns error).
- [ ] **Step 2: Run** `go test ./pkg/replscope/...` — Expected: FAIL (package missing).
- [ ] **Step 3: Implement** the Interfaces. Pure functions, no Redis in `scope.go`/`check.go`.
- [ ] **Step 4: Run** `go build ./... && go vet ./pkg/replscope/... && go test ./pkg/replscope/...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(replscope): add shared replication conflict rule and scope locker`.

### Task 3: Policy service and protection groups — drop `$service`, use `replscope`

**Files:**
- Modify: `pkg/protectiongroups/*.go` (delete `ServiceUser` in `model.go`; `Group.Replications` emits empty user; replace `Group.Overlaps` and its three call sites in `store.go:258,418,486` with `replscope`), `pkg/policy/routing.go`, `pkg/policy/context.go`, `pkg/policy/protection_groups.go`, `pkg/policy/replication.go`, `pkg/accesspolicy/scope.go` (empty user = no filtering), `pkg/objstore/credentials.go` (`HasUser`/`ValidateReplicationID`), `pkg/objstore/connection.go`
- Test: matching `_test.go` files

**Interfaces — Consumes:** Task 1 (`IsStorageLevel`, empty-user keys), Task 2 (`replscope.*`).
**Interfaces — Produces:**
- `policy.Service.ListScopeOwners(ctx context.Context) ([]replscope.Owner, error)` — returns one Owner per protection group (`GroupScope(members, buckets)`), one per user policy and per bucket policy (`ReplicationScope`, regardless of user).
- `objstore.CredsService.HasUser(storage, user string) error`: for `user == ""` returns nil iff the storage has a connection account; otherwise `fmt.Errorf("%w: storage %q has no connection account", dom.ErrFailedPrecondition, storage)`. Real-user behavior unchanged.
- `protectiongroups.Service`/store creation and update call `replscope.Check` inside `replscope.Locker.Lock`, with `ignoreID` = the group being updated; on conflict the group is not created.
- The routing check in `routing.go` (the old ServiceUser bypass at lines ~409/431) uses `repl.IsStorageLevel()`; proxy context (`context.go`) attaches group replications with empty user; `accesspolicy.Filter(storage, "", buckets)` returns all buckets unchanged.
- Per-user "user-level blocks bucket-level" exclusivity (`IsUserReplicationExists` use) applies only when `user != ""`.

- [ ] **Step 1: Write failing tests** — `TestGroupReplicationsEmptyUser` (infra group yields ids with `User()==""`), `TestHasUserEmptyNeedsConnectionAccount` (both branches, message names the storage), `TestAccessPolicyFilterEmptyUser`, `TestGroupCreateRejectedOnOverlapLeavesNothing` (existing group {A,B} all buckets; creating group {A,C} bucket x returns error `errors.Is(ErrAlreadyExists)` and store holds only the first group), `TestListScopeOwnersIncludesGroupsAndPolicies`, `TestStorageLevelNotBlockedByUserExclusivity`.
- [ ] **Step 2: Run** `go test ./pkg/protectiongroups/... ./pkg/policy/... ./pkg/objstore/... ./pkg/accesspolicy/...` — Expected: FAIL.
- [ ] **Step 3: Implement** per Interfaces; delete `ServiceUser` and every reference (`grep -rn 'ServiceUser\|\$service' --include=*.go pkg service tools` must be empty at the end); keep seeding behavior for storage-level groups creating bucket/user policies with empty user.
- [ ] **Step 4: Run gate** (`go build ./... && go vet && go test` on the packages above plus `./service/worker/...`) — Expected: PASS.
- [ ] **Step 5: Commit** `feat(policy): replace the $service identity with empty-user storage-level replication`.

### Task 4: API and proto — optional user, covered buckets, conflict check

**Files:**
- Modify: `proto/tutti/policy.proto`, `proto/tutti/protection_groups.proto` (only if needed), regenerate with `make proto-gen` (commit generated files under `proto/gen`), `pkg/api/policy_handlers.go`, `pkg/api/protection_groups.go`, `pkg/api/webhook_handlers.go` is NOT touched here
- Test: `pkg/api/policy_handlers_*_test.go`, `pkg/api/protection_groups_*_test.go`

**Interfaces — Consumes:** Task 3 (`ListScopeOwners`, `HasUser("", …)`, `replscope`).
**Interfaces — Produces (wire contract used by Tasks 6 and 7):**
- `AddReplication`: `id.user` may be empty (no "replication user is required" error); with empty user, requires connection account on both storages (error from `HasUser`), skips `inUserLock`, and runs `replscope.Check(proposed, ListScopeOwners)` inside `replscope.Locker` before creating anything; conflict returns gRPC `AlreadyExists` / HTTP 409 with the `ConflictError` text.
- `AvailableBuckets` (`POST /replication/list-buckets`): `user` optional; lists buckets with the connection account of `from_storage`; `AvailableBucketsResponse` gains `repeated CoveredBucket covered_buckets = N` with `message CoveredBucket { string bucket = 1; string owner_kind = 2; string owner_id = 3; }` for every bucket in the storage already conflicting for the requested edge (computed with `replscope`); `buckets` no longer contains covered buckets; `replicated_buckets` keeps its meaning for the same edge.
- `Get/List/Pause/Resume/Delete/Switch*` accept empty user; responses never contain `$service`.
- Protection group Create/Update: conflict check under the same locker, with `ignoreID` on update; error text names owner and bucket.

- [ ] **Step 1: Write failing tests** — `TestAddReplicationEmptyUserOK`, `TestAddReplicationEmptyUserNoConnectionAccountNamesStorage`, `TestAddReplicationConflictSameDestination` (409, nothing stored), `TestAddReplicationAllowedFanOut` (`A→B` then `A→C` same bucket OK), `TestAddReplicationReverseRejected`, `TestConcurrentAddOnlyOneWins` (two goroutines, same bucket, exactly one OK), `TestAvailableBucketsMarksCoveredByGroup`, `TestGroupCreateConflictWithReplication`.
- [ ] **Step 2: Run** `go test ./pkg/api/...` — Expected: FAIL.
- [ ] **Step 3: Implement** the proto change, run `make proto-gen` (no hand edits; if it cannot run, stop and report), then handlers per Interfaces.
- [ ] **Step 4: Run gate** `go build ./... && go vet ./pkg/api/... && go test ./pkg/api/... ./pkg/policy/... ./pkg/protectiongroups/...` — Expected: PASS.
- [ ] **Step 5: Commit** in two commits: `feat(api): make replication user optional and report covered buckets` (proto + generated + handlers), tests with it.

### Task 5: Worker, proxy, webhook and task payloads with an empty user

**Files:**
- Modify: `service/worker/handler/*.go`, `service/worker/copy/copy.go`, `service/worker/syncer/syncer.go`, `service/proxy/router/*.go` (policy middleware, object routing for storage-level), `pkg/api/webhook_handlers.go`, `pkg/notifications/*` (ConfigurationId encoding), `pkg/tasks/tasks.go` (diff payload `User`)
- Test: matching `_test.go`, plus `test/migration` only if a fast test exists

**Interfaces — Consumes:** Task 1 (`IsStorageLevel`, `AsString`), Task 3 (`HasUser`, empty-user proxy context in `pkg/policy/context.go`).
**Interfaces — Produces:** `notifications.NewNotificationID(user, storage, ...)`/`UserIDFromNotificationID(id) (user string, ok bool)` round-trip an empty user (exact existing names; extend, do not rename).

- [ ] **Step 1: Write failing tests** — worker copy of an object with an empty-user replication id resolves credentials through the connection account (use the existing fake clients) and does not panic; seed handler (`migration_s3_user_handler.go` / bucket listing handler) with empty user lists all buckets; notification id round-trip with empty user; proxy router test: a write to bucket `x` on a storage covered by a storage-level replication triggers exactly one replication task; object that exists identically on both sides is skipped; **characterization tests** that pin current behavior for same-name-different-content and object-only-in-destination (assert what the code does today, and write the finding in the report).
- [ ] **Step 2: Run** `go test ./service/... ./pkg/notifications/... ./pkg/tasks/...` — Expected: FAIL where empty user is unsupported.
- [ ] **Step 3: Implement** minimal changes so every `ID.User()` consumer works with `""`; no special-casing outside `IsStorageLevel()` conditions already introduced.
- [ ] **Step 4: Run gate** `go build ./... && go vet ./service/... ./pkg/notifications/... ./pkg/tasks/... && go test ./service/worker/... ./service/proxy/... ./pkg/notifications/... ./pkg/tasks/...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(worker): run storage-level replication with the connection account`.

### Task 6: UI — Add replication without a user, covered buckets disabled

**Files:**
- Modify: `ui/src/stores/tuttiAddReplicationStore.ts` (+ `__tests__/tuttiAddReplication.spec.ts`), `ui/src/components/tutti/add-replication/*` (BucketsFormStep, BucketsSelectionList, i18n), `ui/src/pages/TuttiAddReplicationPage/*`, `ui/src/utils/helpers/BucketDiscoveryHelper.ts` (+ spec), `ui/src/utils/types/tuttiCore.ts`, `ui/src/services/TuttiCoreService.ts`, replication list/row components that print the user (`ui/src/pages/TuttiReplicationPage/*`, `ReplicationsHelper.ts`)

**Interfaces — Consumes:** wire contract in Task 4 (`covered_buckets[]` with `bucket`, `ownerKind`, `ownerId`; `user` optional in requests).

- [ ] **Step 1: Write failing tests** — store: no `selectedUser`/`users`/`eligibleUsers` required; `initBucketsList` calls `getBucketsForReplication` without `user`; covered buckets are exposed as disabled with a reason string containing owner kind and id; `createReplication` sends `id` without `user`; a 409 shows the server text on the page and keeps the wizard draft; replications table renders "A → B" when user is empty.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/stores src/components/tutti src/utils/helpers` — Expected: FAIL.
- [ ] **Step 3: Implement** removal of the user select and user discovery from the Add replication flow; disabled rows with tooltip `owner_kind owner_id`; replace the generic list-buckets error with the server message; types updated.
- [ ] **Step 4: Run UI gate** — Expected: PASS (`lint:types` 0 errors).
- [ ] **Step 5: Commit** `feat(ui): add replication without a user and block covered buckets`.

### Task 7: tuttictl and docs

**Files:**
- Modify: `tools/tuttictl/cmd/repl*.go`, `tools/tuttictl/internal/api/{rest.go,mappers.go}`, `tools/tuttictl` tests; `docs/tutti/api.md`, `docs/tutti/protection-groups.md`, `docs/data-model.md`, `docs/tutti/operations.md`; `CHANGELOG.md`

- [ ] **Step 1: Write failing tests** — `--user` optional on `repl add|buckets|delete|pause|resume`; request body has no `user` when the flag is absent; a 409 prints the server message and exits non-zero.
- [ ] **Step 2: Run** `cd tools/tuttictl && go test ./...` — Expected: FAIL. (If Go 1.24 toolchain is unavailable offline, report it and run `go vet` only.)
- [ ] **Step 3: Implement** the flag change and docs: storage-level replication, the conflict rule table from spec 4.4 (allowed vs rejected), removal of `$service`, "one source to many destinations = several replications or one group".
- [ ] **Step 4: Run gate** — Expected: PASS.
- [ ] **Step 5: Commit** `feat(tuttictl): make --user optional; document storage-level replication`.

### Task 8: Integration and real-stack verification

**Files:** read all; small fixes only where integration breaks. New: `docs/superpowers/plans/2026-11-13-storage-level-replication-verification.md`

- [ ] **Step 1: Full gate** — `go build ./... && go vet ./... && go test ./pkg/... ./service/...` and from `ui/` `lint:types`, `test:unit --run`, `lint:eslint`, `build`. Fix every break. Known pre-existing failures in `test/` integration dirs and `tools/tutti-migrate-pg` may be reported, not fixed.
- [ ] **Step 2: Grep gate** — `grep -rn 'ServiceUser\|\$service' --include=*.go --include=*.ts --include=*.vue pkg service tools ui/src` returns nothing.
- [ ] **Step 3: Isolated end-to-end** — own compose project `slrepl` (different ports, memory limits, two MinIO 128m), register both MinIO as storages with connection accounts, **no S3 user**; verify with the API: add bucket-level `A→B`; add `A→C` same bucket (allowed); add `C→B` (409 same destination); add `B→A` (409 reverse); create group {A,B} all buckets (409 against existing); create replications concurrently on the same bucket (one wins); put an object through the proxy and confirm it appears at B; clean everything created.
- [ ] **Step 4: User stack check** — against the user's running stack (project `docker-compose`, storages `hcm02`, `hcm04`), only read-only calls: `POST /replication/list-buckets` without user for `hcm02`→`hcm04`; report the result or the exact error. Do not create replications there.
- [ ] **Step 5: Write the verification report and commit** `chore: verify storage-level replication`.
