# Tutti Integration & End-to-End Testing Guide

This guide explains Tutti's integration testing architecture, end-to-end (E2E) test suites, and how containerized testing with `testcontainers-go` is organized.

---

## 1. Integration Testing Layers

Tutti uses a multi-layered testing strategy to simulate real-world storage clusters:

```
┌────────────────────────────────────────────────────────┐
│             End-to-End Workflow Tests                  │
│  (Client SDK -> S3 Proxy -> Redis -> Worker -> S3)     │
├────────────────────────────────────────────────────────┤
│             Database & Migration Tests                 │
│      (PostgreSQL 16 via testcontainers-go)             │
├────────────────────────────────────────────────────────┤
│           Distributed Queuing & Locks                  │
│       (Real Asynq & Redis test instances)              │
├────────────────────────────────────────────────────────┤
│              Unit & Concurrency Tests                  │
│               (go test -race ./...)                    │
└────────────────────────────────────────────────────────┘
```

---

## 2. Running End-to-End Integration Tests

### 2.1 Full In-Process E2E Suite (`test/`)
The `test/` package runs Tutti's embedded integration suite, launching actual Proxy and Worker instances with real S3 MinIO endpoints and Redis brokers:

```bash
# Run synchronous and asynchronous replication E2E tests
go test -v -race ./test/ -run TestApi_SyncReplication_PutAndDelete

# Run full object multipart and streaming copy tests
go test -v -race ./test/ -run TestObject_
```

**What this verifies:**
1. S3 Client sends a signed SigV4 request (`PUT /bucket/data.csv`) to Tutti Proxy.
2. Proxy validates auth, evaluates virtual-host routing, and writes to primary MinIO.
3. Synchronous replication path streams data to destination replica, updating version vectors.
4. Asynchronous queue verifies replication tasks, verifies idempotency, and checks version timestamps.
5. Deletion propagates with configured delay (`delete_delay_seconds`), confirming destination Delete Marker creation.

---

### 2.2 Containerized Control Plane Tests (`testcontainers-go`)
Tutti uses `github.com/testcontainers/testcontainers-go` to spin up ephemeral PostgreSQL 16 containers:

```bash
# Runs full migrations 0001-0012, lease fencing, and Outbox projection
go test -v -race ./pkg/controlplane/pg/...
go test -v -race ./pkg/controlplane/migrate/...
```

**Key test scenarios covered:**
- `AcquireLifecycleLease`: Verifies that only one worker can lease a rule at a time via `FOR UPDATE SKIP LOCKED`.
- `RenewLifecycleLease`: Verifies heartbeat extensions while a job is running.
- `Outbox Publisher`: Verifies projection from PostgreSQL into Redis hashes without event loss or drift.

---

## 3. Storage Provider Matrix (`test/env/`)

Tutti's test environment (`test/env/components.go`) supports containerized targets:
- **MinIO**: High-speed local S3 testing with versioning and object tagging.
- **Ceph RGW**: Multi-cluster S3 emulation with bucket notification testing.
- **SeaweedFS & Garage S3**: Lightweight alternative S3 engines for POSIX compatibility testing.
- **OpenStack Swift**: Keystone authentication and container sync testing.

To run tests against the multi-storage matrix:
```bash
# Requires Docker running locally
TEST_STORAGE_CONTAINERS=1 go test -v ./test/...
```
