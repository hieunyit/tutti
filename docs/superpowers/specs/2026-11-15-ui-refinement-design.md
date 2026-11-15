# UI refinement: single source of truth (phase 1 foundation)

Status: approved by the user ("đồng ý, làm đi"). Based on the UI audit (parts A to E).
Phase 1 (this foundation: kit, Naive locale, navigation) ships first; phase 2 runs
one agent per work package (WP) below. Earlier design: `2026-11-13-ui-redesign-design.md`.

## 1. Information architecture

Sidebar, 9 items in 4 groups plus the lone Dashboard. Definition lives in one file:
`ui/src/utils/constants/navigation.ts` (groups, items, icons, route, roles, badge).

```
Dashboard                       (no group label; tabs: Overview | Traffic)
CONNECT     Storages, Proxies   (Proxies = fleet status only)
REPLICATE   Replication         (tabs: Replications | Diff reports)
            Protection groups
            Routing rules       (own page)
ACCESS      Users               (was "S3 users")
            Access keys         (was "Proxy accounts")
ADMIN       Console users, System settings, Audit log   (admin role only)
Help menu   Documentation, API reference (not in the sidebar)
```

Old -> new mapping (old URLs stay as redirects without UI, query and hash kept):

| old | new |
|---|---|
| `/home`, `/` | `/dashboard` |
| `/dashboard?tab=metrics`, `/monitoring`, `/health` | `/dashboard?tab=traffic` (`metrics` still accepted), `/health` -> `/proxies` |
| `/proxies?tab=config` | `/system-settings?tab=proxies` |
| `/replication?tab=routing`, `/routing-policies` | `/routing` |
| `/replication/routing/add`, `/routing-policies/add` | `/routing/add` |
| `/diff-reports`, `/diff-reports/add`, `/diff-reports/detail` | `/replication?tab=diff`, `/replication/diff/add`, `/replication/diff/:id` |
| `/failover` | `/protection-groups` |
| `/proxy-accounts`, `/access-keys` | `/access-keys` (canonical; route name `TUTTI_ACCESS_KEYS`) |
| `/replication/detail?from&to` | unchanged (deep link kept) |
| `/storages/:s/credentials/:alias?` | user detail or storage `?tab=users` (unchanged) |

Roles (existing mechanism, `authStore.isAdmin` / `canWrite` / `isViewer`, route meta
`requiresAdmin` / `requiresWrite`, `WriteGate` for mutating controls):
- admin: everything. viewer: read-only Dashboard (Overview only), Storages, Proxies,
  Replication, Protection groups, Routing rules, Users, Access keys; every mutating control sits in `WriteGate`.
- Hidden from viewers in sidebar, quick search and route guards: the Dashboard Traffic tab
  (`?tab=traffic|metrics` falls back to Overview), and all Admin items (Console users, System settings, Audit log).
- The route meta `requiresAdmin` is the single guard flag; a nav item must carry `adminOnly` exactly when its route does (unit-tested).

## 2. Glossary (one word per concept; use it in UI text, i18n keys, docs and tests)

| concept | en | de | vi |
|---|---|---|---|
| Storage (an S3 backend) | Storage | Speicher | Storage |
| Proxy | Proxy / Proxies | Proxy / Proxys | Proxy |
| Replication (copy of buckets between storages) | Replication | Replikation | Replication |
| first full copy | Initial copy | Erstkopie | Sao chép ban đầu |
| Source / Destination | Source / Destination | Quelle / Ziel | Nguồn / Đích |
| Protection group | Protection group | Schutzgruppe | Nhóm bảo vệ |
| Routing rule (never "policy") | Routing rule | Routing-Regel | Routing rule |
| User (an S3 user; never identity, account) | User | Benutzer | User |
| Access key (never proxy account, credential) | Access key | Zugriffsschlüssel | Access key |
| Console user (signs in to this console) | Console user | Konsolenbenutzer | Người dùng console |
| Failover | Failover | Failover | Failover |
| Diff report | Diff report | Diff-Report | Báo cáo diff |
| Traffic (was Metrics) | Traffic | Datenverkehr | Lưu lượng |
| Status words | Healthy, Degraded, Down; Active, Paused | Fehlerfrei, Eingeschränkt, Ausgefallen; Aktiv, Pausiert | Hoạt động tốt, Suy giảm, Ngừng; Đang chạy, Tạm dừng |

