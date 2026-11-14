# Throughput tuning: verification report

## 1. Gate
- Go: `go build ./...`, `go vet ./pkg/... ./service/...`, `go test ./pkg/... ./service/...` all clean (no flakes). `tools/tuttictl`: build and test ok.
- UI: `lint:types` ok, `test:unit --run` 91 files / 604 tests ok, `lint:eslint` ok, `build` ok.
- No integration breaks, no code changes were needed.

## 2. Contract (by reading code)
`proto/tutti/tutti.proto` and `ui/src/utils/types/tutti.ts`: `bulkConcurrencyPercent`, `multipartThreads`, `multipartPartSizeMib` match; `TuttiWorkerStat` / `TuttiWorkerStatsTotals` fields match the proto (int64 as `TuttiInt64 = string | number`, doubles as numbers); `TuttiService.getWorkerStats` uses `GET /tutti/worker-stats`; `PUT /tutti/system-settings` body is the nested `{"worker":{...}}`. No mismatch.

## 3. Real run (own compose project `perf`, ports 58080/59671/59090, memory limits redis 384m, postgres 384m, worker 1g, 2 x minio 128m, web-ui 64m; removed afterwards)
Seed: A `bulk` 300 x 1 MiB, `big` 4 x 128 MiB; storages `sta`, `stb` with connection accounts; no S3 user.

- (a) PASS. Storage-level A->B `bulk`. Worker stats are published every 5 s, so an unthrottled copy (300 objects, about 7 s) is barely visible. With `bandwidthLimitBytes` = 15 MiB/s (24 s copy) polled every 2 s: inFlight 1 -> 9 -> 10, concurrency 10 (live 8), objectsPerSec1m 5.1 -> 6.3 -> 2.6 -> 3.8, bytesPerSec1m 5.4 -> 6.6 -> 2.7 -> 4.0 MB/s, buffer total 268435456 (see issue 1 for the 0 after a partial PUT), buffer used 1-10 MiB; inFlight <= concurrency in all samples. Prometheus on :9090 (metrics.enabled): `tutti_copy_objects_total{result="success"}`, `tutti_copy_bytes_total`, `tutti_copy_duration_seconds`, `tutti_copy_inflight`, `tutti_copy_buffer_used_bytes`, `tutti_copy_buffer_total_bytes`, `tutti_copy_buffer_wait_seconds_total` all present.
- (b) PASS for storage and live effect; FAIL for expected gain. `PUT {"worker":{"multipartThreads":4}}` returns 200 and GET returns 4 without restart. Wall time of `big` (4 x 128 MiB, loopback, minio 128m): threads=1 10.6 s, 11.0 s, 13.6 s; threads=4 27.6 s, 42.7 s, 45.6 s. Threads=4 was 3-4x SLOWER here. Loopback with 128 MiB minio containers and 16 concurrent range streams is probably the cause (resource contention, no network latency to hide), but this was not isolated.
- (c) PASS. `bufferBudgetBytes` 32 MiB live, `big` copied: sample `inFlight 4, bufferTotal 33554432, bufferUsed 33554432 (100%), bufferWaiters 2`, later `bufferWaitSecondsTotal` 9.01; Prometheus `tutti_copy_buffer_wait_seconds_total` 9.01. The UI rule (used >= 90% and waiters > 0) held. Budget restored to 256 MiB afterwards.
- (d) PASS. 400 InvalidArg: `worker.multipartThreads must be 0 (default) or in 1..16: 99`; `worker.bulkConcurrencyPercent must be 0 (default) or in 5..95: 3`; `worker.multipartPartSizeMib must be 0 (default) or in 5..512: 2`.
- (e) PASS. Viewer: `GET /tutti/worker-stats` 200; `PUT /tutti/system-settings` 403 `role "viewer" is not allowed to call /tutti.Resilience/UpdateSystemSettings`.
- (f) PASS. Chromium 1248 via playwright-core. Metrics tab Throughput panel shows real tiles during a copy (e.g. objects/s 1.08 -> 4.80, 4.8 MB/s, in flight 10 / 10, buffer 4% 10.0 MB of 256 MB, hint "All workers are busy..."); the hint "Buffer budget is the bottleneck: raise it." appeared once during the 32 MiB run. System settings Worker tab: the three new fields with "Needs worker restart" / "Applies immediately" badges; "Recommended values" is collapsed by default, expands to four rows each with a Fill in button (4 found, clicked). Screenshots in `/tmp/claude-1000/-home-hellc-tutti/5a90509e-a3c8-4966-8436-f12e94d9ab2d/scratchpad/perf-shots/` (m1-*, c-*, settings-*).

## 4. Fixed
Nothing needed fixing.

## 5. Remaining issues
1. Partial `PUT /tutti/system-settings` with a worker body resets `bandwidthLimitBytes` and `bufferBudgetBytes` to 0 (unlimited) because `pkg/api/system_settings_handlers.go` applies them when `>= 0` and proto3 cannot tell "absent" from 0. A request of only `{"worker":{"multipartThreads":4}}` therefore removed the 256 MiB buffer budget (stats showed buffer total 0). The UI sends the full worker object so it is not affected, but API/tuttictl callers are. Needs optional fields or an update mask.
2. multipartThreads=4 was slower than 1 in this loopback setup; real gain only unverified (needs a WAN-like or latency-injected target).
3. Worker stats refresh every 5 s (TTL 20 s), so short copies (< 10 s) are hardly visible in the panel.
4. Recommended values helper is collapsed by default and has to be expanded before Fill in is visible.
