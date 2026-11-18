# Tutti Architecture & Sequence Flows

This document details the system architecture of **Tutti** using the C4 model and provides sequence diagrams for key operational scenarios.

---

## 1. C4 Architecture Models

### 1.1 Level 1: System Context Diagram

```mermaid
graph TD
    User["Storage Administrator / Operator"]
    AppClient["Application Clients / S3 SDKs<br/>(AWS CLI, Boto3, rclone, MinIO SDK)"]

    subgraph TuttiPlatform ["Tutti Distributed Storage Platform"]
        TuttiNode["Tutti Engine<br/>(Proxy, Worker, Control Plane)"]
    end

    SourceStorage["Primary Storage Cluster<br/>(MinIO / Ceph RGW / AWS S3)"]
    FollowerStorage["Replication Target / Disaster Recovery<br/>(Ceph RGW / AWS S3 / OpenStack Swift)"]
    AlertingSystem["Monitoring & Observability<br/>(Prometheus / Grafana / OpenTelemetry)"]

    AppClient -->|"S3 REST API (GET, PUT, DELETE, Multipart)"| TuttiNode
    User -->|"Web Console (HTTPS:8080) / tuttictl CLI"| TuttiNode
    TuttiNode -->|"Forward Writes / Health Checks"| SourceStorage
    TuttiNode -->|"Asynchronous / Synchronous Replication"| FollowerStorage
    TuttiNode -->|"Export Metrics & Traces"| AlertingSystem
```

---

### 1.2 Level 2: Container Diagram

```mermaid
graph TB
    subgraph Clients ["Clients & UI"]
        Browser["Web Console (Vue 3 / Naive UI)"]
        CLI["tuttictl CLI"]
        S3App["S3 Clients / SDKs"]
    end

    subgraph Ingress ["API & Proxy Layer"]
        Proxy["Tutti S3 Proxy (:80)\n- SigV4 / SigV2 Re-signing\n- Virtual-Host & Path Routing\n- Circuit Breakers\n- Synchronous Quorum Replicator"]
        MgmtAPI["Management Gateway (:8080)\n- gRPC / HTTP REST Gateway\n- RBAC (Admin / Viewer)\n- Fail-Closed Webhook Ingress"]
    end

    subgraph State ["Coordination & State Layer"]
        Postgres[("PostgreSQL 15+ Control Plane\n- Policies & Routing Tables\n- Outbox Event Table\n- Delete Events & Lifecycle Rules\n- Distributed Lease Fencing")]
        RedisDB[("Redis 7+ (In-Memory)\n- Asynq Task Queue\n- Object Lock Manager\n- Version Vectors & Delete Deadlines\n- Ephemeral Projection Cache")]
    end

    subgraph Workers ["Worker Execution Layer"]
        Worker["Tutti Worker Node\n- Asynq Task Consumer\n- S3/Swift Streaming Copy Engine\n- Pattern Filter Engine (Glob O(N))\n- Lifecycle Cron Scheduler\n- Periodic Rescan & Checkpoint"]
    end

    subgraph Storages ["Storage Backends"]
        S3Source[("Source S3 Storage")]
        S3Follower[("Destination S3 Storage")]
    end

    S3App --> Proxy
    Browser --> MgmtAPI
    CLI --> MgmtAPI
    Proxy --> S3Source
    Proxy --> RedisDB
    Proxy --> Postgres
    MgmtAPI --> Postgres
    MgmtAPI --> RedisDB
    Worker --> RedisDB
    Worker --> Postgres
    Worker --> S3Source
    Worker --> S3Follower
```

---

## 2. Sequence Diagrams for Core Scenarios

### 2.1 Write & Replication Flow (Synchronous & Asynchronous)

