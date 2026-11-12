# Tutti Helm Chart

Production Helm chart for deploying **Tutti** — the multi-active resilience, data protection, and routing platform for S3 object storage.

## Features
- **Stateless S3 Proxy**: Horizontal autoscaling (HPA), per-storage circuit breaking, SigV4 authentication.
- **Worker Management & Replication Engine**: Async queue processing, coordinated switchover, PostgreSQL control plane.
- **PostgreSQL Control Plane**: Decoupled persistent storage for credentials, users, policies, and audit logs.
- **Redis State & Work Queue**: Integrated Bitnami Redis dependency or connection to existing Redis cluster/Sentinel.
- **Enterprise Web Console**: Web UI for operations and monitoring with Ingress TLS support.

## Installation

### 1. From Local Chart
```shell
helm install tutti ./deploy/tutti \
  --namespace tutti \
  --create-namespace \
  -f values.yaml
```

### 2. Upgrading
```shell
helm upgrade tutti ./deploy/tutti \
  --namespace tutti \
  -f values.yaml
```

## Quick-Start Values

```yaml
redis:
  enabled: true
  auth:
    enabled: true
    password: "SuperSecureRedisPassword!"

controlPlane:
  postgres:
    enabled: true
    dsn: "postgres://tutti:SuperSecretPgPassword@postgres.tutti.svc:5432/tutti?sslmode=disable"
    autoMigrate: true

worker:
  replicaCount: 1
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2000m
      memory: 2Gi

proxy:
  replicaCount: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
    targetCPUUtilizationPercentage: 70
  service:
    type: LoadBalancer

ui:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - host: tutti.example.com
        paths:
          - path: /
            pathType: Prefix
```