Avoid: endpoint/backend (Storage), migration/sync (Replication), From/To/Target (Source/Destination),
identity/S3 user (User), proxy account/credential (Access key), policy (Routing rule),
circuit breaker, fail-fast, Control-Plane, transactional (jargon: only inside a "Details" section).
vi keeps English loanwords for product nouns (Storage, Proxy, Replication); do not mix `Sao chép` for the same noun.
Sentence case everywhere ("Add storage", "Created at"). Never show raw enum values (map to labels).

## 3. Patterns (kit components in `ui/src/components/kit`, import from `@/components/kit` only)

**Page header** (`PageHeader`, used by `PageShell`): breadcrumb is in the topbar. Row: H1 | optional ONE-line description
(only when it adds what the title lacks; omit on self-explanatory pages) | right side: at most one primary
action plus a secondary actions menu (`secondaryActions`, a "More" dropdown). No refresh button when data
auto-refreshes. Never repeat the H1 as a card or section title.

**Table toolbar** (`TableToolbar`): `[search] [Filters (n)] ... [bulk actions]`. Filters are collapsed by default,
active filters show as removable chips, "Clear filters" resets. Pass `hidden` (empty and unfiltered list) and the
whole toolbar disappears. Row actions: name is the primary link, ONE overflow menu; no header+row+menu duplicates.

**Empty state** (`EmptyState`/`EmptyBlock`): one title saying what is missing, ONE body sentence only if it explains a
prerequisite, ONE primary action (verb-first). Title and body must not repeat each other. While the empty state shows
its action, hide the same action in the page header. Filtered-zero: "No <things> match" + Clear filters.

**Wizard shell** (`WizardShell`): only for more than 3 dependent steps; otherwise one form with defaults. Visible
"Step n of m" + progress, Back/Next/Finish footer, optional `summary` slot, no "Start over" on step 1, step names
are not repeated as headings inside the step. Draft lives in `useWizardDraft`.

**Status badge** (`StatusBadge`): colored dot + text, tones success/warning/danger/info/neutral, built from
`--tt-*` tokens; map domain words with `mapStatusToVariant`. Never use a coloured pill for implementation facts.

**Section card** (`SectionCard`): a bordered card only when it adds information beyond its title (list, table, KPI).
Simple groupings use `variant="section"` (no border, no fill, heading + content). A card title never repeats the
page or tab title.

**Forms**: visible label on every field; placeholders only for format hints; no "Please Input/Select" defaults
(Naive locale sets them empty); blur-first validation; one primary action per page; destructive actions confirm with
the named object (`ConfirmDialog`). **KPI tiles**: at most 4, each with a decision value, no 0/0 tiles, no subtext
restating the label. **Tabs**: only for 3+ peer views, always in the URL (`useQueryTab`). **Tables** < 640 px scroll horizontally.

## 4. Copy, status and feedback rules

- No stutter: label, heading, placeholder, button and empty-state text must not repeat the same words.
- Do not restate the title in the description. Buttons are verb-first ("Add storage", "Create user", never OK/Submit).
- Loading: nothing under 100 ms, inline up to 1 s, skeleton beyond (kit blocks do this). Errors say what happened
  and offer recovery (retry); never blame the user. Success: a short toast, not a banner.
- One status text per fact (not "Saved version 2" + "Last updated" + footnote). Prefer auto-refresh over a Refresh button.
- Colors only from `--tt-*` tokens (no new hex values); animations respect `prefers-reduced-motion`
  (kit uses `--tt-duration-*`; global CSS removes them under reduced motion).
- Language: ONE switch, the topbar selector (en/de/vi). Pages must not carry their own language switch.
- Naive UI strings (`Please Input`, date pickers, empty table text) come from `src/i18n/naiveLocale.ts`.

## 5. Engineering rules

- No new file over 300 lines (split by component/composable); do not grow existing files past that.
- i18n: edit en, de and vi together (`localeCompleteness.spec.ts` enforces parity); prune keys that become unused
  (`grep` each key before removing). Route titles live in `ui/src/i18n/messages/routes.ts` (owned by phase 1; ask for changes).
- Tests: update every spec whose labels/routes you change; kit changes get a spec in `kit/__tests__`.
- Keyboard: every interactive element reachable and visible-focus; dialogs trap and restore focus.
- Use `RouteName` enum members, never literal paths, for links. New routes get meta (`navGroup`, `navSection`, roles) and a
  `navigation` entry only if they are sidebar items.

## 6. Work packages for phase 2 (disjoint file sets; paths under `ui/src`)

Common to all: do not edit `components/kit/**`, `utils/constants/navigation.ts`, `router/**`, `i18n/messages/routes.ts`,
`components/layout/**` (request changes from the lead). Page-local i18n files belong to the package that owns the page.