```mermaid
sequenceDiagram
    autonumber
    actor Client as S3 Client (Boto3 / AWS CLI)
    participant Proxy as Tutti S3 Proxy
    participant Primary as Primary Storage (MinIO)
    participant Redis as Redis (Asynq & Locks)
    participant Worker as Tutti Replication Worker
    participant Replica as Replica Storage (Ceph/S3)

    Client->>Proxy: PUT /bucket/data.csv (SigV4)
    Note over Proxy: Verify Signature & Rate Limits<br/>Evaluate Bucket Routing & Circuit Breaker
    Proxy->>Primary: PUT /bucket/data.csv
    Primary-->>Proxy: 200 OK (ETag, VersionId)

    alt Synchronous Replication Enabled (writeQuorum >= 2)
        Proxy->>Redis: Acquire Object Lock (Destination, Bucket, Key)
        Proxy->>Replica: Stream Copy Object to Replica
        Replica-->>Proxy: 200 OK
        Proxy->>Redis: Update Destination Version Vector & Release Lock
    end

    Proxy->>Redis: Enqueue ObjectSyncPayload (Asynq Queue)
    Proxy-->>Client: 200 OK (X-Tutti-Replicas: 2/2)

    Note over Worker,Redis: Asynchronous Safety Net & Replication
    Worker->>Redis: Dequeue ObjectSyncPayload
    Worker->>Redis: Acquire ObjectLockID (toStorage, toBucket, key)
    Worker->>Redis: GetObjVersion(key)
    alt Version Already Current (fromVer <= toVer)
        Worker-->>Redis: Ack Task (Skip duplicate copy)
    else Version Behind
        Worker->>Primary: Stream Read Object
        Worker->>Replica: Multipart Streaming Copy
        Replica-->>Worker: 200 OK
        Worker->>Redis: UpdateIfGreater(toVer)
        Worker-->>Redis: Ack Task
    end
    Worker->>Redis: Release ObjectLockID
```

---

### 2.2 Delete Propagation & Delete Marker Sync Flow

```mermaid
sequenceDiagram
    autonumber
    actor Client as S3 Client
    participant Proxy as Tutti S3 Proxy
    participant Primary as Source Storage
    participant PG as PostgreSQL (Control Plane)
    participant Redis as Redis (Deadlines & Queues)
    participant Worker as Tutti Worker Syncer
    participant Target as Destination Storage

    Client->>Proxy: DELETE /bucket/report.pdf
    Proxy->>Primary: DELETE /bucket/report.pdf
    Primary-->>Proxy: 204 No Content

    Proxy->>PG: Record Delete Event (status: pending, generation: G)
    Proxy->>Redis: DeleteObjWithDeadline(key, now + deleteDelay)
    Proxy->>Redis: Enqueue ObjectSyncPayload(Deleted: true, DeleteEventID)
    Proxy-->>Client: 204 No Content

    Note over Worker: Worker processes deletion task
    Worker->>Redis: Dequeue ObjectSyncPayload(Deleted: true)
    Worker->>PG: Resolve Policy (mode, includePrefixes, delaySeconds)
    Worker->>Primary: StatObject(bucket, report.pdf)
    alt Object Recreated on Primary
        Primary-->>Worker: 200 OK (Object exists)
        Worker->>PG: Log Audit Entry (action: skipped, reason: source recreated)
        Worker-->>Redis: Ack Task (abort deletion)
    else Object Confirmed Deleted
        Primary-->>Worker: 404 NoSuchKey
        Worker->>Redis: Check Delete Deadline
        alt time.Now() < deadline
            Worker-->>Redis: Reschedule / Defer Task until deadline
        else Deadline Expired
            alt mode == "marker-only"
                Worker->>Target: GetBucketVersioning(bucket)
                alt Destination Versioning != Enabled
                    Worker->>PG: Record Audit (action: failed, reason: unversioned)
                    Worker-->>Redis: Fail Task (prohibit hard delete)
                else Versioning Enabled
                    Worker->>Target: RemoveObject(bucket, key) [Create Marker]
                    Worker->>PG: Record Audit (action: marker_created)
                end
            else mode == "sync"
                Worker->>Target: RemoveObject(bucket, key, versionId)
                Worker->>PG: Record Audit (action: deleted)
            else mode == "ignore"
                Worker->>PG: Record Audit (action: skipped, reason: ignore mode)
            end
            Worker->>PG: Update Delete Event Status (status: completed)
            Worker-->>Redis: Ack Task
        end
    end
```

