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

## First login and the admin password

The first start (empty database) creates the console user `admin`. Choose one:

1. **Existing Secret** (the password never appears in values; minimum 12 characters, a shorter one stops the worker):
   ```shell
   kubectl create secret generic tutti-admin --from-literal=password='<min 12 characters>'
   ```
   ```yaml
   auth:
     bootstrapAdmin:
       existingSecret:
         name: tutti-admin
         key: password
       forceChange: true   # default; the admin must change the password at first login
   ```
   `forceChange` maps to `TUTTI_BOOTSTRAP_ADMIN_FORCE_CHANGE`, the Secret to `TUTTI_BOOTSTRAP_ADMIN_PASSWORD`.
2. **Generated password**: leave `existingSecret.name` empty. The worker prints it once on stderr:
   ```shell
   kubectl logs deploy/<release>-worker | grep -A6 'INITIAL ADMIN PASSWORD'
   ```
   Do not ship this output to a log aggregation system without redacting it.

Lost the password? Reset it. All sessions of the user are revoked and a new random password is printed
(the image has no shell, so `kubectl exec` cannot pass `TUTTI_ADMIN_NEW_PASSWORD`; change the generated one at first login):
```shell
kubectl exec deploy/<release>-worker -- /bin/tutti admin reset-password
```
See [First login and password recovery](../../docs/tutti/operations.md#first-login-and-password-recovery).
