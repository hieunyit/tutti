# Resume status (2026-11-10)

Runtime storage management is complete. Backend commit: `1c76a4c`; UI follows in the next local commit. No push or production-demo rebuild was performed.

- Redis definitions/version + encrypted dynamic credentials; worker/proxy reloads; health/rate-limit refresh and in-flight client safety.
- REST/gRPC list/create/update/delete/test with config/main/reference protections.
- Two-step Add storage and Settings tab (TLS, provider, endpoint, region, durations, rate limit, changed-value hints, mandatory test, in-use confirmation and typed delete).
- Finished the missing phase-D route pages/auth wiring needed to make the inherited working tree build: Users, Create/Detail user, Proxies, Console users and forced/change-password flows.
- Validation: Go unit suites (except standalone default-port test while the existing demo owns 9669), `go vet`, targeted race, full real-MinIO resilience suite; 228 UI tests, complete UI lint and production build. Real API Chromium create/test/update/config-readonly/delete passed at 1440px/390px, light/dark; screenshots `/tmp/tutti-storage-live/`.
- API and operation details: `docs/tutti/runtime-storages.md`.
- Still backlog: flatten Access keys page, performance report/fixes, early quorum return and other items below unrelated to storage management. The detached performance/demo worktrees were preserved.
- Temporary isolated verification stack: API29881/proxy29879/Redis26379 under `/tmp/tutti-runtime/`; main demo8080/9671/9669 was left untouched.

The original Claude handoff below is historical; its in-progress/not-started storage and missing-page entries are superseded by this status.

---

# Tutti: handoff (2026-11-10)

Tutti = fork of Clyso Chorus (Go S3 proxy + worker, Redis, asynq) with the resilience features of
ReplicaT4 ported in, plus an enterprise web console (Vue 3, `ui/`). Module `github.com/hieunyit/tutti`.
All work is LOCAL commits on `main`; nothing has been pushed.

## 1. Done (committed)

| Commit(s) | What |
|---|---|
| ab97996 | Rename fork to Tutti |
| 96d3046, f3b063a | Proxy: unsigned content-type, never drop replication events; aws-chunked decode + verify (`pkg/s3/chunked`) |
| 178745f | Optional synchronous replication on the write path (`replication.sync`, header `X-Tutti-Replicas`) |
| 5949da2 | Per-storage circuit breakers, health checks, fail-fast (`pkg/health`) |
| a5e56fb, 95084c2, 90917fc | Automatic failover (policy per source storage, ordered targets, `after`, `maxEventLag`, no auto failback); Resilience API; routing fix after failover |
| 1ca8525, 8ff789b | Resilience e2e suite (`test/resilience`, real MinIO containers), CI, docs (`docs/tutti/*`) |
| 34a416c, 1b1401a | Console login, roles admin/viewer, sessions in Redis, throttling, trustedProxies |
| bb80e38 | Per-account access policies (read-only / write-only / bucket globs) |
| e9cab3b, e309882 | Traffic stats; status every 2 s with server-side rates; 6 h stats history in Redis, `GET /tutti/stats/history` |
| 41c830e | Atomic `PUT /routing/update`; dynamic proxy access keys (create/rotate with grace/disable/delete, secrets sealed AES-GCM) |
| e7f84c8 | (by the user) security + replication hardening |
| 435fe38 | S3 user management API (`/tutti/users`: list/create with verify/update creds/delete/test) and managed console users (`/tutti/console-users`, `POST /auth/password`) |
| aa279a0 | Server-side enforcement of `mustChangePassword` (403, ErrorInfo reason `PASSWORD_CHANGE_REQUIRED`) |
| 2c20181 | docker-compose demo: auth on (admin/admin, viewer/viewer), proxy failover on, dynamic-credentials master password |
| 50d53e3 … a6224d2 | UI: new design system and shell (dark grouped sidebar, tokens `ui/src/styles/tokens.scss`, shared `ui/src/components/ui/*`), Login, Dashboard, Monitoring (history API, ranges), Failover (banner "policies will not trigger", "Will it trigger?"), Proxy accounts, Replication/Routing/Diff list pages, Documentation (`/docs`, markdown from `docs/`), API reference (`/docs/api`, lazy Swagger UI); UI image takes optional `docs`/`proto` build contexts |
| 91f7474, d543f98 | Design source for every screen: `docs/design/console/*.dc.html` (static HTML mockups; ignore `<x-dc>`, `<sc-for>`, `{{ }}`). Live canvas: https://claude.ai/artifact/GemX8nJrerwx7Kpms5pTsE (owner-private) |

## 2. In progress when handed off (UNCOMMITTED, check `git status` / `git diff`)

Three background agents were still running at handoff time; their edits may be partial.

