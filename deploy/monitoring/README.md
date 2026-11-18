# Tutti Monitoring & Grafana Dashboard

This directory provides pre-configured dashboards and Prometheus scrape configurations for monitoring Tutti in production.

---

## 1. Quick Setup: Prometheus Scrape Config

Ensure Prometheus scrapes the Tutti Proxy, Worker, and Management API metrics endpoints:

```yaml
scrape_configs:
  - job_name: 'tutti-proxy'
    scrape_interval: 10s
    static_configs:
      - targets: ['tutti-proxy:8080'] # or configured metrics port
    metrics_path: '/metrics'

  - job_name: 'tutti-worker'
    scrape_interval: 10s
    static_configs:
      - targets: ['tutti-worker:8080']
    metrics_path: '/metrics'
```

---

## 2. Importing the Grafana Dashboard

1. Open your Grafana instance.
2. Navigate to **Dashboards** $\rightarrow$ **New** $\rightarrow$ **Import**.
3. Upload `grafana-dashboard.json` or paste its raw JSON content.
4. Select your **Prometheus** datasource when prompted.
5. Click **Import**.

---

## 3. Dashboard Metrics Covered

| Section | Panels | Metrics Tracked |
| :--- | :--- | :--- |
| **S3 Proxy** | Request rate, P50/P95/P99 latency, bandwidth | `tutti_proxy_requests_total`, `tutti_proxy_request_duration_seconds_bucket`, `tutti_proxy_bytes_in_total`, `tutti_proxy_bytes_out_total` |
| **Circuit Breakers** | Health state (Closed, Half-Open, Open), probe latency | `tutti_circuit_breaker_state`, `tutti_storage_health_probe_duration_seconds` |
| **Replication Workers** | Queue backlog, copy throughput, delete actions | `asynq_queue_size`, `tutti_worker_bytes_copied_total`, `tutti_delete_propagation_events_total` |
| **Lifecycle Engine** | Objects expired per hour, cumulative space freed | `tutti_lifecycle_objects_deleted_total`, `tutti_lifecycle_bytes_freed_total` |
