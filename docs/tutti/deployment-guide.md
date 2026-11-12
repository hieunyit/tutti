# Tutti Production Deployment & Operations Guide
### Running on Docker, Kubernetes (K8s), and Linux Virtual Machines (VM)

This guide provides end-to-end, production-ready deployment instructions for Tutti — the multi-active resilience, data protection, and routing platform for S3 object storage.

---

## 1. Architecture Overview

Tutti decouples the **data plane** from the **control plane** to achieve high availability, low latency, and zero single points of failure:

```
                  ┌────────────────────────────────────────────────────────┐
                  │                 Clients (SDK, AWS CLI, mc)             │
                  └───────────────────────────┬────────────────────────────┘
                                              │ S3 API (:9669)
                                              ▼
                                   ┌─────────────────────┐
                                   │     Tutti Proxy     │
                                   │  (Stateless Cluster)│
                                   └───────┬─────────────┘
                                           │
         ┌─────────────────────────────────┼─────────────────────────────────┐
         │                                 │                                 │
         ▼                                 ▼                                 ▼
┌───────────────────┐             ┌─────────────────┐             ┌───────────────────┐
│ Primary S3 Node   │◄───────────►│ Redis 7 Data Pl.│◄───────────►│ Secondary S3 Node │
│ (Ceph / MinIO)    │ Replication │ (Events, Queue) │ Replication │ (AWS / Wasabi)    │
└───────────────────┘             └────────▲────────┘             └───────────────────┘
                                           │
                                  ┌────────┴────────┐
                                  │   Tutti Worker  │◄────────────┐
                                  │ (Sync & Replic.)│             │
                                  └────────┬────────┘             │
                                           │                      │ Management (:9671)
                                           ▼                      ▼
                                  ┌─────────────────┐    ┌─────────────────┐
                                  │ PostgreSQL 15+  │    │   Tutti Web UI  │
                                  │ (Control Plane) │    │  (Vue 3 / Nginx)│
                                  └─────────────────┘    └─────────────────┘
```

### Components
1. **Tutti Proxy** (`tutti-proxy`): Stateless data-plane S3 proxy. Intercepts S3 SigV4 traffic, enforces access policies, decodes chunked streaming uploads, manages per-storage circuit breakers, and routes requests according to Protection Groups.
2. **Tutti Worker** (`tutti-worker`): Asynchronous background queue processor and control plane. Manages replication, coordinated switchovers, storage connection testing, and serves the management REST/gRPC API.
3. **PostgreSQL 15+** (`control-plane-db`): Persistent source of truth for console users, S3 credentials, access policies, protection groups, and audit logs.
4. **Redis 7+** (`data-plane`): Distributed queue (Asynq), proxy health heartbeat, traffic rate windows, and writer leasing locks.
5. **Tutti Web UI** (`tutti-web-ui`): Modern web console built with Vue 3 and Naive UI, served via Nginx.

---

## 2. Option 1: Docker & Docker Compose Deployment

Docker Compose is ideal for single-node deployments, staging environments, and edge appliances.

### 2.1 Directory Structure
```
tutti-docker/
├── docker-compose.yml
├── worker-conf.yaml
├── proxy-conf.yaml
└── .env
```

### 2.2 Environment File (`.env`)
```bash
POSTGRES_USER=tutti
POSTGRES_PASSWORD=SuperSecretPgPassword123!
POSTGRES_DB=tutti
REDIS_PASSWORD=SuperSecretRedisPassword123!
TUTTI_MASTER_PASSWORD=SuperSecretMasterKey32BytesLong!
JWT_SECRET=SuperSecretJwtSigningKeyForTutti12345!
```

### 2.3 Worker Configuration (`worker-conf.yaml`)
```yaml
version: 1
appId: "worker-01"

storage:
  main: "minio1"
  storages:
    minio1:
      type: S3
      address: http://minio1:9000
      provider: Minio
      isSecure: false
      connectionAccount:
        accessKeyId: "minioadmin"
        secretAccessKey: "minioadmin"
    minio2:
      type: S3
      address: http://minio2:9000
      provider: Minio
      isSecure: false
      connectionAccount:
        accessKeyId: "minioadmin"
        secretAccessKey: "minioadmin"
  dynamicCredentials:
    enabled: true
    masterPassword: "${TUTTI_MASTER_PASSWORD}"

redis:
  address: "redis:6379"
  password: "${REDIS_PASSWORD}"
  db: 0

controlPlane:
  postgres:
    enabled: true
    dsn: "postgres://tutti:${POSTGRES_PASSWORD}@postgres:5432/tutti?sslmode=disable"
    autoMigrate: true

api:
  enabled: true
  httpPort: 9671
  grpcPort: 9670
  auth:
    enabled: true
    jwtSecret: "${JWT_SECRET}"
    allowManagedUsers: true

log:
  level: info
  format: json
```

