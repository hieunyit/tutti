# Tutti Dependency Management & Module Directives

This document details third-party dependencies, custom patches, and `replace` directives in Tutti's `go.mod`.

---

## 1. Module Directives (`replace`) Overview

Tutti relies on two local / forked dependencies to implement high-throughput distributed queuing and multi-backend S3 compatibility:

```go
replace (
	github.com/hibiken/asynq => ./third_party/asynq
	github.com/minio/minio-go/v7 => github.com/aiivashchenko/minio-go/v7 v7.0.0-20260910131941-154d9dfa4858
)
```

---

## 2. Detailed Patch Analysis

### 2.1 `github.com/hibiken/asynq => ./third_party/asynq`

#### Context & Purpose
[Asynq](https://github.com/hibiken/asynq) is a distributed task queue built on Redis. In large-scale storage replication (millions of objects), queuing tasks without strict admission control leads to unbounded Redis memory growth and out-of-memory (OOM) crashes.

#### Patches Applied (`third_party/asynq/TUTTI-PATCHES.md`)
1. **Atomic High-Watermark Admission Control**:
   - Upstream Asynq does not enforce queue capacity checks atomically during task creation.
   - Tutti patched the Lua enqueue scripts (`asynq:{queue}:high_watermark`) to enforce quota checks in the *exact same Redis transaction* as task creation.
   - When the queue exceeds `high_watermark`, it rejects tasks with `ErrQueueFull` without allocating Redis memory.
2. **Retry-Independent Deferral (`Config.IsFailure`)**:
   - The Asynq worker processor was patched to re-schedule tasks whose errors are classified as non-failures by `Config.IsFailure` even when remaining retries are `0`.
   - **Why this matters**: Delete propagation uses this to defer deletion until `delete_delay_seconds` has elapsed without exhausting Asynq's retry counter.
3. **Internal Redis Client Exposure**:
   - `Inspector.RedisClient` exposes the internal queue connection cleanly without requiring unsafe reflection to inspect queue state during health checks.
4. **Controlled Redrive Quota**:
   - `RunAllArchivedTasks` moves only up to the remaining capacity of the destination queue rather than flooding the active queue.

#### Upstream Strategy & Maintenance
- **Upstream PR**: Propose atomic watermark admission control and `IsFailure` non-decrementing retry policies to `hibiken/asynq`.
- **Maintenance**: The snapshot under `third_party/asynq` contains only production Go sources and retains the MIT License. Tested via `pkg/tasks` test suite against real Redis instances.

---

### 2.2 `github.com/minio/minio-go/v7 => github.com/aiivashchenko/minio-go/v7`

#### Context & Purpose
Tutti supports diverse S3-compatible backends: AWS S3, MinIO, Ceph RadosGW (RGW), and OpenStack Swift S3 API. Different storage providers deviate in subtle ways regarding XML error responses, versioning headers, and multipart part numbering.

#### Patches Applied
1. **Ceph RadosGW Error & Status Normalization**:
   - Standard `minio-go` expects strict AWS S3 XML error formats. Ceph RGW sometimes returns variations for bucket status, versioning state, and multipart completions.
   - The fork normalizes error code parsing and adds support for Ceph-specific response structures in standalone integration tests.
2. **Streamed Chunk & Hash Verification**:
   - Enhances buffer reuse and connection keep-alives under high-throughput parallel object streaming.

#### Upstream Strategy & Maintenance
- **Upstream Strategy**: Evaluate whether recent `minio-go/v7` (v7.0.70+) releases natively support all required Ceph RGW error mappings.
- **Risk Assessment**: Low risk for standard S3 APIs; regression coverage is validated via `pkg/s3client` and `test/sync_replication_test.go`.

---

## 3. Core Enterprise Dependencies

| Package | Version | Purpose |
| :--- | :--- | :--- |
| `github.com/jackc/pgx/v5` | v5.7.4 | High-performance PostgreSQL driver with native binary protocol and `FOR UPDATE SKIP LOCKED` lease support. |
| `github.com/redis/go-redis/v9` | v9.14.1 | Thread-safe Redis client for version tracking, locks, and task coordination. |
| `github.com/bsm/redislock` | v0.10.0 | Distributed locking engine used for per-object mutual exclusion. |
| `github.com/testcontainers/testcontainers-go` | v0.44.0 | Containerized integration testing with real PostgreSQL 16 and Redis containers. |
| `github.com/rs/zerolog` | v1.34.0 | Zero-allocation structured JSON logging. |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metric exposition for proxy, worker, and storage latencies. |
| `naive-ui` & `vue` | 3.5+ | Modern Vue 3 single-page application framework for the Tutti Web Console. |

---

## 4. Upgrade & Auditing Guidelines

When upgrading Go dependencies in `go.mod`:
1. Always run static analysis: `go vet ./...`
2. Run data race detection: `go test -race ./pkg/... ./service/...`
3. Run containerized integration tests: `go test -race ./pkg/controlplane/migrate/... ./pkg/controlplane/pg/...`
4. Verify that local patches in `third_party/asynq` compile without deprecation warnings.
