# UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the Tutti console UI with a new light/dark design system, a 10-item menu, and redesigned pages, without touching the backend.

**Architecture:** WP0 builds the foundation (tokens, theme, shared `components/kit`, layout shell, routes, redirects). WP1–WP7 then redesign page groups in parallel, each owning disjoint files and importing only from the kit. WP8 integrates, builds and visually verifies.

**Tech Stack:** Vue 3.5, Naive UI 2.43, Pinia 3, vue-router 4.6, vue-i18n 11, Vite 7, Vitest 4, vue-tsc.

**Spec:** `docs/superpowers/specs/2026-11-13-ui-redesign-design.md` (section numbers below refer to it).

## Global Constraints

- Only `ui/` changes. No backend, proto or API changes. Route *names* are never renamed, only added.
- Library stays Naive UI. Theme sáng/tối; default follows OS via `stores/colorSchemeStore.ts`.
- Colors/spacing/radius only through `--tt-*` variables from `src/styles/tokens.scss`; no hard-coded colors.
- Every string through vue-i18n with `en`, `de`, `vi`. Button labels are verbs ("Tạo storage"), never "OK"/"Submit".
- Every page: loading skeleton (>100ms), empty state with CTA, error state with "Thử lại", write actions wrapped in `WriteGate`.
- No page file over 400 lines. Keep existing store/service public APIs; split components and composables instead.
- Quality gate per task (run from `/home/hellc/tutti/ui`): `corepack yarn lint:types`, `corepack yarn test:unit --run <scope>`, `corepack yarn lint:eslint <changed files>`, `corepack yarn lint:prettier`.
- Task reports to the controller are at most 10 lines. Do not commit files outside your owned scope.
- Old URLs must keep working (redirects, spec section 2).

## Review Focus

- Deep link to an old URL with query string (`/diff-reports/detail?id=X`) must land on the new route with the id preserved.
- Viewer (non-admin) role must not see Console users, System settings, Audit log, nor reach them by URL.
- A page whose API call fails must show `ErrorBlock` with working retry, not a blank page (the original symptom: blank page when the backend was unreachable).
- Wizard draft survives navigating away and back; "Làm lại từ đầu" clears it.
- One-time secrets (proxy account secret, console-user password, S3 user key) are shown once and are never written to localStorage or the URL.

---

## Dependency order

`WP0` → (`WP1`, `WP2`, `WP3`, `WP4`, `WP5`, `WP6`, `WP7` in parallel) → `WP8`.

### Task 0 (WP0): Foundation — tokens, kit, layout, routes

**Files:**
- Modify: `ui/src/styles/tokens.scss`, `ui/src/styles/theme.ts`, `ui/src/App.vue`
- Create: `ui/src/components/kit/{PageShell,DataTable,FilterBar,StatTile,DetailLayout,FormSection,WizardShell,ConfirmDialog,StateBlock,StatusBadge,CopyValue}.vue`, `ui/src/components/kit/index.ts`, `ui/src/components/kit/i18nKit.ts`
- Modify: `ui/src/components/layout/{AppSidebar,AppBreadcrumbs,PageHeader,AppQuickSearch,AppThemeToggle}.vue`; Create `ui/src/components/layout/AppTopbar.vue`, `HelpMenu.vue`
- Modify: `ui/src/router/routes.ts`, `ui/src/utils/constants/navigation.ts`, `ui/src/utils/types/router.ts`, `ui/src/components/layout/i18nLayout.ts`
- Test: `ui/src/components/kit/__tests__/*.spec.ts`, `ui/src/router/__tests__/redirects.spec.ts`, update `ui/src/components/layout/__tests__/*`