### 2.4 Proxy Configuration (`proxy-conf.yaml`)
```yaml
version: 1
appId: "proxy-01"

proxy:
  port: 9669
  cors:
    enabled: true

storage:
  main: "minio1"
  storages:
    minio1:
      type: S3
      address: http://minio1:9000
      provider: Minio
      isSecure: false
    minio2:
      type: S3
      address: http://minio2:9000
      provider: Minio
      isSecure: false
  dynamicCredentials:
    enabled: true
    masterPassword: "${TUTTI_MASTER_PASSWORD}"

redis:
  address: "redis:6379"
  password: "${REDIS_PASSWORD}"
  db: 0

controlPlane:
  postgres:
    enabled: true
    dsn: "postgres://tutti:${POSTGRES_PASSWORD}@postgres:5432/tutti?sslmode=disable"

health:
  failureThreshold: 3
  openDuration: "30s"

replication:
  onEventError: fail
  eventRetryBudget: "2s"
  sync:
    enabled: true
    writeQuorum: 2
    timeout: "15s"

auth:
  requireSignedContentType: false

log:
  level: info
```

### 2.5 Compose File (`docker-compose.yml`)
```yaml
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB}"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: ["redis-server", "--requirepass", "${REDIS_PASSWORD}"]
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "${REDIS_PASSWORD}", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

  worker:
    image: tutti-worker:latest
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    volumes:
      - ./worker-conf.yaml:/etc/tutti/worker.yaml:ro
    environment:
      TUTTI_MASTER_PASSWORD: ${TUTTI_MASTER_PASSWORD}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      REDIS_PASSWORD: ${REDIS_PASSWORD}
      JWT_SECRET: ${JWT_SECRET}
    ports:
      - "9671:9671"
      - "9670:9670"

  proxy:
    image: tutti-proxy:latest
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    volumes:
      - ./proxy-conf.yaml:/etc/tutti/proxy.yaml:ro
    environment:
      TUTTI_MASTER_PASSWORD: ${TUTTI_MASTER_PASSWORD}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      REDIS_PASSWORD: ${REDIS_PASSWORD}
    ports:
      - "9669:9669"

  web-ui:
    image: tutti-web-ui:latest
    restart: unless-stopped
    depends_on:
      - worker
      - proxy
    ports:
      - "8080:80"
    environment:
      API_URL: "http://worker:9671"
      PROXY_URL: "http://proxy:9669"

volumes:
  pgdata:
  redisdata:
```

### 2.6 Starting & Verifying
```bash
# 1. Start all services
docker compose up -d

# 2. Verify containers are healthy
docker compose ps

# 3. Test S3 Proxy endpoint
curl -I http://localhost:9669/

# 4. Access Web UI in browser:
# http://localhost:8080 (Default login: admin / adminadminadmin)
```

---

## 3. Option 2: Kubernetes (K8s) Production Deployment

For enterprise, cloud-native deployments with auto-scaling and zero-downtime rolling updates.

### 3.1 Architecture on K8s
- **PostgreSQL**: Managed PostgreSQL (AWS RDS / Cloud SQL) or CloudNative-PG Operator.
- **Redis**: Bitnami Redis Sentinel or Redis Operator.
- **Tutti Worker**: StatefulSet / Deployment (scale = 1 or active/passive).
- **Tutti Proxy**: Deployment or DaemonSet with Horizontal Pod Autoscaler (HPA) behind a Layer 4 LoadBalancer Service.
- **Tutti Web UI**: Deployment behind an Ingress with cert-manager TLS termination.

### 3.2 Namespace & Secrets (`01-namespace-secrets.yaml`)
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tutti
---
apiVersion: v1
kind: Secret
metadata:
  name: tutti-secrets
  namespace: tutti
