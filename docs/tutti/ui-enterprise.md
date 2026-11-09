# Enterprise UI: J1–J3

J1 introduced the grouped sidebar, mobile drawer, breadcrumbs, shared page header, 24-hour date formatter and compact footer/About dialog. J2 was committed as `7c5b36c`; J3 completes the dashboard, login and monitoring work.

## Completed behavior

| Area | Behavior |
|---|---|
| Page layout | Shared PageHeader with title, description and page actions; content cards do not duplicate the page title. |
| Actions | Primary create actions sit in the page header; table actions have accessible names and confirmations for destructive changes. Viewer controls remain inert while read-only report navigation stays available. |
| Empty states | Lists distinguish empty data, filtered results, loading and API failure, with a relevant next action or retry. |
| Date/time | Shared locale-aware 24-hour formatting, including table timestamps and proxy keys. |
| Proxy accounts | One summary row per user; expanding it shows a dedicated table for static/dynamic keys, lifecycle timestamps and actions. Independently sorted source/key sets are not interpreted as parallel arrays. |
| Dashboard | Six KPIs; actionable alerts for unavailable storage, unacknowledged failover, paused/lagging replication and stale proxies; storage and replication summaries; refresh every 30 seconds while visible. |
| Dashboard reliability | Stale proxy health is unknown, stale traffic rates expire, partial API failure is visible, and an optional event response cannot mask failure of all core sources. Late responses after leaving/resetting the page are ignored. |
| Login | Explicit username/password placeholders, version, field labels, validation/error feedback and protection against repeated submission. |
| Monitoring | KPI groups for traffic, latency and errors; latest-update text and exact timestamp tooltip; pause/resume; unavailable updates do not show a green live indicator. Pending requests are canceled and obsolete responses ignored. |
| Responsive detail | Diff report tables scroll inside their own region; progress and descriptions stack on small screens without widening the page. |
| Loading cost | Routes load lazily. Main JS entry is about 1,044 kB / 299 kB gzip, down from about 1,653 kB / 474 kB gzip before splitting. Shared entry remains above Vite's 500 kB warning threshold. |

## Verification on 2026-11-09

- J2 staged snapshot tested independently of J3: 96 unit tests, type-check, ESLint and build passed.
- Completed UI: 120 unit tests passed. Full `npm run lint` passed, including type-check, ESLint, Oxlint, Stylelint and Prettier; production build passed.
- Chromium checked 15 content/form/detail pages at 1440×1000 and 390×844 in both light and dark themes. Sidebar collapse, mobile drawer navigation, expanded key tables, viewer controls and login errors were exercised.
- Empty states were checked on eight pages in mobile/light mode; API-error states on nine pages in mobile/dark mode.
- Browser checks found a mobile diff-report overflow and Vue warnings from fragment-root triggers/forms. After correction, affected pages were rechecked in all four modes with zero document overflow and no page errors or Vue warnings.
- Browser API responses were deterministic local fixtures. The backend API was not running; these checks do not replace an integration test against live storage.

The screenshots and browser findings for this session are under `/tmp/tutti-enterprise-checks/` (temporary local artifacts).

```bash
cd ui
npm run lint
npm run test:unit -- --run
npm run build
```

The earlier backend security/replication fixes remain separate from the J2/J3 UI commits.
