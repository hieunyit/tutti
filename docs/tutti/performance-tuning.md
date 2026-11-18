# Tutti Performance Tuning & Scalability Guide

This guide provides practical recommendations for tuning Tutti to achieve maximum throughput, sub-millisecond proxy routing, and horizontal scalability across millions of objects.

---

## 1. Operating System & Kernel Tuning

For servers hosting Tutti S3 Proxy or Replication Workers under high I/O:

```ini
# /etc/sysctl.d/99-tutti-tuning.conf

# File descriptors and socket queues
fs.file-max = 2097152
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535

# Ephemeral port range
net.ipv4.ip_local_port_range = 1024 65535

# Socket buffer sizes (for 10GbE / 25GbE networks)
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# Connection reuse
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
```

Increase system limits in `/etc/security/limits.conf`:
```
tutti    soft    nofile    1048576
tutti    hard    nofile    1048576
```

---

## 2. Storage HTTP Transport Tuning

Tutti maintains a shared, dedicated `http.Transport` per storage endpoint to eliminate TLS handshake overhead and connection churning:

```yaml
# service/proxy/config.yaml or service/worker/config.yaml
storage:
  transport:
    maxIdleConns: 2048             # Total idle connections across all storages
    maxIdleConnsPerHost: 512       # Maximum idle connections per storage backend
    idleConnTimeout: 90s           # Keep connections alive across bursts
    disableCompression: true       # Never compress S3 payloads (CPU waste)
    dialTimeout: 3s
    responseHeaderTimeout: 30s
```

> [!TIP]
> Setting `maxIdleConnsPerHost` to at least 256–512 ensures parallel replication workers do not repeatedly open new TCP sockets to MinIO/Ceph clusters.

---

## 3. PostgreSQL Control Plane Connection Pool

The control plane coordinates policies, outbox queues, delete events, and lifecycle leases.

```yaml
controlPlane:
  db:
    maxConns: 64                  # Maximum connections per node
    minConns: 8                   # Minimum warm connections
    maxConnLifetime: 1h           # Recycle connections periodically
    maxConnIdleTime: 15m
```

### Avoiding Database Bottlenecks under Horizontal Scaling
- **Idempotent Indexes**: Tutti Migration `0010_performance_indexes.sql` creates covering indexes on `delete_events(replication_id, status, id DESC)` and `lifecycle_rules(next_run_at, enabled)`.
- **Non-blocking Locks**: All scheduler lease operations execute `UPDATE ... WHERE lease_token=$2 AND lease_until > clock_timestamp()` or `SELECT ... FOR UPDATE SKIP LOCKED`. Workers never block each other waiting for table locks.
- **Outbox Batching**: The database outbox publisher drains up to 1,000 events per query (`controlplane.OutboxConfig{BatchSize: 1000}`).

---

## 4. Redis Queue & Asynq Worker Concurrency

```yaml
worker:
  concurrency: 64                 # Number of concurrent task execution goroutines
  queues:
    critical: 6                   # Priority ratio for live user requests
    default: 3                    # Standard replication tasks
    low: 1                        # Periodic rescan / background repair
  asynq:
    highWatermark: 500000         # Atomic Lua limit to prevent OOM
```

> [!IMPORTANT]
> If Redis memory is constrained, set `highWatermark` appropriately. When queue size reaches this watermark, Tutti applies backpressure (`ErrQueueFull`) rather than crashing Redis with out-of-memory.

---

## 5. Lifecycle Management Engine Tuning

For buckets with tens of millions of objects:
- **Chunked Pagination**: Tutti streams `ListObjects` iteratively. Memory consumption remains $O(1)$ regardless of bucket size.
- **Lease Fencing Margin**: The lease duration is 15 minutes (`lifecycleLeaseDuration`), and background workers renew every 2 minutes. The renewal margin is 5 minutes (`lifecycleRenewMargin`), ensuring high network latency does not trigger lease expiration.
- **Prefix Isolation**: Partition lifecycle rules across distinct object prefixes (`logs/`, `archive/`, `temp/`) so separate workers can process distinct rules in parallel without contention.

---

## 6. Benchmarking Guide

Run Tutti's internal benchmarks directly:

```bash
# 1. Pattern filter throughput benchmark (O(N) glob evaluation)
go test -bench=BenchmarkPatternFilter -benchmem ./pkg/filter/...

# 2. S3 Streaming Copy Engine throughput (simulated latency & part sizes)
go test -bench=BenchmarkCopy -benchmem ./service/worker/copy/...

# 3. HTTP Transport & Proxy Routing benchmark
go test -bench=BenchmarkTransportTLS -benchmem ./pkg/s3client/...
```
