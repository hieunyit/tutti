# Tutti Kubernetes (k8s) Manifests

Production-ready Kubernetes manifests for deploying Tutti.

## Deployment Order

Apply manifests in numerical order:

```bash
# 1. Namespace
kubectl apply -f 00-namespace.yaml

# 2. Generate unique credentials, then apply ConfigMaps
sh create-secrets.sh
kubectl apply -f 01-configmaps.yaml

# 3. PostgreSQL Database (StatefulSet)
kubectl apply -f 02-postgres.yaml

# 4. Redis Queue & State (StatefulSet)
kubectl apply -f 03-redis.yaml

# 5. Tutti Worker (Replication Engine & Management API)
kubectl apply -f 04-worker.yaml

# 6. Tutti Proxy (S3 Data Plane Load Balancer)
kubectl apply -f 05-proxy.yaml

# 7. Tutti Web UI (Vue 3 Management Console)
kubectl apply -f 06-web-ui.yaml
```

`create-secrets.sh` requires Python 3 and kubectl. It sends freshly generated
credentials directly to Kubernetes without writing them into Git or command
arguments. It uses `kubectl create`, so an existing `tutti-secrets` is preserved
and the command fails rather than rotating passwords, the encryption master key,
or the JWT signing key. Skip this step when upgrading an existing installation.
Back up the secret securely; losing or changing the master key can make stored
credentials unreadable. If you previously deployed credentials from the removed
`01-secrets-configmap.yaml`, replace them using a planned credential/key rotation.

## Verifying Deployment
```bash
# Check all pods are running and healthy
kubectl get pods -n tutti

# Check services
kubectl get svc -n tutti
```