type: Opaque
stringData:
  POSTGRES_DSN: "postgres://tutti:SecurePass@postgres-cluster.tutti.svc:5432/tutti?sslmode=verify-full"
  REDIS_PASSWORD: "SuperSecureRedisPassword!"
  MASTER_PASSWORD: "MasterKeyAtLeast32BytesLength!!"
  JWT_SECRET: "SecureJwtSecretForTuttiAuth12345!"
```

### 3.3 Worker Deployment (`02-worker.yaml`)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tutti-worker
  namespace: tutti
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tutti-worker
  template:
    metadata:
      labels:
        app: tutti-worker
    spec:
      containers:
        - name: worker
          image: ghcr.io/hieunyit/tutti-worker:latest
          args: ["-config", "/etc/tutti/worker.yaml"]
          ports:
            - containerPort: 9671
              name: http-api
            - containerPort: 9670
              name: grpc-api
          envFrom:
            - secretRef:
                name: tutti-secrets
          volumeMounts:
            - name: config
              mountPath: /etc/tutti
          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"
            limits:
              cpu: "2"
              memory: "2Gi"
          readinessProbe:
            httpGet:
              path: /tutti/storages
              port: 9671
            initialDelaySeconds: 10
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /tutti/storages
              port: 9671
            initialDelaySeconds: 15
            periodSeconds: 15
      volumes:
        - name: config
          configMap:
            name: tutti-worker-config
---
apiVersion: v1
kind: Service
metadata:
  name: tutti-worker
  namespace: tutti
spec:
  selector:
    app: tutti-worker
  ports:
    - name: http-api
      port: 9671
      targetPort: 9671
    - name: grpc-api
      port: 9670
      targetPort: 9670
```

### 3.4 Proxy Deployment with HPA (`03-proxy.yaml`)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tutti-proxy
  namespace: tutti
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tutti-proxy
  template:
    metadata:
      labels:
        app: tutti-proxy
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: tutti-proxy
                topologyKey: kubernetes.io/hostname
      containers:
        - name: proxy
          image: ghcr.io/hieunyit/tutti-proxy:latest
          args: ["-config", "/etc/tutti/proxy.yaml"]
          ports:
            - containerPort: 9669
              name: s3-proxy
          envFrom:
            - secretRef:
                name: tutti-secrets
          volumeMounts:
            - name: config
              mountPath: /etc/tutti
          resources:
            requests:
              cpu: "1000m"
              memory: "1Gi"
            limits:
              cpu: "4"
              memory: "4Gi"
          readinessProbe:
            tcpSocket:
              port: 9669
            initialDelaySeconds: 5
            periodSeconds: 5
          livenessProbe:
            tcpSocket:
              port: 9669
            initialDelaySeconds: 10
            periodSeconds: 10
      volumes:
        - name: config
          configMap:
            name: tutti-proxy-config
---
apiVersion: v1
kind: Service
metadata:
  name: tutti-s3
  namespace: tutti
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
spec:
  type: LoadBalancer
  selector:
    app: tutti-proxy
  ports:
    - name: s3
      port: 443
      targetPort: 9669
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: tutti-proxy-hpa
  namespace: tutti
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: tutti-proxy
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

### 3.5 Web UI & Ingress (`04-web-ui.yaml`)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tutti-web-ui
  namespace: tutti
spec:
  replicas: 2
  selector:
    matchLabels:
      app: tutti-web-ui
  template:
    metadata:
      labels:
        app: tutti-web-ui
    spec:
      containers:
        - name: web-ui
          image: ghcr.io/hieunyit/tutti-web-ui:latest
          ports:
            - containerPort: 80
          env:
            - name: API_URL
              value: "http://tutti-worker:9671"
---
apiVersion: v1
kind: Service
metadata:
  name: tutti-web-ui
  namespace: tutti
spec:
  selector:
    app: tutti-web-ui
  ports:
    - port: 80
      targetPort: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: tutti-ingress
  namespace: tutti
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - console.tutti.example.com
      secretName: tutti-console-tls
  rules:
    - host: console.tutti.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: tutti-web-ui
                port:
                  number: 80
