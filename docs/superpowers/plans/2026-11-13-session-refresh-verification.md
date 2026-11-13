# Sliding sessions: verification report (2026-11-13)

## Gate
- Go: `go build ./... && go vet ./pkg/... ./service/... && go test ./pkg/... ./service/...` all green (after the two fixes below).
- UI: `lint:types`, `test:unit --run`, `lint:eslint`, `build` all green.

## Fixed during verification
1. `ChangeOwnPassword` -> `restamp` set `CreatedAt = now`, restarting the 24 h cap (commit ac2f29b). New `Session.StartedAt` (original login) is the only anchor of the absolute cap; `CreatedAt` still drives the "password changed after session" check; `restamp` keeps StartedAt (legacy sessions without it fall back to CreatedAt). Tests: `TestOwnPasswordChangeKeepsOriginalCap` (23h40 old session changes password: other session revoked, own continues, refresh gives ExpiresAt = MaxExpiresAt = start+24h), `TestLegacySessionWithoutStartedAtUsesCreatedAt`.
2. `CFG_API_AUTH_SESSIONIDLETIMEOUT` / `CFG_API_AUTH_SESSIONMAXLIFETIME` made the worker exit (`no matching config field found`): the env loader only overrides keys present in YAML and the new keys were only comments in `service/worker/config.yaml`. Both keys are now declared (`0s` = built-in default); test `TestSessionTimeoutEnvOverrides` (commit 9abf1ca).

## Real run (compose project `sess`, worker 49671, web-ui 48080, memory limits 256m+128m+768m+128m)
Worker env: IDLE=2m, MAXLIFETIME=5m, bootstrap admin password set, FORCE_CHANGE=false. Login: `expiresAt` = serverTime+2m, `maxExpiresAt` = serverTime+5m, `serverTime` present (env took effect).

| Check | Result |
|---|---|
| `/auth/me` + `/storage` every 10 s for 3 min do not keep session alive | PASS: 200 until 111 s, first 401 at 121.0 s |
| Refresh at 90.5 s | PASS: expiresAt = 14:58:09 + 2m = 15:00:09 |
| Refresh near cap (login 14:56:39, cap 15:01:39; refresh at 201 s, 15:00:00) | PASS: expiresAt = maxExpiresAt = 15:01:39 (uncapped would be 15:02:00) |
| Refresh after cap (305.4 s) | PASS: 401 `authentication required` |
| 7th refresh in a minute | PASS: #1-#6 200, #7 429 `too many session refreshes`, `Retry-After: 60` |

Browser (Chromium 1248 via playwright-core). Warning at 2:00 remaining needs idle > 2m (with idle=2m the dialog is due immediately), so the worker was restarted with IDLE=4m, MAXLIFETIME=8m for this part.

| Check | Result |
|---|---|
| Idle dialog appears at 2:00 remaining, not before | PASS: first visible 121.6 s after login (expected 120 s) |
| "Continue session" extends | PASS: clicked at 129.7 s, dialog closed; expiry moved to ~369.7 s |
| Active user (click, key, mouse for 170 s) gets no dialog | PASS (dialogSeen=false; note below) |
| Max banner 5 min before cap | PASS: 180.9 s (cap 480 s) |
| Second dialog after Continue | PASS: 250.9 s (expected 249.7 s) |
| Expiry redirect | PASS: 371.0 s to `/login?redirect=/dashboard&reason=idle`, notice "You were signed out after a period of inactivity." |
| Second tab follows logout | PASS: tab2 (own login, same context) at `/login` 70 ms after sign out in tab1 |

Screenshots: `/tmp/claude-1000/-home-hellc-tutti/5a90509e-a3c8-4966-8436-f12e94d9ab2d/scratchpad/sess-shots/` (not in repo). Everything of project `sess` (containers, volumes `sess_*`, network, images `sess-worker:test`, `sess-web-ui:test`) removed; the `docker-compose` project was untouched.

## Remaining notes
- Activity is pointerdown/keydown/wheel/touchstart (throttled 5 s); plain mouse movement does not count as activity by design.
- Token is held in memory per tab, so each tab has its own session; the tab-sync check logs in separately in both tabs.
- With very short idle windows (< 20 m remaining always true, e.g. the 2m test) a quiet refresh can fire on every activity burst; the server limit of 6 refreshes per minute then returns 429, which the UI retries without logging out. With the 30m default this does not occur.
- Sessions created before the upgrade have no `startedAt`; they use `createdAt` for the cap (as before).