**Interfaces — Produces (consumed by WP1–WP7, exact):**
- `PageShell` props: `title: string`, `description?: string`, `state: 'loading' | 'empty' | 'error' | 'ready'`, `tabs?: {key: string; label: string; badge?: number}[]`, `activeTab?: string`; emits `update:activeTab`, `retry`; slots `actions`, `default`, `empty`.
- `DataTable` props: `columns: DataTableColumns<T>`, `rows: T[]`, `loading: boolean`, `error?: string`, `rowKey: (r: T) => string`, `selectable?: boolean`, `density?: 'comfortable' | 'compact'`; emits `retry`, `select(keys: string[])`; slots `empty`, `bulk-actions`.
- `ConfirmDialog` props: `open: boolean`, `title: string`, `targetName?: string`, `requireTyping?: boolean`, `danger?: boolean`, `confirmLabel: string`; emits `confirm`, `cancel`.
- `WizardShell` props: `steps: {key: string; label: string; valid: boolean}[]`, `draftKey: string`; emits `finish`; slots per step key.
- `StatusBadge` props: `tone: 'success' | 'warning' | 'danger' | 'info' | 'neutral'`, `label: string`.
- `CopyValue` props: `value: string`, `oneTime?: boolean` (shows value once, requires "đã lưu" tick before closing).
- `useQueryTab(key = 'tab', fallback: string): Ref<string>` exported from `kit/index.ts` (wraps the existing `useQueryTab`).

- [ ] **Step 1: Write failing kit tests** — `kit/__tests__/PageShell.spec.ts` asserting each `state` renders skeleton / empty slot / `ErrorBlock` with a button that emits `retry` / default slot; `ConfirmDialog.spec.ts` asserting confirm stays disabled until `targetName` is typed when `requireTyping`; `redirects.spec.ts` asserting every old path in spec section 2 resolves to the new route and preserves `query`.
- [ ] **Step 2: Run `corepack yarn test:unit --run src/components/kit src/router`** — Expected: FAIL (components/routes missing).
- [ ] **Step 3: Update tokens/theme** — add `--tt-space-1..8`, `--tt-radius-{control,card,modal}`, `--tt-accent`, semantic colors, light + dark values in `tokens.scss`; mirror in `theme.ts` Naive overrides (spec section 3). Keep every existing `--tt-*` name.
- [ ] **Step 4: Implement the 11 kit components** with the exact props above; keep old `components/ui/*` and `components/tutti/common/*` as re-exports of kit equivalents so existing imports compile.
- [ ] **Step 5: Layout** — `AppTopbar` (breadcrumbs, quick search Ctrl/Cmd+K, `HelpMenu` linking `/docs` and `/docs/api`, theme toggle, user menu); sidebar reads the new `NAV_GROUPS` (5 groups, 10 items, `adminOnly` for Console users, System settings, Audit log); remove the `help` group.
- [ ] **Step 6: Routes** — add `/dashboard` (alias route name `TUTTI_DASHBOARD` rendering the current Home page until WP1 swaps it), `/replication` tabs routes, `/replication/routing/add`, `/replication/diff/add`, `/replication/diff/:id`; add all redirects from spec section 2 (preserve `query`); `/` redirects to `/dashboard`.
- [ ] **Step 7: Run gate** (`lint:types`, tests for `components/kit components/layout router`, eslint, prettier) — Expected: all pass.
- [ ] **Step 8: Commit** — `git add ui/src/styles ui/src/components/kit ui/src/components/layout ui/src/router ui/src/utils ui/src/App.vue && git commit -m "feat(ui): add design tokens, kit components, new layout and routes"`

### Task 1 (WP1): Dashboard — spec 4.1

**Files:** Create `ui/src/pages/DashboardPage/{DashboardPage.vue,DashboardOverviewTab.vue,DashboardMetricsTab.vue,useDashboardData.ts,i18nDashboard.ts}`; Modify `ui/src/components/dashboard/*`, `ui/src/components/monitoring/*`; Remove (after redirect works) `pages/TuttiHomePage`, `pages/TuttiMonitoringPage`; Modify route target in `routes.ts` only for the `TUTTI_DASHBOARD` component import line (the single line WP0 left as placeholder).
**Interfaces — Consumes:** `PageShell`, `StatTile`, `StatusBadge`, `CopyValue`, `useQueryTab` (Task 0). **Produces:** `DashboardPage` default export.

