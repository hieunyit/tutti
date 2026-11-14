# Rename verification: chorus -> tutti (2026-11-14)

Commits (branch main): proto `0a46268`, Go/ops `df06912`, UI `e10e8fc`, docs/deploy/CI `e907a65`.

## Collision decisions

| Old name | Natural new name | Exists already | Decision |
|---|---|---|---|
| `proto/chorus/chorus.proto` | `tutti.proto` | yes (`proto/tutti/tutti.proto`) | `core.proto` (generated `core.pb.go`, `core_grpc.pb.go`, `core.pb.gw.go`) |
| proto service `Chorus` | `Tutti` | no (other services: Resilience, Policy, Diff, Webhook, ProtectionGroups) | `Tutti` (`pb.NewTuttiClient`, `RegisterTuttiServer`) |
| `ui/src/services/ChorusService.ts`, `ChorusService` | `TuttiService` | yes | `TuttiCoreService.ts` / `TuttiCoreService` |
| `ui/src/utils/types/chorus.ts` | `tutti.ts` | yes | `tuttiCore.ts` (imports `@/utils/types/tuttiCore`) |
| `ui/src/components/chorus/*` | `components/tutti/*` | dirs exist, no file path collides | merged into `components/tutti/*` (no `core/` subdir needed) |
| test-local `const chorus` vs `const tutti` in `tuttiAddReplication.spec.ts` | `tutti` | yes (same scope) | `tuttiCore` |
| `RouteName.CHORUS_*`, Redis `chorus:*`, metrics | `TUTTI_*`, `tutti:*` | no collision found | plain rename |
| router lazy-import consts `ChorusReplicationPage` etc. | `Tutti*Page` | only inside path strings | plain rename |

Method: all identifiers containing chorus (any case) were mapped to the tutti spelling and checked as whole words against the repository
before renaming; only the entries above collided. `go build`, `vue-tsc` and the test suites confirmed there were no hidden ones.

## Intentional compatibility fallback

- UI `chorus-locale` localStorage key: read once in `i18nStore.initLocale()`, copied to `tutti-locale`, then removed (covered by a unit test).

## Remaining `chorus` hits (all on the keep list)

| Reason | Where |
|---|---|
| Legal attribution (Clyso lines, NOTICE) | NOTICE, source headers (lines also carrying `Clyso`) |
| Upstream URLs (`github.com/clyso/chorus`, `clyso/chorus`, `docs.clyso.com`, `harbor.clyso.com/chorus/*`, `chorus-dev.clyso.cloud`) | README.md, CONTRIBUTING.md, .goreleaser.yml (homepage), ui/package.json (repository), tools/tuttictl/README.md, docs/swift-migration.md, proto/tutti/policy.proto, service/worker/handler/replication_switch.go, test/migration/switch_test.go, tools/bench/pkg/bench/start.go |
| Fork statements | README.md, ui/README.md, CHANGELOG.md (first release entry), docs/tutti/configuration.md (upstream behavior), docs/design/console/Login.dc.html and Docs.dc.html, i18n "based on Chorus by Clyso GmbH" strings |
| Historical documents | docs/tutti/upgrading-from-chorus.md (name and content unchanged), CHANGELOG.md past entries, links/slug to it (`DocsHelper.ts`, `i18nDocs.ts`, `DocsHelper.spec.ts`, docs/tutti/operations.md) |
| Compatibility fallback | `ui/src/stores/i18nStore.ts`, `ui/src/i18n/__tests__/locales.spec.ts` |
| This document | rename description |

`git ls-files | grep -i chorus` lists only `docs/tutti/upgrading-from-chorus.md`.

## Smoke test (compose project `ren`, removed afterwards)

Built worker/proxy/web-ui from this tree; limits 256m postgres, 256m redis, 512m worker, 256m proxy, 128m web-ui.

- worker starts: PASS
- login (bootstrap password, forced change via `/auth/password`): PASS
- `GET /tutti/storages` and `/tutti/system-settings`: PASS (200)
- gRPC `tutti.Tutti/GetAppVersion` + `GetStorages` on the worker gRPC port (generated client): PASS; `tuttictl storage` over REST: PASS
- web UI served, login page rendered in Chromium, title `Tutti | Login`, no page errors: PASS
- `/bin/tutti admin reset-password` in the worker container (banner prints the new command): PASS
- Prometheus endpoint: 18 `tutti_*` samples, 0 `chorus` matches: PASS
- worker and proxy logs contain no `chorus`: PASS
- cleanup: containers, volumes, network and images of project `ren` removed (verified with docker ps -a / volume ls / network ls / images)