---

### 2.3 Automatic Storage Failover Flow

```mermaid
sequenceDiagram
    autonumber
    participant Client as S3 Clients
    participant Proxy as Tutti S3 Proxy
    participant CB as Per-Storage Circuit Breaker
    participant Primary as Primary Storage Cluster
    participant Follower as Replica Storage Cluster
    participant Ctrl as Failover Controller
    participant PG as PostgreSQL Control Plane

    loop Every 5 seconds (Health Probe)
        Proxy->>Primary: HEAD /health-check-bucket
        Primary-->>Proxy: 200 OK
    end

    Note over Primary: Primary Storage Network Degradation / Outage
    Client->>Proxy: GET /bucket/image.png
    Proxy->>Primary: GET /bucket/image.png
    Primary-->>Proxy: 504 Gateway Timeout / Connection Refused
    Proxy->>CB: Record Failure
    Note over CB: Consecutive errors exceed threshold (Trip Circuit Breaker)

    Ctrl->>CB: Query Storage Health State
    Ctrl->>PG: Verify Failover Policy (Enabled: true, MaxLag: 5)
    Note over Ctrl: Evaluate Promotion Eligibility
    Ctrl->>PG: Promote Follower to Primary Route
    Ctrl->>PG: Record FailoverEvent (From: Primary, To: Follower)
    Ctrl->>Proxy: Invalidate Routing Cache

    Client->>Proxy: GET /bucket/image.png
    Note over Proxy: Route resolves to Follower Storage
    Proxy->>Follower: GET /bucket/image.png
    Follower-->>Proxy: 200 OK (Payload)
    Proxy-->>Client: 200 OK
```

---

### 2.4 Lifecycle Management Engine Flow

```mermaid
sequenceDiagram
    autonumber
    participant Sched as Lifecycle Scheduler
    participant PG as PostgreSQL (Control Plane)
    participant Worker as Lifecycle Executor
    participant S3 as Storage Bucket
    participant Audit as Lifecycle Audit Table

    loop Every Check Interval (e.g. 1 minute)
        Sched->>PG: ListDueRules(now)
        PG-->>Sched: [Rule#42: TTL 30 days, Prefix: logs/]
    end

    Sched->>Worker: Dispatch Rule#42 Execution
    Worker->>PG: AcquireLifecycleLease(Rule#42, ownerToken, 15m)
    Note over PG: Atomic conditional update:<br/>lease_until < now OR lease_token IS NULL
    PG-->>Worker: Lease Granted (ownerToken)

    rect rgb(240, 248, 255)
        Note over Worker: Background Heartbeat Goroutine (Every 2m)
        Worker->>PG: RenewLifecycleLease(Rule#42, ownerToken, +15m)
    end

    Worker->>S3: GetBucketVersioning(bucket)
    alt Bucket Not Versioned
        Worker->>PG: Mark Rule Execution Failed (Versioning required)
    else Versioning Enabled
        Worker->>S3: ListObjects(Prefix: "logs/", WithVersions: true)
        loop For Each Object Version
            Note over Worker: Evaluate ShouldExpire(lastModified, 30, nowUTC)<br/>Boundary: Midnight 00:00:00 UTC
            alt Eligible for Expiration
                Worker->>Worker: leaseGuard.EnsureValid()
                Worker->>S3: RemoveObject(bucket, key, versionId)
                S3-->>Worker: 204 Deleted
                Worker->>Audit: RecordAudit(RuleID, Key, Size, DeletedAt)
            end
        end
        Worker->>PG: CompleteLifecycleLease(Rule#42, ownerToken, nextRunAt, stats)
        Note over PG: Clear lease_token & lease_until,<br/>update last_run_at and next_run_at
    end
```