1. **UI phase D** (files under `ui/` only). Goal, per design boards `Storages`, `StorageDetail`, `Proxies`, `Users`,
   `UserCreate`, `UserDetail`, `ConsoleUsers`, `ProxyAccounts` (renamed "Access keys"):
   - nav: Overview (Dashboard, Monitoring) · Storage (Storages, Proxies) · Data protection (Replication, Routing policies,
     Failover, Diff reports) · Access (Users, Access keys) · Administration (Console users, admin only) · Help (Documentation,
     API reference); `/health` redirects to `/storages`.
   - Storages merged with Storage health (one list + summary KPIs); Storage detail with tabs Overview / Health by proxy / Users / Settings.
   - Proxies page (settings drift warning).
   - Users list, 3-step Create user wizard (identity → per-storage credentials with Verify → first key + permission), User detail tabs.
   - Access keys flat list (fix: nested keys table was clipped at 1440 px).
   - Console users (invite with one-time password, reset, disable, delete; self/last-admin protection).
   - Change-password dialog in the user menu; forced `/change-password` screen when `/auth/me.mustChangePassword` or a 403 with
     `details[].reason == PASSWORD_CHANGE_REQUIRED` (REST body: `{"code":7,"message":"password change required","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"PASSWORD_CHANGE_REQUIRED","domain":"tutti.auth"}]}`).
   - To finish: `cd ui && corepack yarn lint && corepack yarn test:unit --run && corepack yarn build`, then check every page in a browser
     (light/dark, 390 px width, admin and viewer) against the design boards, then commit.
2. **Runtime storage management backend** (Go only). Goal: add/update/delete S3 storages at runtime without restarts.
   - Managed storages in Redis (`tutti:storages` + version, secrets sealed with the dynamic-credentials key); config storages stay read-only.
   - API under `/tutti/storages`: List, Create (verify connection/credentials before save, `force` to skip), Update (managed only; `confirm` when referenced), Delete (refused when main or referenced by replications/routings/failover/events/diffs), Test.
   - Propagation to worker + all proxies within ~5 s (poll like `pkg/objstore/credentials.go`), safe client swap, health checks/breakers for new storages.
   - Changing `storage.main` stays out of scope (config + restart).
   - To finish: `gofmt -l . ; go build ./... ; go vet ./pkg/... ./service/... ; go test -count=1 ./pkg/... ./service/... ; go test -count=1 ./test/resilience/`
     (`service/standalone` TestStandaloneDefaultConfig fails only while the docker demo holds port 9669), plus a live check, then commit.
3. **Performance measurement** (no source edits; writes only to the scratchpad `perf/` folder and a temporary git worktree there).
   Expected output: `REPORT.md` with proxy overhead vs direct MinIO (warp), CPU/heap profiles, Redis ops per request,
   large-upload memory, worker replication throughput, UI bundle analysis, and a ranked optimisation list. If the worktree still exists:
   `git worktree list` and `git worktree remove --force <path>`.

## 3. Not started (backlog)

1. **UI for runtime storages** (after item 2.2): "Add storage" two-step dialog on Storages and the Settings tab on Storage detail.
   Design board `StorageAdd.dc.html` (changed-field hint, in-use warning, Test before Save, config storages read-only, delete zone).
2. **Performance fixes** from the perf report (pick the top measured items).
3. **Sync replication early quorum return**: with `writeQuorum` < number of destinations the proxy still waits for all.
4. Access-policy and S3-user/console-user updates are read-modify-write without transactions (concurrent admin edits can race).
5. Login does not show "attempts left" (backend does not return it; UI already supports `remainingAttempts`).
6. Replications cannot be edited in place (only pause/resume/delete/re-add).
7. Docs: keep `docs/tutti/{api,configuration,operations}.md` in sync with the work in section 2.
8. The ReplicaT4 repo (`/home/hellc/ReplicaT4`, Rust) still has ~58 uncommitted files from the first review/fix round; decide whether to commit or drop.

## 4. How to run and test

- Backend: `go build ./... && go test -count=1 ./pkg/... ./service/...`; e2e with real MinIO: `go test -count=1 ./test/resilience/` (Docker needed).
- Proto: edit `proto/chorus/*.proto` + `proto/http.yaml`, then `make proto-gen`.
- UI: `cd ui && corepack yarn install --frozen-lockfile && corepack yarn dev` (set `VITE_DEV_API_TARGET=http://127.0.0.1:<worker REST port>`), `yarn lint`, `yarn test:unit --run`, `yarn build`.
- Demo stack: `docker compose -p tutti -f docker-compose/docker-compose.yml --profile fake --profile proxy up -d --build`
  → UI http://localhost:8080 (admin/admin), proxy S3 :9669, worker REST :9671. (It was last built from a detached worktree in the
  scratchpad at a6224d2; rebuild from the repo.)
- Local dev env used during development (may be gone after a reboot): Redis :16379 (`tutti-redis`), MinIO :19001/:19002
  (`r4-minio1/2`, minioadmin/minioadmin), worker REST :9881, proxy :9879, configs in the session scratchpad `tutti-bin/conf`
  (worker auth users admin / Admin-Pass-2026, viewer / Viewer-Pass-2026).

## 5. Conventions

- Licences: backend Apache-2.0 (Clyso header), `ui/` AGPL-3.0; new Tutti files use "Copyright © 2026 hieunyit"; ReplicaT4-derived parts MIT.
- UI: use the shared components and `--tt-*` tokens, one primary action per page in `PageHeader`, `WriteGate` for admin-only actions,
  `StatusTag` semantics (green ok / amber degraded / red failed / blue in progress / grey off), en dash for empty values,
  `DateTimeHelper` for dates, EN + DE i18n for every string, no horizontal scroll at 390 px, light + dark.
- Security: never return secrets or hashes from list APIs; secrets only once on create/rotate; destructive actions need confirmation;
  config-defined objects are read-only through the API.
- Commit locally after each finished phase with a descriptive message; do not push unless asked.
