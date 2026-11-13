# Sliding Session Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Console sessions expire after 30 minutes idle or 24 hours absolute, renew on real user activity through `POST /auth/refresh`, and the UI warns before expiry.

**Architecture:** The opaque server-side token stays. Redis TTL of the session key is `min(idle, absolute remaining)`; only `/auth/refresh` extends it (never `Authenticate`, so background polling cannot keep a session alive). The UI tracks real user activity, refreshes quietly, shows a countdown dialog, and syncs tabs.

**Tech Stack:** Go (`pkg/auth`), Redis, Vue 3 + Pinia + Vitest, axios interceptor in `ui/src/http`.

**Spec:** `docs/superpowers/specs/2026-11-13-session-refresh-design.md` (section numbers refer to it).

## Global Constraints

- Defaults: idle `30m`, absolute max `24h`, warnings at `2m` (idle) and `5m` (absolute). Config keys `sessionIdleTimeout`, `sessionMaxLifetime`; legacy `sessionTTL` still accepted (used as max lifetime when the new keys are unset).
- `Authenticate` never extends a session. Token never rotates. Revocation (logout, password change, disabled user) unchanged.
- Commit only your own paths on branch `main`: `git add <paths>` then `git commit -m "..." -- <paths>`. NEVER run `git add -A`, `git commit-tree`, `git update-ref`, `git rebase`, `git reset`, `git stash`, file checkouts. `deploy/k8s/01-secrets-configmap.yaml` stays untracked. Commit bodies end with a blank line and `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.
- All new UI strings in en, de, vi (a locale completeness test exists in `ui/src/i18n/__tests__/localeCompleteness.spec.ts`).
- Task reports to the controller: max 10 lines.

## Wire contract (fixed; Tasks 1 and 2 are built against it in parallel)

All times are RFC3339 UTC strings.
- `POST /auth/login` → `{token, expiresAt, maxExpiresAt, serverTime, username, role}` (`expiresAt` = idle expiry now; `maxExpiresAt` = absolute cap).
- `GET /auth/me` → existing fields plus `expiresAt`, `maxExpiresAt`, `serverTime`.
- `POST /auth/refresh` (Bearer token, no body) → `200 {expiresAt, maxExpiresAt, serverTime}`; `401` when the token is invalid, expired or the absolute cap is reached; `429` with `Retry-After` when called more than 6 times per minute per token.
- A `MustChangePassword` session keeps today's restrictions (refresh is allowed, like `/auth/me`).

## Review Focus

- A burst of background API calls (polling) over 31 minutes must NOT keep a session alive (Task 1 test).
- Refresh at 23h59m must give only 1 minute, never extend past the cap; the next refresh returns 401 (Task 1).
- Password change after a refresh still invalidates the session (Task 1).
- Client clock 10 minutes off from the server must not shorten or lengthen the displayed countdown (Task 2).
- Two tabs: refresh in tab A updates tab B's deadline; logout in A logs out B (Task 2).
- Network error during refresh must retry, not log the user out; only a 401 logs out (Task 2).

---

### Task 1: Backend sessions with idle timeout, cap and refresh

**Files:**
- Modify: `pkg/auth/config.go` (new keys, validation, accessors `IdleTimeout()`, `MaxLifetime()`), `pkg/auth/service.go` (`Session.LastActiveAt`, `Login`, `Authenticate` unchanged semantics, new `Refresh`), `pkg/auth/http.go` (`POST /auth/refresh`, login/me fields), `pkg/auth/interceptor.go` (add `/auth/refresh` to the HTTP-handled, non-gRPC set if needed), config docs `docs/tutti/configuration.md`, `docs/tutti/api.md`, `service/worker/config.yaml`, `docker-compose/worker-conf.yaml` (comment only), `CHANGELOG.md`
- Test: `pkg/auth/service_test.go`, `pkg/auth/http_test.go`, `pkg/auth/config_test.go` (use the existing fake clock/miniredis helpers in the package)

**Interfaces — Produces (exact):**
```go
type SessionTimes struct{ ExpiresAt, MaxExpiresAt, ServerTime time.Time }
func (s *Service) Refresh(ctx context.Context, token string) (*SessionTimes, error) // ErrInvalidSession when invalid/expired/at cap; ErrRefreshRateLimited when over 6/min per token
type LoginResult struct{ /* existing */ MaxExpiresAt, ServerTime time.Time }
func (c Config) IdleTimeout() time.Duration   // default 30m
func (c Config) MaxLifetime() time.Duration   // default 24h; falls back to legacy SessionTTL when new key unset
```
Redis TTL after login/refresh = `min(IdleTimeout, MaxLifetime − (now − CreatedAt))`. Sessions without `LastActiveAt` use `CreatedAt`. `Validate()` rejects idle ≤ 0 and max < idle.

- [ ] **Step 1: Write failing tests** with the fake clock: `TestAuthenticateDoesNotExtendSession` (login, 100 `Authenticate` calls spread over 31 min → last one returns `ErrInvalidSession`); `TestRefreshExtendsToIdle` (after 20 min, refresh → TTL back to 30 min, `ExpiresAt = now+30m`); `TestRefreshCappedAtMax` (at 23h59m refresh gives `ExpiresAt = created+24h`; at 24h refresh returns `ErrInvalidSession`); `TestRefreshRateLimited` (7th call within a minute); `TestPasswordChangeInvalidatesAfterRefresh`; `TestLegacySessionTTLUsedAsMax` and `TestConfigValidateRejectsBadTimes`; HTTP tests: login body has `maxExpiresAt` and `serverTime`, `POST /auth/refresh` returns 200/401/429 with `Retry-After`, `GET /auth/me` has the new fields.
- [ ] **Step 2: Run** `go test ./pkg/auth/...` — Expected: FAIL.
- [ ] **Step 3: Implement** per Interfaces and Global Constraints; rate limit with a Redis counter `auth:refresh:<hash(token)>` (INCR + EXPIRE 60s). Legacy sessions already stored keep working until their Redis TTL ends.
- [ ] **Step 4: Run gate** `go build ./... && go vet ./pkg/auth/... && go test ./pkg/auth/... ./service/worker/...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(auth): idle timeout, absolute cap and POST /auth/refresh`.

### Task 2: UI session keeper

**Files:**
- Create: `ui/src/utils/composables/useSessionKeeper.ts`, `ui/src/components/auth/SessionExpiryDialog.vue`, `ui/src/components/auth/i18nSession.ts`, `ui/src/utils/helpers/ActivityTracker.ts`
- Modify: `ui/src/stores/authStore.ts` (state `expiresAt`, `maxExpiresAt`, `serverOffsetMs`, actions `refreshSession()`), `ui/src/services/AuthService.ts` (`refresh()`, login/me types), `ui/src/App.vue` or the layout that hosts the dialog, `ui/src/http/authInterceptor.ts` only if needed
- Test: `ui/src/utils/composables/__tests__/useSessionKeeper.spec.ts`, `ui/src/utils/helpers/__tests__/ActivityTracker.spec.ts`, `ui/src/components/auth/__tests__/SessionExpiryDialog.spec.ts`, `ui/src/stores/__tests__/authSession.spec.ts`

**Interfaces — Consumes:** the Wire contract above (mock `POST /auth/refresh`).
**Interfaces — Produces:**
```ts
// useSessionKeeper(deps?): { remainingIdleMs: Ref<number>, remainingMaxMs: Ref<number>, warning: Ref<'none'|'idle'|'max'>, extend(): Promise<void>, stop(): void }
// ActivityTracker: start(onActivity: (at: number) => void), stop(); listens to pointerdown, keydown, wheel, touchstart; throttled to 1 call per 5 s; never counts programmatic requests.
```
Behavior: server offset = `serverTime − Date.now()` at the response; deadlines computed with `Date.now() + offset`. Quiet refresh when there was user activity since the last refresh and fewer than 20 minutes (of the 30) remain. Warning `idle` when ≤ 2 min remain (dialog with countdown, "Continue" calls `extend()`, "Sign out" logs out); warning `max` when the cap is ≤ 5 min away (banner text with local time `hh:mm`, no extend). Cap reached or 401 from refresh: `authStore.expire('idle'|'max')` clears the session and routes to `/login?redirect=<current path>&reason=idle|max`; the login page shows a message for `reason`. Network error on refresh: retry after 10 s, up to 3 times, no logout. Multi-tab: `BroadcastChannel('tutti-session')` (fallback `storage` event) messages `{type:'refreshed', expiresAt, maxExpiresAt}` and `{type:'logout'}`.

- [ ] **Step 1: Write failing tests** with fake timers and mocked `AuthService.refresh`: polling requests do not trigger refresh; pointer activity then 11 minutes triggers one refresh; dialog appears at 2:00 remaining and "Continue" refreshes; `max` banner at 5:00 before the cap; clock skew of +10 min still shows the correct remaining time; 401 on refresh logs out with `reason=idle`; network error retries 3× without logout; BroadcastChannel message from another tab updates the deadline and a `logout` message signs out; locale keys for en/de/vi exist.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/utils src/components/auth src/stores` — Expected: FAIL.
- [ ] **Step 3: Implement** per Interfaces; the dialog uses the kit components (`@/components/kit`).
- [ ] **Step 4: Run UI gate** from `ui/`: `corepack yarn lint:types`, `corepack yarn test:unit --run`, `corepack yarn lint:eslint <changed files>`, `npx prettier --check <changed files>` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(ui): sliding session keeper with expiry warnings`.

### Task 3: Integration and real run

**Files:** read all; small fixes only where integration breaks. New: `docs/superpowers/plans/2026-11-13-session-refresh-verification.md`.

- [ ] **Step 1: Full gate** `go build ./... && go vet ./... && go test ./pkg/... ./service/...`; from `ui/`: `lint:types`, `test:unit --run`, `lint:eslint`, `build`.
- [ ] **Step 2: Real run** in an isolated compose project `sess` (own ports, `--memory` limits ≤ 1.5 GiB total, RAM guard `MemAvailable ≥ 1500 MiB`), worker env `CFG_API_AUTH_SESSIONIDLETIMEOUT=2m`, `CFG_API_AUTH_SESSIONMAXLIFETIME=5m`, `TUTTI_BOOTSTRAP_ADMIN_PASSWORD` set for that project only (do not touch project `docker-compose`). Via the API: login returns both times; `GET /auth/me` called every 10 s for 3 minutes does NOT keep the session alive (401 after ~2 min without refresh); a refresh at 1m30s extends to +2m; refresh at 4m30s returns `expiresAt` = cap; after 5m refresh → 401; 7th refresh in a minute → 429. In the browser (Chromium via playwright-core as before) with the same stack: warning dialog at ~2:00 remaining, "Continue" extends, max banner at 5:00 before cap is shown (use `maxLifetime=8m` for that case), expiry redirects to `/login?reason=idle` with the message; two tabs sync. Remove everything created; verify with `docker ps -a`, `docker volume ls`, `docker network ls`, `docker images`.
- [ ] **Step 3: Write the verification report and commit** `chore: verify sliding sessions`.
