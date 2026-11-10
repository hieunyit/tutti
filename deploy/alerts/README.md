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

## Enabled Phase 3 / Phase 4 / Phase 6 Alerts

In addition to storage breaker, policy snapshot, proxy response status, and worker task failure alerts, `tutti-alerts.yml` enables:

- `TuttiReplicationLagHigh`: `replication_event_lag > 1000` for 10m.
- `TuttiProtectionGroupCopiesDegraded`: `protection_group_healthy_copies < protection_group_wanted_copies` for 5m.
- `TuttiProtectionGroupQuarantinedMember`: `protection_group_quarantined_members > 0` for 1m.
- `TuttiProtectionGroupFrozen`: `protection_group_frozen == 1` for 10m.
- `TuttiProtectionGroupPromoted`: `increase(protection_group_promotions_total[5m]) > 0`.
- `TuttiControlPlaneOutboxBacklog`: `controlplane_outbox_oldest_seconds > 30` for 2m, or `increase(controlplane_outbox_failures_total[5m]) > 0`.

## Not covered (no metric yet)

The file has commented-out TODO rules for these. They need a metric (or log-based alert) before they can be enabled:

- Worker cannot reach Redis. The worker has no Redis error counter.
- Missing credentials (`storage ... has no credentials for user ...`, answered with 503). Only a log line exists; use a log-based alert.

Metrics in use: `storage_available`, `storage_breaker_transitions_total`, `failover_total`, `policy_snapshot_age_seconds`, `policy_snapshot_refresh_errors_total`, `policy_snapshot_stale_served_total`, `proxy_response_status`, `worker_failed_tasks_total`, `worker_processed_tasks_total`, `replication_event_lag`, `protection_group_wanted_copies`, `protection_group_healthy_copies`, `protection_group_frozen`, `protection_group_generation`, `protection_group_active_writers`, `protection_group_quarantined_members`, `protection_group_promotions_total`, `protection_group_reconcile_errors_total`, `controlplane_outbox_pending`, `controlplane_outbox_oldest_seconds`, `controlplane_outbox_failures_total`.