- [ ] **Step 1: Write failing tests** — `useDashboardData.spec.ts`: returns `state === 'error'` when `useTuttiDashboardStore` rejects, `'ready'` otherwise; `DashboardPage.spec.ts`: renders tabs `overview`/`metrics`, `?tab=metrics` selects the second.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/pages/DashboardPage` — Expected: FAIL.
- [ ] **Step 3: Implement** the page, two tabs and `useDashboardData(): { state, refresh, updatedAt }` with 30s auto-refresh only while the tab is visible; widgets keep their own `StateBlock`. Prometheus widgets only when `HAS_PROMETHEUS`.
- [ ] **Step 4: Run gate**, then commit `feat(ui): redesign dashboard with overview and metrics tabs`.

### Task 2 (WP2): Storages and Proxies — spec 4.2, 4.3, 4.4

**Files:** Modify `ui/src/pages/{TuttiStoragesPage,TuttiStorageDetailsPage,TuttiProxiesPage}/*`, `ui/src/components/tutti/{storages,health}/*`; Create `ui/src/pages/TuttiProxiesPage/{ProxyFleetTab.vue,ProxyConfigTab.vue}`, `ui/src/stores/{tuttiProxiesStore.ts,tuttiProxyFleetConfigStore.ts}`; `TuttiHealthPage` becomes a redirect (component removed, route already redirects).
**Interfaces — Consumes:** kit from Task 0; existing `TuttiService.getProxies/getProxyFleetConfig/setProxyFleetConfig`. **Produces:** `useTuttiProxiesStore(): { proxies, loading, error, load(), startPolling(ms), stopPolling() }`, `useTuttiProxyFleetConfigStore(): { draft, dirty, save(): Promise<void>, reset() }`.

- [ ] **Step 1: Write failing tests** — store specs: `startPolling(15000)` calls `getProxies` again after fake-timer advance and stops after `stopPolling()`; `dirty` becomes true after editing `draft`; `save` calls `setProxyFleetConfig` once with the draft.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/stores src/pages/TuttiProxiesPage` — Expected: FAIL.
- [ ] **Step 3: Implement** stores (move page-local state/interval out of the page), Fleet and Config tabs (5 `FormSection`s, save through `ConfirmDialog` showing before/after), Storages list with `DataTable` + filters, Storage details with `DetailLayout` and 4 tabs, per-group settings forms split from the 468-line `StorageSettings`.
- [ ] **Step 4: Run gate**, commit `feat(ui): redesign storages and proxies pages`.

### Task 3 (WP3): Protection groups — spec 4.5, 4.6

**Files:** Modify `ui/src/pages/{TuttiProtectionGroupsPage,TuttiProtectionGroupDetailPage,TuttiFailoverPage}/*`, `ui/src/components/tutti/{protection,failover}/*`; Create `ui/src/pages/TuttiProtectionGroupDetailPage/{MembersTab,WritersTab,ActivityTab,SettingsTab}.vue`, `ui/src/utils/composables/useProtectionGroupActions.ts`.
**Interfaces — Consumes:** `DetailLayout`, `WizardShell`, `ConfirmDialog`, `DataTable`. **Produces:** `useProtectionGroupActions(name: Ref<string>): { switchover(target: string): Promise<void>, abortSwitch(): Promise<void>, clearWriter(w: string): Promise<void>, remove(): Promise<void> }` (each wraps `ProtectionGroupService` and refreshes `useTuttiProtectionStore`).

- [ ] **Step 1: Write failing tests** — composable spec: `remove()` calls `ProtectionGroupService.remove(name)` and then the store `load`; `ConfirmDialog` for delete requires typing the group name (component spec).
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/pages/TuttiProtectionGroupDetailPage src/utils/composables` — Expected: FAIL.
- [ ] **Step 3: Implement** list with freeze toggle + `DataTable`, creation as a 3-step `WizardShell` (`ProtectionGroupDialog` split), detail with 4 tabs; page files ≤ 250 lines.
- [ ] **Step 4: Run gate**, commit `feat(ui): redesign protection groups pages`.

### Task 4 (WP4): Replication hub — spec 4.7 (split into 4a and 4b if one agent runs out of context)

**Files:** Create `ui/src/pages/TuttiReplicationPage/{ReplicationHub.vue,ReplicationsTab.vue,RoutingTab.vue,DiffTab.vue}`; Modify `ui/src/pages/{TuttiReplicationPage,TuttiRoutingPoliciesPage,TuttiDiffReportsPage,TuttiAddReplicationPage,TuttiAddRoutingPolicyPage,TuttiAddDiffReportPage,TuttiDiffReportDetailPage}/*` and the related `ui/src/components/tutti/**`; Create `ui/src/utils/composables/useRowActions.ts`.
**Interfaces — Consumes:** `DataTable`, `FilterBar`, `WizardShell`, `DetailLayout`, `useQueryTab`. **Produces:** `useRowActions<T>(defs: {key: string; label: string; danger?: boolean; run: (row: T) => Promise<void>; confirm?: (row: T) => string}[]): { actionsFor(row: T), running: Ref<Set<string>> }`.

- [ ] **Step 1: Write failing tests** — `useRowActions.spec.ts`: a `danger` action with `confirm` opens the dialog and runs only after confirmation; `ReplicationHub.spec.ts`: `?tab=routing` and `?tab=diff` select the right tab; wizard draft test: after filling step 1 and remounting with the same `draftKey`, step 1 values are restored.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/pages/TuttiReplicationPage src/utils/composables` — Expected: FAIL.
- [ ] **Step 3: Implement** hub with 3 tabs on the existing stores (`useTuttiReplicationsStore`, `useTuttiRoutingPoliciesStore`, `useTuttiDiffReportsStore`), replace the three Tile/ListActions/ActionsCell trios with `DataTable` + `useRowActions`, convert the three Add* pages to `WizardShell` with drafts, add loading/error to Add* hosts and Diff detail.
- [ ] **Step 4: Run gate**, commit `feat(ui): merge replication, routing and diff pages into a replication hub`.

### Task 5 (WP5): Access — spec 4.8, 4.9, 4.10

**Files:** Modify `ui/src/pages/{TuttiUsersPage,TuttiUserCreatePage,TuttiUserDetailPage,TuttiProxyAccountsPage,TuttiConsoleUsersPage}/*`, `ui/src/components/tutti/{users,accounts}/*`; Create `ui/src/pages/TuttiUserCreatePage/{IdentityStep,PermissionStep,KeysStep}.vue`, `ui/src/utils/composables/useUserCreate.ts`.
**Interfaces — Consumes:** `WizardShell`, `DetailLayout`, `ConfirmDialog`, `CopyValue(oneTime)`, `DataTable`. **Produces:** `useUserCreate(): { form, step, canNext, submit(): Promise<{ secret: string }> }`.

- [ ] **Step 1: Write failing tests** — `useUserCreate.spec.ts`: `canNext` false until the name is valid; `submit` calls `TuttiService.createUser` once; secret is not present in `localStorage` after submit. `ConsoleUsersPage.spec.ts`: current user's row has delete and role-change disabled.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/pages/TuttiUserCreatePage src/pages/TuttiConsoleUsersPage` — Expected: FAIL.
- [ ] **Step 3: Implement** users list with `SegmentedFilter` + `DataTable` (empty vs filtered-zero distinct), create wizard split into 3 steps, user detail with `DetailLayout`/3 tabs and a danger zone (typed-name delete), `AccessPolicyDrawer` split by permission group, proxy accounts table, console users table with one-time password.
- [ ] **Step 4: Run gate**, commit `feat(ui): redesign access pages`.

### Task 6 (WP6): System settings and Audit log — spec 4.11, 4.12

**Files:** Modify `ui/src/pages/{TuttiSystemSettingsPage,TuttiAuditLogPage}/*`, `ui/src/components/tutti/settings/*`; Create `ui/src/pages/TuttiAuditLogPage/{AuditFilters,AuditTable,AuditEntryDrawer}.vue`, `ui/src/stores/tuttiAuditLogStore.ts`, `ui/src/utils/composables/useAuditCursor.ts`.
**Interfaces — Produces:** `useAuditCursor(fetchPage: (cursor?: string) => Promise<{items: AuditEntry[]; next?: string; prev?: string}>): { items, loadOlder(), loadNewer(), loading, error }`.

- [ ] **Step 1: Write failing tests** — `useAuditCursor.spec.ts`: `loadOlder()` passes the last `next` cursor and appends; an error keeps already loaded items and sets `error`. `SystemSettings` spec: save button disabled when not dirty; version conflict shows the reload/overwrite dialog.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/pages/TuttiAuditLogPage src/pages/TuttiSystemSettingsPage` — Expected: FAIL.
- [ ] **Step 3: Implement** audit log split (page ≤ 200 lines) and settings page with tabs, sticky save bar, diff summary, "Cần restart worker" labels for startup-only fields (concurrency, intervals, shutdownTimeout, lock overlap).
- [ ] **Step 4: Run gate**, commit `feat(ui): redesign system settings and audit log`.

### Task 7 (WP7): Auth and Docs — spec 4.13

**Files:** Modify `ui/src/pages/{LoginPage,TuttiChangePasswordPage,DocsPage.vue}`, `ui/src/components/auth/*`, `ui/src/components/docs/*`; Create `ui/src/utils/composables/useLoginFlow.ts`.
**Interfaces — Produces:** `useLoginFlow(): { form, submit(): Promise<void>, errorMessage: Ref<string>, remainingAttempts: Ref<number | null>, initFailed: Ref<boolean>, retryInit(): Promise<void> }`.

- [ ] **Step 1: Write failing tests** — `useLoginFlow.spec.ts`: a 401 response with `remainingAttempts: 8` sets `remainingAttempts` to 8 and a non-empty `errorMessage`; `initFailed` true when `/auth/me` is unreachable and `retryInit` re-calls it.
- [ ] **Step 2: Run** `corepack yarn test:unit --run src/pages/LoginPage` — Expected: FAIL.
- [ ] **Step 3: Implement** the single-column login (show/hide password, attempts left, unreachable-server state with "Thử lại"), change-password with rules and loading/error, docs with swagger skeleton.
- [ ] **Step 4: Run gate**, commit `feat(ui): redesign login, change password and docs`.

### Task 8 (WP8): Integration and visual check

**Files:** Read all of `ui/`; small fixes anywhere under `ui/src` only to resolve integration breaks.

- [ ] **Step 1: Full gate** — `corepack yarn lint`, `corepack yarn lint:types`, `corepack yarn test:unit --run`, `corepack yarn build` — Expected: all pass, no warnings above the existing baseline.
- [ ] **Step 2: Redirect sweep** — for each old path in spec section 2 (including a query-string case), assert it lands on the new route (extend `router/__tests__/redirects.spec.ts` if a case is missing).
- [ ] **Step 3: Rebuild and run the stack** — `cd /home/hellc/tutti/docker-compose && docker compose --profile proxy up -d --build web-ui && docker compose restart web-ui`.
- [ ] **Step 4: Visual check** — with the browser tools, open every menu page in light and dark at 1280px and 375px; confirm no horizontal overflow, 4 states render, no console errors. Record findings in `docs/superpowers/plans/2026-11-13-ui-redesign-verification.md`.
- [ ] **Step 5: Fix and commit** — `git add ui docs && git commit -m "chore(ui): integrate redesign and verify"`.
