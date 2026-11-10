# Tutti alert rules

`tutti-alerts.yml` is a Prometheus rule file. Every expression uses a metric that exists in the code (`pkg/metrics`, `pkg/policy/snapshot.go`). Metrics are exported by the proxy and the worker only when `metrics.enabled: true`, on `metrics.port`.

## Load

Plain Prometheus: add the file under `rule_files` and reload.

```yaml
# prometheus.yml
rule_files:
  - /etc/prometheus/rules/tutti-alerts.yml
```

Validate before loading:

```bash
promtool check rules deploy/alerts/tutti-alerts.yml
# without promtool:
python3 -c 'import yaml,sys; yaml.safe_load(open(sys.argv[1]))' deploy/alerts/tutti-alerts.yml
```

Prometheus Operator: wrap the `groups` list in a `PrometheusRule` object (`spec.groups`), with the labels your Prometheus `ruleSelector` expects. The Helm chart in `deploy/chorus` has a proxy ServiceMonitor and a worker metrics Service; the rules do not filter on `job`, so they apply to every scraped Tutti instance.

Runbook links (`runbook_url`) are relative to the repository (`docs/tutti/runbooks.md#...`). Replace the prefix with the URL where your team hosts the docs if you want clickable links in Alertmanager.

## Thresholds to tune

| Alert | Default | Tune when |
|---|---|---|
| `TuttiStorageBreakerOpen` | `storage_available == 0` for 2m | Lower the `for` only if you accept alerts on short breaker trips. |
| `TuttiStorageBreakerFlapping` | 3 opens in 30m | Traffic and network stability. |
| `TuttiPolicySnapshotStale` | `policy_snapshot_age_seconds > 30` | Keep above `snapshot.pollInterval` (default 1s) with margin. |
| `TuttiProxy5xxRateWarning` / `Critical` | 2% / 10% of responses, 10m / 5m | Your normal error rate. |
| `TuttiProxyAccessDeniedSpike` | more than 1 req/s with status 403 | Absolute rate. Use a ratio against a baseline if traffic varies a lot. |
| `TuttiReplicationTaskFailures` | more than 5% failed attempts, 15m | Retry behavior. Failed attempts include retries. |

Use the alert `for` values to avoid noise. `TuttiAutomaticFailover` fires for every routing failover; it is informational and needs an operator to decide on failback.

## Not covered (no metric yet)

The file has commented-out TODO rules for these. They need a metric (or log-based alert) before they can be enabled:

- Replication lag above a threshold. Lag is only known as `EventMigration.Unprocessed` through the API and `chorctl repl`. Its `LAG` column is the age of the oldest queued task, not a count.
- Protection group copies below the wanted count, and protection group primary changes. `pkg/protectiongroups` has no metrics.
- Worker cannot reach Redis. The worker has no Redis error counter.
- Missing credentials (`storage ... has no credentials for user ...`, answered with 503). Only a log line exists; use a log-based alert.
- Queue depth of the asynq replication queues.

Metrics in use: `storage_available`, `storage_breaker_transitions_total`, `failover_total`, `policy_snapshot_age_seconds`, `policy_snapshot_refresh_errors_total`, `policy_snapshot_stale_served_total`, `proxy_response_status`, `worker_failed_tasks_total`, `worker_processed_tasks_total`.
