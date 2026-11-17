# Tutti

<div align="center">

![banner](./docs/media/banner.png)

**Enterprise-Grade S3 & OpenStack Swift Storage Replication, Routing, and Lifecycle Engine**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Vue 3](https://img.shields.io/badge/Vue.js-3.5-4FC08D?style=flat&logo=vuedotjs)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-Asynq-DC382D?style=flat&logo=redis)](https://redis.io/)
[![Security Review](https://img.shields.io/badge/Security_Review-Documented-blue?style=flat&logo=shield)](./docs/audit/2026-11-17-comprehensive-security-and-bugfix-report.md)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)

[Quick Start](#-quick-start) • [Core Features](#-core-capabilities) • [Architecture](#-architecture) • [Security & Hardening](#-security--concurrency-hardening) • [API & Documentation](#-documentation)

</div>

---

## 📖 Overview

**Tutti** is a vendor-agnostic, distributed storage replication and proxy engine designed for massive-scale object data management across Amazon S3, MinIO, Ceph, and OpenStack Swift.

Originating as an enhanced fork of [Chorus](https://github.com/clyso/chorus) (Clyso GmbH) with resilience extensions from [ReplicaT4](https://github.com/barreeeiroo/ReplicaT4), Tutti has evolved into a robust cloud-native platform featuring **S3-compliant Lifecycle management**, **intelligent deletion sync policies**, **high-performance pattern filtering**, and **hardened multi-tenant access controls**.

Tutti enables live migrations with zero-downtime, continuous active-passive and active-active replication, real-time S3 proxy routing with circuit-breaking, and granular object retention.

---

## 🚀 Core Capabilities

### 🥇 1. Deletion Propagation Policy & Delete Marker Sync
Tutti gives operators complete control over how deletions on the source storage propagate to downstream replicas:
- **`sync`**: Exact hard deletion on the destination storage upon source removal.
- **`marker-only`**: Creates an S3 Delete Marker on versioned destination buckets, preserving object history while reflecting deletion.
- **`ignore`**: Retains destination objects indefinitely when deleted at the source (backup/archive mode).
- **Prefix Scoping & Delay**: Configure `includePrefixes`, `excludePrefixes`, and `delayBeforeDelete` (soft-delete grace period up to 90 days).
- **Automatic Audit Logging**: Every delete operation, skip, or failure is recorded in PostgreSQL with destination latency and retry state.
- **Safety Guard**: Prior to destination deletion, Tutti probes source storage metadata via `StatObject` to prevent accidental deletion if the object still exists.

### 🥈 2. Advanced Pattern Filtering Engine
Synchronize only the objects that matter with glob wildcard filtering:
- Supports wildcard globs (`*.jpg`, `logs/*`, `backup-*-2026-*.sql`, `data/**`).
- **Linear $O(N)$ Evaluation**: Evaluated without regex backtracking, completely immune to Regular Expression Denial of Service (ReDoS).
- Seamlessly applied across live replication, versioned migrations, and periodic source rescan jobs.

### 🥉 3. Full-Featured S3 Lifecycle Rules Engine
Automate storage cost optimization and compliance retention:
- **AWS S3 Compliant Scheduling**: Calculates object expiration aligned to midnight 00:00:00 UTC of the day following `(LastModified + expirationDays)`.
- **Flexible Cron Schedules**: Supports standard 5-field cron syntax (e.g. `0 2 * * *` for 02:00 AM UTC).
- **Dry-Run Simulation**: Estimate eligible object count and reclaimable bytes before applying policies.
- **Non-blocking Distributed Execution**: Utilizes PostgreSQL `FOR UPDATE SKIP LOCKED` and dynamic lease renewal heartbeats to coordinate multiple worker nodes without deadlock.
- **Safety Floor**: Hardened with zero-value checks (`lastModified.IsZero()`) to prevent premature deletion of objects with missing metadata.

Automatic expiration requires enabled bucket versioning and immutable, non-null version IDs.
Keys with older live versions are retained rather than exposing an older value. Unversioned
or suspended buckets fail safely without deleting data; Dry Run remains available.
See [lifecycle operations](./docs/tutti/lifecycle-operations.md) for upgrade and recovery steps.

### ⚡ 4. High-Performance S3 Proxy & Streaming
- **`aws-chunked` Streaming**: Decodes and verifies chunk-signature hashes on the fly without loading entire payloads into memory.
- **AWS Signature V4 & V2 Compatibility**: Re-signs forwarded requests cleanly using internal storage credentials.
- **Virtual-Host & Path-Style Routing**: Supports both `http://bucket.s3.domain.com` and `http://s3.domain.com/bucket`.
- **Per-Storage Circuit Breakers**: Fails fast (sub-millisecond) on dead storages instead of holding client threads.
- **Automatic Failover**: Reroutes users and buckets to healthy replicas when primary storage degrades.

---

## 🏛️ Architecture

```
                                  ┌──────────────────────────┐
                                  │      Client / S3 SDK     │
                                  │   (AWS CLI, Boto3, UI)   │
                                  └─────────────┬────────────┘
                                                │ S3 Requests / REST / gRPC
                                                ▼
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                   TUTTI S3 PROXY                                       │
│  - SigV4 / SigV2 Signature Verification & Re-signing                                   │
│  - Virtual-Host & Path Routing                                                         │
│  - Circuit Breakers & Dynamic Health Probing                                           │
│  - Synchronous Write Quorum & Event Capture                                            │
└───────────────────────────────┬──────────────────────────────┬─────────────────────────┘
                                │                              │
                     Direct I/O │                              │ Push Replication Tasks
                                ▼                              ▼
┌──────────────────────────────────────────────┐   ┌─────────────────────────────────────┐
│             STORAGE CLUSTERS                 │   │             REDIS QUEUE             │
│   Source (MinIO)  ──►  Destination (Ceph/S3) │   │   - Asynq Task Queue                │
│                                              │   │   - Distributed Locks (ObjectLockID)│
└───────────────────────▲──────────────────────┘   └──────────────────┬──────────────────┘
                        │                                             │
                        │ Replicate / Sync / Expire                   │ Pull Tasks
                        │                                             ▼
┌───────────────────────┴────────────────────────────────────────────────────────────────┐
│                                   TUTTI WORKER                                         │
│  - Asynq Task Workers & Multipart Streaming Copy Engine                                │
│  - S3 / Swift Object Syncer (Delete Propagation, Pattern Filter)                       │
│  - Periodic Rescan & Checkpointing Engine                                              │
│  - Lifecycle Scheduler (UTC Midnight Expiration, Dry-Run, Audit Logs)                  │
│  - Management API (gRPC / HTTP Gateway) with RBAC & Fail-Closed Token Interceptor      │
└───────────────────────────────────────────────┬────────────────────────────────────────┘
                                                │
                                                ▼ State & Metadata
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                              POSTGRESQL CONTROL PLANE                                  │
│  - Migrations 0001 - 0010 (Idempotent schema with performance indexes)                 │
│  - Policies, Delete Events, Lifecycle Rules, Audit Trail (FOR UPDATE SKIP LOCKED)      │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 🔒 Security & Concurrency Hardening

Tutti has undergone exhaustive adversarial penetration testing and concurrency audits. Key defenses include:

- **Fail-Closed Webhook Gateway**: Endpoints reject unauthenticated notifications on shared management ports unless an explicit secret token is provided.
- **BOLA / IDOR Protection**: Read-only `Viewer` roles are strictly scoped and blocked from querying global cluster delete logs.
- **Unified Distributed Locking**: All S3 and Swift syncer handlers lock uniformly on `(DestinationStorage, toBucket, objectKey)`, preventing split-brain concurrent writes.
- **Heartbeat Lock Renewal**: Ongoing multi-gigabyte transfers continuously refresh Redis lock TTLs every 2 seconds via `lock.DoCtx`, eliminating lock starvation.
- **Anti-SSRF & IMDS Shielding**: Endpoint validation in `ManagedStorage.Normalize()` automatically rejects link-local addresses and cloud metadata services (`169.254.169.254`, `fd00:ec2::254`).
- **Strict Bucket Sanitization**: Request parser verifies bucket names against strict RFC naming standards (`CheckValidBucketNameStrict`), defeating WAF-evasion and header confusion attacks.
- **DoS / Memory Exhaustion Guard**: Webhook batch requests are capped at 1,000 records per call, preventing out-of-memory crashes.

### Cross-origin management UI

When the UI and management API use different origins, configure the worker with
an exact origin allowlist, including scheme and port when needed:

```yaml
api:
  corsAllowedOrigins:
    - https://console.example.com
```

With an empty list, only localhost and private IP browser origins are permitted
for development. DNS names beginning with a private IP prefix are not trusted.

---

## ⚡ Quick Start

### 1. Using Docker Compose (Recommended for Testing)

The fastest way to test Tutti with MinIO and Redis locally:

```bash
# Clone the repository
git clone https://github.com/hieunyit/tutti.git && cd tutti

# Configure environment
cd docker-compose
cp .env.example .env

# Build and start services
docker compose up --build -d

# Check initial admin credentials
docker compose logs worker | grep -A6 'INITIAL ADMIN PASSWORD'
```

Access the Web Console at **http://localhost:8080** (User: `admin`).

### 2. Running From Source

Requirements: **Go 1.24+**, **Node.js 20+**, **PostgreSQL 15+**, **Redis 7+**.

```bash
# 1. Build backend binaries
go build -o build/tutti ./cmd/tutti
go build -o build/worker ./cmd/worker
go build -o build/proxy ./cmd/proxy

# 2. Run database migrations and worker
./build/worker -config ./service/worker/config.yaml

# 3. Build and run Web UI
cd ui
npm install
npm run build
```

---

## 💻 Management Tools

Tutti provides three complementary management interfaces:

### Web Console (`ui/`)
Modern, responsive single-page application built with Vue 3, Naive UI, and Pinia:
- **Replications View**: Visual status, throughput graphs, in-flight transfer monitoring, and zero-downtime switch triggers.
- **Replication Policies Card**: Inline configuration of Delete Propagation modes and include/exclude prefix and wildcard filters.
- **Lifecycle Manager**: Create and inspect rule schedules, view audit trails in a side drawer, and simulate expiration with the **Dry Run Modal**.
- **Role-Based Views**: WriteGate guards all destructive and operational actions behind `RoleAdmin`.

### CLI Tool (`tuttictl`)
Full-featured command-line utility for automation and scripting:
```bash
# Check replication status
tuttictl replication list

# Set delete propagation policy
tuttictl replication set-delete-policy --id "main:backup:bucket1:bucket2" --mode sync --delay 24h

# Trigger dry-run lifecycle evaluation
tuttictl lifecycle dry-run --storage main --bucket uploads --days 30
```

### gRPC & REST Gateway
All control-plane functions are exposed via gRPC (`proto/tutti/`) and mapped to RESTful JSON endpoints via `grpc-gateway`:
- OpenAPI 3.0 / Swagger schema available at `proto/gen/openapi/tutti/tutti.swagger.json`.

---

## 🧪 Testing & Verification

Tutti maintains strict code quality and verification standards:

```bash
# Static analysis and linting
go vet ./...
npm run lint:types --prefix ui
npm run lint:eslint --prefix ui

# Concurrency race detector & unit tests
go test -race ./pkg/...
go test -race ./service/...

# Frontend unit testing
npm run test:unit --prefix ui

# Production frontend bundle
npm run build --prefix ui
```

---

## 📚 Documentation

Detailed architecture and operations guides are located in [`docs/`](./docs/):
- [Deployment Guide (Docker, K8s, VM)](./docs/tutti/deployment-guide.md)
- [Management REST & gRPC API Reference](./docs/tutti/api.md)
- [Operations & Troubleshooting Guide](./docs/tutti/operations.md)
- [PostgreSQL Control Plane & Migrations](./docs/tutti/control-plane-db.md)
- [Protection Groups Architecture](./docs/tutti/protection-groups.md)
- [Security & Concurrency Audit Report](./docs/audit/2026-11-17-comprehensive-security-and-bugfix-report.md)

---

## 📜 License & Attribution

- **Backend (`pkg/`, `service/`, `cmd/`)**: [Apache License 2.0](./LICENSE). Originally based on Chorus (© Clyso GmbH).
- **Web UI (`ui/`)**: GNU AGPL-3.0 (© Clyso GmbH, with Tutti modifications).
- **Resilience Extensions**: Derived from ReplicaT4 (MIT License, © Diego Barreiro Perez).

See [NOTICE](./NOTICE) for complete copyright and license disclosures.
