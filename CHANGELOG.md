# Changelog

## Tutti 0.1.0 (unreleased)

First Tutti release: a fork of [Chorus](https://github.com/clyso/chorus) (base commit `8d2237b`) with resilience features ported from [ReplicaT4](https://github.com/barreeeiroo/ReplicaT4). See [docs/tutti](./docs/tutti/configuration.md) and [upgrading from Chorus](./docs/tutti/upgrading-from-chorus.md).

### Added

- Auth: first-run admin password. The generated password is printed once as a banner on stderr instead of a JSON log field; a provided `TUTTI_BOOTSTRAP_ADMIN_PASSWORD` / `bootstrapAdminPassword` must have at least 12 characters (startup fails otherwise); new `chorus admin reset-password [--user] [--password-env]` recovery command that revokes the user's sessions; `docker-compose/.env.example`, Helm `auth.bootstrapAdmin.{existingSecret,forceChange}` and a login page hint. See [First login and password recovery](./docs/tutti/operations.md#first-login-and-password-recovery).
- Worker: `bulkConcurrencyPercent`, `multipartThreads` and `multipartPartSizeMib` in System settings (system setting > YAML/env > default; multipart knobs and the buffer budget apply to the next copy without restart). Worker throughput: per-process counters published to Redis every 5 s, `GET /tutti/worker-stats` and Prometheus metrics `tutti_copy_*`.
- Auth: sliding console sessions. A session ends after `api.auth.sessionIdleTimeout` (default 30m) without refresh and never lives past `api.auth.sessionMaxLifetime` (default 24h, replaces `sessionTTL`, which is still read as the max lifetime when the new key is unset). New `POST /auth/refresh` is the only call that extends a session (6 per minute per token, `429` with `Retry-After`; `401` at the cap); ordinary API calls and polling never extend it. Login and `GET /auth/me` return `expiresAt`, `maxExpiresAt` and `serverTime`.
- Storage-level replication: `user` is optional on replications (API, `tuttictl repl`, UI); an empty user copies storage A to B with each storage's connection account. The internal `$service` identity was removed. A shared overlap rule applies to replication add, group create and group update: same destination or reverse direction with intersecting buckets is rejected with HTTP 409 (reason in the error details; `tuttictl` prints it and exits non-zero); replications of the same user (storage-level, or the same real user) with a different source storage on a shared bucket (or both all-buckets) are rejected too, with a distinct reason, because the policy stores keep one source per bucket; same source with different destinations, and chains or unrelated pairs on disjoint buckets are allowed; "all" intersects everything, objects of different real users conflict only on intersecting explicit bucket lists, and storage-level objects cover every bucket of the storage. `list-buckets` also returns `coveredBuckets`. One source to many destinations is several replications or one group. Data behavior: same name with different content is overwritten by the source, destination-only objects are never touched, identical objects are skipped.
- Proxy: decode and verify `aws-chunked` (`STREAMING-*`) payloads before forwarding; `mc`/`minio-go` signed streaming uploads now work. Checks chunk and trailer signatures, `x-amz-checksum-*` and decoded length (`f3b063a`).
- Proxy: optional synchronous replication on the write path (`replication.sync.{enabled,writeQuorum,timeout,onQuorumFailure}`), `X-Tutti-Replicas` response header (`178745f`).
- Proxy: per-storage circuit breakers fed by periodic health probes and real traffic; fail-fast `503` for unavailable storages; `health.{failureThreshold,openDuration}`, `storage.<name>.healthCheckInterval`; metrics `storage_available`, `storage_breaker_transitions_total` (`5949da2`).
- Proxy: optional automatic failover with operator-defined policies and ordered targets (`failover.*`), Redis lock so one proxy acts, failover events, metric `failover_total` (`a5e56fb`).
- API: `Resilience` gRPC/REST service (`/tutti/proxies`, `/tutti/failover/policies`, `/tutti/failover/events`), `tuttictl proxies` and `tuttictl failover policies|events`, proxy status published to Redis (`95084c2`).
- UI: Tutti branding, Storage health (`/health`) and Failover (`/failover`) pages (`5be1f60`).
- Tests and CI: resilience e2e suite against MinIO (`test/resilience`), `go test -race` unit job, UI job (`1ca8525`).

### Changed

- `Content-Type` no longer has to be signed in SigV4 requests; `auth.requireSignedContentType: true` restores the strict check. `host` and `x-amz-*` must still be signed (`96d3046`).
- Replication event recording is retried (`replication.eventRetryBudget`, default 2 s); if it still fails the proxy answers `503` instead of a silent success (`replication.onEventError: fail`, default; `log` restores old behavior) (`96d3046`).
- Requests only carry replications whose source is the storage the request was routed to, so a failed-over scope does not record events against the failed storage (`90917fc`).
- Rename: Go module `github.com/hieunyit/tutti`; protobuf package and service names unchanged (`ab97996`).

### Fixed

- Data races in standalone startup and shared globals (`features`, logger) (`f76ce1f`).
- Data race in `pkg/objstore` credentials sync (`1ca8525`).

### Known limitations

- No automatic failback; see [manual failback](./docs/tutti/operations.md#manual-failback).
- Failover policies cannot be set with `tuttictl`; use REST/gRPC or the UI.
- Swift storages are not health-probed.

### License

Backend Apache-2.0; `ui/` AGPL-3.0; ReplicaT4-derived parts MIT. See [NOTICE](./NOTICE).
