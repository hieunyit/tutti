# UI redesign verification (Task 8)

## Gate (from `ui/`)
- `lint:types` pass, `test:unit --run` 65 files / 461 tests pass, `lint:eslint` pass (0 warnings), `lint:stylelint` pass, `build` pass (only the existing large swagger-ui chunk warning).
- Prettier is clean for every file changed by the branch; the 24 older files that failed before the branch were left alone.

## Fixed during integration
- kit.spec.ts: `rowKey` typing broke `vue-tsc`.
- Colour scheme: `main.ts` forced dark as the default, so the app ignored the OS light setting. It now follows the OS (`initColorScheme()`); a saved choice still wins.
- Replication wizard: `createReplication` no longer navigates away or clears the draft when every request fails. It now returns `true` only if at least one replication was created (tests added for both outcomes).
- Kit `DataTable`: additive `rowClickable` prop and `rowClick` emit (clicks on links, buttons, checkboxes and menus are ignored). The Storages list uses it, so a whole row opens the details.
- `StorageSettings.vue` 407 -> 374 lines (dead styles removed); `TuttiStoragesPage.vue` is 400.
- `ProtectionGroupDialog.vue`: dead create branch removed (it is edit-only), 561 -> 346 lines; `group` is now a required prop.
- Dead code removed: old Routing/Diff list pages, old Tile/List/ActionsCell/Wizard components under `components/chorus`, `HomeStatistics`/chart widgets, `ProxyCredentialsModal`, `tutti/common` TabBar/RowActionButton/ConfirmDialog/FilterBar, `components/ui` KpiCard/CopyField, `useQueryTab` (kit has its own), `tuttiRootStore`, `tuttiLegacyStoragesStore`, `tuttiStorageDetailsStore`, `LogHelper`, `phaseD.scss`, `i18nUi` and their orphaned i18n files. An import-graph check from `main.ts` found no other unreachable source (apart from the exceptions below).
- Redirect tests: now also cover `/access-keys`, the legacy credentials URLs, and viewer vs admin on `/console-users`, `/system-settings`, `/audit-log` through the real auth guard (viewer lands on `/dashboard`). `/diff-reports/detail?id=X` keeps the id (already covered).

## Visual check (headless Chromium against the rebuilt web-ui, http://localhost:8080)
All 16 routes (Dashboard overview and metrics, Storages, Proxies fleet and config, Protection groups, Replication x3 tabs, S3 users, Proxy accounts, Console users, System settings, Audit log, Docs, API reference) x light/dark x 1280/375: no horizontal overflow, none blank, an h1 on every page, empty states with a CTA render (dev stack has no data). No console errors apart from expected `401 /api/auth/me` before login and `501 /api/proxy` (the dev backend does not implement it; the dashboard shows dashes instead of blanking).
Screenshots are in the session scratchpad (`.../scratchpad/shots/`), not in the repo.

## Remaining
- `AccessPolicyDrawer.vue` is still 800 lines: its form state (grantMode, group and bucket selection) is shared across the whole template, so a split would change behaviour. Left as is.
- Other files over 400 lines: `AppSidebar.vue` 466 and `DocsGuides.vue` 613 (both pre-existing layout/docs files).
- Proxy accounts page title says "Access keys" while the menu says "Proxy accounts".
- At 375px the Replication tab shows six stacked filters before the list; consider collapsing them.
- At 375px the topbar wraps onto two rows in some widths.
- `tuttiFailoverStore.ts` and `tuttiHealthStore.ts` are only used by specs; kept as asked.
- Only the empty and top-level states were seen live (no seed data); populated tables, the error state and the loading skeleton are covered by unit tests, not by this browser run.
