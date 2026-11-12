# Tutti Kubernetes (k8s) Manifests

Production-ready Kubernetes manifests for deploying Tutti.

## Deployment Order

Apply manifests in numerical order:

```bash
# 1. Namespace
kubectl apply -f 00-namespace.yaml

# 2. Secrets and ConfigMaps (edit passwords before applying!)
kubectl apply -f 01-secrets-configmap.yaml

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

## Verifying Deployment
```bash
# Check all pods are running and healthy
kubectl get pods -n tutti

# Check services
kubectl get svc -n tutti
```