| WP | Pages and components owned |
|---|---|
| WP1 Dashboard | `pages/DashboardPage/**`, `components/tutti/dashboard/**`, `monitoring/**`, `throughput/**`, `home/**`, `stores/tuttiDashboardStore`, `tuttiMonitoringStore`, `tuttiWorkerStatsStore` |
| WP2 Storages | `pages/TuttiStoragesPage/**`, `pages/TuttiStorageDetailsPage/**`, `components/tutti/storages/**`, `stores/tuttiStoragesStore` |
| WP3 Proxies | `pages/TuttiProxiesPage/{TuttiProxiesPage,ProxyFleetTab,i18nProxies}`, `components/tutti/health/**`, `stores/tuttiProxiesStore`, `tuttiHealthStore` (ProxyConfigTab, ProxyScopeForm belong to WP8) |
| WP4 Replication | `pages/TuttiReplicationPage/{TuttiReplicationPage,ReplicationHub,ReplicationsTab,DiffTab,diffRoutes,i18nReplicationHub}`, `pages/TuttiReplicationDetailPage/**`, `pages/TuttiDiffReportDetailPage/**`, `components/tutti/replications/**`, `diff-reports/**`, `diff-report-detail/**`, stores `tuttiReplicationsStore`, `tuttiDiffReportsStore`, `tuttiDiffReportDetailStore`, `tuttiDiffReportEntriesStore` |
| WP4b Wizards | `pages/TuttiAddReplicationPage/**`, `pages/TuttiAddDiffReportPage/**`, `components/tutti/add-replication/**`, `add-diff-report/**`, `common/TuttiWizardPage/**`, `stores/tuttiAddReplicationStore`, `tuttiAddDiffReportStore` |
| WP4c Routing rules | `pages/TuttiRoutingPage/**`, `pages/TuttiReplicationPage/RoutingTab.vue` (delete when unused), `pages/TuttiAddRoutingPolicyPage/**`, `components/tutti/routing-policies/**`, `add-routing-policies/**`, stores `tuttiRoutingPoliciesStore`, `tuttiAddRoutingPolicyStore` |
| WP5 Protection | `pages/TuttiProtectionGroupsPage/**`, `pages/TuttiProtectionGroupDetailPage/**`, `components/tutti/protection/**`, stores `tuttiProtectionStore`, `tuttiFailoverStore` |
| WP6a Users | `pages/TuttiUsersPage/**`, `pages/TuttiUserCreatePage/**`, `pages/TuttiUserDetailPage/**`, `components/tutti/users/**`, `components/tutti/accounts/AccessPolicyDrawer.vue` (split it), `stores/tuttiUsersStore`, `tuttiAccessPolicyStore` |
| WP6b Access keys | `pages/TuttiProxyAccountsPage/**`, `components/tutti/accounts/**` except AccessPolicyDrawer, cross-links with Users |
| WP7a Console users | `pages/TuttiConsoleUsersPage/**`, `stores/tuttiConsoleUsersStore` |
| WP7b Audit log | `pages/TuttiAuditLogPage/**`, `stores/tuttiAuditLogStore` (drop the page language switch; map raw families and actions to labels) |
| WP8 Settings | `pages/TuttiSystemSettingsPage/**`, `components/tutti/settings/**`, `pages/TuttiProxiesPage/{ProxyConfigTab,ProxyScopeForm}.vue`, `stores/tuttiSystemSettingsStore`, `tuttiProxyFleetConfigStore` ("Proxies" tab: proxy defaults) |
| WP9 Docs and auth | `pages/DocsPage.vue`, `components/tutti/docs/**` (shrink the deploy guide to a short getting-started; keep the route), `pages/LoginPage/**`, `pages/TuttiChangePasswordPage/**`, `components/auth/**` |

Audit priorities per page are in the audit (part B); H items first: Dashboard, Storages, Replication list and wizard,
Users, Create user, Access keys, Audit log.

**Done means** for every WP: (1) the audit items for that page are applied using the kit patterns above; (2) unit tests
updated and passing (`corepack yarn test:unit --run <your dirs>`), including new tests for new logic; (3) i18n en/de/vi
in sync and unused keys pruned; (4) `lint:types`, eslint and prettier clean on changed files; (5) no new file above 300 lines;
(6) screenshots at 1440/1024/390 in light and dark re-shot and attached to the report; (7) only own paths committed, one commit per
logical step, no pushes.
