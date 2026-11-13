# Storage-level replication: verification report (Task 8)

## 1. Gate
- `go build ./... && go vet ./...`: clean. `go test ./pkg/... ./service/...`: all packages ok (also re-run after the fixes below).
- `ui/`: `lint:types` ok, `test:unit --run` 74 files / 505 tests ok, `lint:eslint` ok, `build` ok.
- `tools/tuttictl`: build, vet, test ok.

## 2. Grep gate
`grep -rn 'ServiceUser\|\$service'` over pkg, service, tools, ui/src, docs: only sentences in CHANGELOG, docs/data-model.md, docs/tutti/*.md and the spec/plan that describe the removal. No code hits. UI string `wizardLoadError` now says "Could not load storages." (en, de, vi).

## 3. End-to-end (own compose project `slrepl`, removed afterwards)
Three MinIO (A=sta, B=stb, C=stc, 128m each), redis 384m, postgres 384m, worker 768m, proxy 384m; each storage registered with a connection account; no S3 user while checking (a)-(i). Buckets are `bkt1..bkt4` (MinIO requires 3+ chars), standing in for b1..b4. Web-ui was not started (not in the memory budget; its build is covered by `yarn build`).

Bugs found and fixed during the run:
1. **Postgres backend rejected storage-level replications** (`pkg/controlplane/pg/replication.go` `setGen`): `POST /replication/add` without user returned `400 InvalidArg: replication generation: user and destination are required`. Fixed to require only the destination; regression test added in `pkg/controlplane/pg/repos_test.go`. Not caught earlier because unit tests used Redis or user-bearing ids.
2. **Group creation over an existing unmanaged replication returned 400 FailedPrecondition** ("use explicit adoption") instead of 409 for replications outside the group topology (`pkg/policy/protection_groups.go`, infrastructure branch). Now returns `AlreadyExists` (409) when the replication is not part of the group's own topology; the adoptable-but-not-adopted case keeps FailedPrecondition.

Captured results (final build):
- (a) list-buckets A->B without user: 200 `{"buckets":["bkt1","bkt2","bkt3"],"replicatedBuckets":[],"coveredBuckets":[]}`. PASS
- (b) add A->B bkt1, no user: 200 `{}`. PASS
- (c) add A->C bkt1: 200 (fan-out). PASS
- (d) add C->B bkt1: 409 `bucket "bkt1" on stc->stb is already covered by replication ":sta:stb:bkt1:bkt1"`. PASS
- (e) add B->A bkt1: 409 `bucket "bkt1" on stb->sta is already covered by replication ":sta:stb:bkt1:bkt1"`. PASS
- (f) group {A,B} all buckets while bkt1 and bkt2 replications exist: 409 `replication :sta:stb:bkt2:bkt2 already covers part of this group's scope` (after fix 2; before: 400). PASS
- (g) group {A,B} bucket bkt2 only (no overlap with A->B bkt1): 200, group state ready. Matches the rule (bucket sets disjoint). A group {A,B} for bkt1 returns 400 FailedPrecondition "unmanaged replication ... use explicit adoption" because the existing A->B bkt1 is exactly the group's own topology (adoptable with `adoptExisting`); this is the pre-existing adoption design, not the 409 overlap path.
- (h) concurrent add A->B bkt3 twice: one 200, one 409 `replication already exists, delete it first` (identical id is caught by the existing duplicate check). Concurrent A->B and C->B on bkt4 (the real scope-lock case): one 200, one 409 `bucket "bkt4" on sta->stb is already covered by replication ":stc:stb:bkt4:bkt4"`. List shows exactly one replication per bucket. PASS
- (i) After (b)-(c): for the A->B edge bkt1 is NOT in `buckets` and is in `replicatedBuckets` (`{"buckets":["bkt2","bkt3"],"replicatedBuckets":["bkt1"],"coveredBuckets":[]}`); `coveredBuckets` is non-empty on the conflicting edges, e.g. C->B and B->A: `[{"bucket":"bkt1","ownerKind":"replication","ownerId":":sta:stb:bkt1:bkt1"}]`. PARTIAL: matches the implemented design (same edge goes to replicatedBuckets, other conflicting edges to coveredBuckets), not the literal wording of the check.
- (j) Put through the proxy as a real user (the proxy needs a proxy key; direct writes to MinIO emit no event, event source is proxy): `new.txt` reached B and C in about 3 s. The initial seed copied `seed.txt` to B and C. Same name with different content: B had `B-DIRECT-CONTENT`, then A via proxy wrote `A-VIA-PROXY-1`: A, B and C all end with `A-VIA-PROXY-1` (source wins). `onlyb.txt` on B stays. PASS
- (k) Delete all replications and the group: all 200; `replication` list empty, groups empty, list-buckets shows all buckets again, `coveredBuckets` empty. PASS
- (l) User-level all buckets A->B for alice and for bob (users created with connection-account mapping on all storages): both 200. A storage-level all-buckets A->B afterwards is 409 `bucket "*" on sta->stb is already covered by replication "alice:sta:stb"`. PASS

Cleanup: `docker compose -p slrepl down -v`, images `slrepl-worker:test` and `slrepl-proxy:test` removed. `docker ps -a`, `volume ls`, `network ls`, `images` show nothing of `slrepl`. The user's `docker-compose` project was not touched. Build cache from the two image builds was left (no prune).

## 4. Remaining issues
- Conflict messages show the raw replication id with an empty user (`":sta:stb:bkt1:bkt1"`) and name a group's managed replication, not the group.
- `NOTE`: Step 4 of the plan (read-only check on the user's `hcm02`->`hcm04` stack) was not run: the task forbade touching that project's stack.
- The user-level conflict path of the spec (two real users, all buckets) passes at API level here; `BuildProxyContext` still rejects an empty client user, so a proxy client always needs a user key.
- Pre-existing failures in `test/` integration dirs and `tools/tutti-migrate-pg` were not examined (not part of the gate commands run).
