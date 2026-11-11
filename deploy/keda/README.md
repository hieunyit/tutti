# KEDA Worker Autoscaler for Tutti

This directory provides Kubernetes Event-driven Autoscaling (KEDA) manifests to dynamically scale `tutti-worker` replicas according to queue depth and replication backlog.

## Triggers

1. **Prometheus Metric Trigger (`sum(replication_unprocessed_events)`)**:
   - Monitored by `pkg/policy/replication_metrics.go`.
   - Aggregates unprocessed replication events across all users and bucket replications.
   - Recommended threshold: 50 backlog events per worker replica.

2. **Redis Task Queue Trigger (`asynq:{default}:pending`)**:
   - Direct Redis list length measurement of pending tasks in Asynq.
   - Provides near-instant scale-out when bursts of replication tasks are enqueued.
   - Configured with `TriggerAuthentication` referencing the Redis password secret.

## Applying the ScaledObject

```bash
kubectl apply -f deploy/keda/worker-scaler.yaml
```

Verify KEDA status:

```bash
kubectl get scaledobject tutti-worker-scaler
kubectl get hpa keda-hpa-tutti-worker-scaler
```