```

---

## 4. Option 3: Linux Virtual Machine (VM / Bare Metal) Deployment

Step-by-step instructions for running Tutti directly on Ubuntu 22.04 / 24.04 LTS, Debian 12, or RHEL 9 / Rocky Linux 9 using `systemd`.

### 4.1 System Prerequisites
```bash
# 1. Update OS packages
sudo apt-get update && sudo apt-get install -y postgresql-client redis-tools nginx curl

# 2. Create system user and directories
sudo useradd -r -s /bin/false -d /var/lib/tutti tutti
sudo mkdir -p /etc/tutti /var/log/tutti /var/lib/tutti
sudo chown -R tutti:tutti /etc/tutti /var/log/tutti /var/lib/tutti
sudo chmod 750 /etc/tutti /var/log/tutti
```

### 4.2 Building and Installing Binaries
```bash
# On your build machine (with Go 1.22+ installed):
git clone https://github.com/hieunyit/tutti.git
cd tutti

# Build binaries
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/tutti-worker ./service/worker
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/tutti-proxy ./service/proxy
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/tuttictl ./tools/tuttictl
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/tutti-migrate-pg ./tools/tutti-migrate-pg

# Install on target VM
sudo install -m 755 bin/tutti-worker /usr/local/bin/
sudo install -m 755 bin/tutti-proxy /usr/local/bin/
sudo install -m 755 bin/tuttictl /usr/local/bin/
sudo install -m 755 bin/tutti-migrate-pg /usr/local/bin/
```

### 4.3 Database Initialization
```bash
# Run schema migrations against PostgreSQL
tutti-migrate-pg schema \
  --dsn "postgres://tutti:password@127.0.0.1:5432/tutti?sslmode=disable"
```

### 4.4 Systemd Units

#### Worker Service (`/etc/systemd/system/tutti-worker.service`)
```ini
[Unit]
Description=Tutti Object Storage Replication Worker & Management API
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=tutti
Group=tutti
WorkingDirectory=/var/lib/tutti
ExecStart=/usr/local/bin/tutti-worker -config /etc/tutti/worker.yaml
Restart=always
RestartSec=5s
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

#### Proxy Service (`/etc/systemd/system/tutti-proxy.service`)
```ini
[Unit]
Description=Tutti High-Performance S3 Resilient Proxy
After=network.target redis.service
Wants=redis.service

[Service]
Type=simple
User=tutti
Group=tutti
WorkingDirectory=/var/lib/tutti
ExecStart=/usr/local/bin/tutti-proxy -config /etc/tutti/proxy.yaml
Restart=always
RestartSec=5s
LimitNOFILE=1048576
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Enable and start services:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now tutti-worker
sudo systemctl enable --now tutti-proxy
sudo systemctl status tutti-worker tutti-proxy
```

### 4.5 Web UI Deployment via Nginx
Build and copy the Web UI artifacts:
```bash
# Build UI
cd ui
yarn install && yarn build
sudo cp -r build/* /var/www/tutti-ui/
```

Configure Nginx (`/etc/nginx/sites-available/tutti.conf`):
```nginx
server {
    listen 80;
    server_name console.tutti.internal;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name console.tutti.internal;

    ssl_certificate /etc/ssl/certs/tutti.crt;
    ssl_certificate_key /etc/ssl/private/tutti.key;

    root /var/www/tutti-ui;
    index index.html;

    client_max_body_size 0;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy Management REST API
    location /tutti/ {
        proxy_pass http://127.0.0.1:9671;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location /auth/ {
        proxy_pass http://127.0.0.1:9671;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```
Reload Nginx:
```bash
sudo ln -s /etc/nginx/sites-available/tutti.conf /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

---

## 5. Maintenance & Disaster Recovery

### 5.1 Backup PostgreSQL Control Plane
```bash
pg_dump -U tutti -h 127.0.0.1 -Fc tutti > /backups/tutti-controlplane-$(date +%F).dump
```

### 5.2 Restore PostgreSQL Control Plane
```bash
pg_restore -U tutti -h 127.0.0.1 -d tutti --clean /backups/tutti-controlplane-2026-11-11.dump
```

### 5.3 Rotating Master Encryption Key
To rotate `masterPassword` used for sealing S3 credentials, use the `tuttictl` command:
```bash
tuttictl re-encrypt \
  --old-key "$OLD_KEY" \
  --new-key "$NEW_KEY" \
  --config /etc/tutti/worker.yaml
```
