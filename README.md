# Tutti

![banner](./docs/media/banner.png)

Tutti is a fork of [Chorus](https://github.com/clyso/chorus) (Clyso GmbH), the vendor-agnostic tool for backup, migration and routing of S3 and OpenStack Swift object storage, extended with resilience features ported from [ReplicaT4](https://github.com/barreeeiroo/ReplicaT4): it accepts the streaming uploads of `mc`/`minio-go`, never silently loses a replication event, can replicate synchronously with a write quorum, fails fast on dead storages with per-storage circuit breakers, and can automatically fail routing over to a replica. Everything is opt-in or upstream-compatible by default, and all Chorus features (replication, live migration, routing, diff check, `chorctl`, Web UI) are retained.

## What Tutti adds to Chorus

| Feature | Config key | Default | Why |
|---|---|---|---|
| Streaming (`aws-chunked`) uploads decoded and verified at the proxy | built in | on | `mc`, `minio-go` and SDKs upload with signed streaming payloads; upstream rejected them. Tampered chunks are rejected and no object is created. |
| Content-Type need not be signed | `auth.requireSignedContentType` | `false` | AWS SigV4 only requires `host` and `x-amz-*` to be signed; `true` restores the strict check. |
| Replication events never silently dropped | `replication.onEventError`, `replication.eventRetryBudget` | `fail`, `2s` | If the event cannot be recorded (Redis OOM/read-only) the client gets `503` and a retry records it, instead of a replica silently diverging. |
| Synchronous replication with write quorum | `replication.sync.{enabled,writeQuorum,timeout,onQuorumFailure}` | off, `0` (all), `30s`, `fail` | A successful PUT confirms the configured number of copies; `X-Tutti-Replicas` reports the copies confirmed before the response. |
| Circuit breakers and health checks | `health.{failureThreshold,openDuration}`, `storage.<name>.healthCheckInterval` | `3`, `30s`, `10s` | Requests to a dead storage fail in milliseconds instead of waiting for `httpTimeout`. |
| Automatic failover | `failover.{enabled,checkInterval,after,maxEventLag}` + per-storage policies in Redis | off, `10s`, `60s`, `0` | Re-route users/buckets to an operator-approved, healthy replica when the main storage stays down. Failback is manual. |
| Resilience API, `chorctl proxies` / `chorctl failover`, UI pages `/health` and `/failover` | n/a | n/a | See live breaker state and manage failover policies and events. |

## Tutti documentation

- [Configuration reference](./docs/tutti/configuration.md)
- [Operations guide](./docs/tutti/operations.md): payload handling, 503 semantics, sync replication, breakers, failover and failback, multi-instance
- [Resilience API, chorctl, Redis keys, metrics, UI](./docs/tutti/api.md)
- [Upgrading from Chorus](./docs/tutti/upgrading-from-chorus.md)
- [Changelog](./CHANGELOG.md)

## What Chorus provides

Chorus enables:
 - Faster data transfers between storages using multiple machines.
 - Resumable transfers with checkpointing on failure.
 - Syncing of existing buckets, objects, and metadata (e.g., ACLs) from source to destination.
 - Real-time capture and propagation of bucket/object changes.
 - User-level and bucket-level replication policies.
 - Routing of S3 requests to different storages based on user-defined rules.
 - Diff check to verify data integrity between storages.
 - Reduce downtime up to zero for switching to different storage provider.

Listed features can be configured per S3 user and per bucket with [management CLI](./tools/chorctl), [REST](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/clyso/chorus/refs/heads/main/proto/gen/openapi/chorus/chorus.swagger.json)/[gRPC](./proto/chorus/chorus.proto) API, or [WebUI](./ui/). The Tutti additions to the API are in [proto/chorus/tutti.proto](./proto/chorus/tutti.proto).

## Components
[Chorus S3 Proxy](./service/proxy) service responsible for routing S3 requests and capturing data change events.
[Chorus Worker](./service/worker) service does actual data replication. Worker also hosts management API and webhook endpoints, so it is a central and the only required component to start.
Communication between Proxy and Worker is done over work queue.
[Asynq](https://github.com/hibiken/asynq) with [Redis](https://github.com/redis/redis) is used as a work queue.

![diagram.png](./docs/media/diagram.png)

For more details, see:
- [Proxy](./service/proxy)
- [Worker](./service/worker)
- [Management CLI](./tools/chorctl)
- [Web UI](./ui)
- [Standalone](./service/standalone) - all-in-one binary. Local playground with zero dependencies.

## Documentation

- Upstream Chorus documentation: [docs.clyso.com](https://docs.clyso.com/docs/products/chorus/overview).
- Project [Blog](https://docs.clyso.com/blog/tags/chorus/).

## Quick start

The Tutti features are enabled in the proxy config; see the [configuration reference](./docs/tutti/configuration.md) for a ready-to-use example per feature. Start from the upstream examples below and add the `replication`, `health` and `failover` sections to the proxy config.

For a hands-on introduction, use the [docker-compose](./docker-compose) example. It includes a step-by-step README and sample configurations—ideal if you’re familiar with Docker and S3.

If using containers is not an option, try to run [standalone](./service/standalone) binary. It bundles all docker-compose services into one executable, letting you follow the same steps without containers.

## Installation

### From source

Requires [Go language](https://go.dev/doc/install). Clone and build with:
```shell
git clone https://github.com/hieunyit/tutti.git && cd tutti

# Run worker
go run ./cmd/worker

# Build worker binary
go build ./cmd/worker
./worker

# Run with custom config
go run ./cmd/worker -config <path-to-worker.yaml>
go run ./cmd/proxy -config <path-to-proxy.yaml>
```

Or install globally:
```shell
# Ensure $GOPATH/bin is in $PATH
export PATH=$PATH:$GOPATH/bin

# Install binaries
go install github.com/hieunyit/tutti/cmd/worker@latest
go install github.com/hieunyit/tutti/cmd/proxy@latest
go install github.com/hieunyit/tutti/cmd/chorus@latest

# Run with config
worker -config <path-to-worker.yaml>
```

### Prebuilt Binaries
Download binaries for Linux, Windows, and macOS from the GitHub releases page of this repository (upstream Chorus releases: [clyso/chorus](https://github.com/clyso/chorus/releases)).

### Docker

Upstream Chorus images (they do not contain the Tutti changes; build Tutti images from the [Dockerfile](./Dockerfile)):
- `harbor.clyso.com/chorus/proxy`
- `harbor.clyso.com/chorus/worker`
- `harbor.clyso.com/chorus/web-ui`

Example:
```shell
docker run -v /path/to/worker.yaml:/bin/config/config.yaml harbor.clyso.com/chorus/worker
```

Built from [Dockerfile](./Dockerfile) and [ui/Dockerfile](./ui/Dockerfile).

See also [docker-compose](./docker-compose) example.

### Kubernetes
Deploy with Helm:
```shell
helm install <release name> oci://harbor.clyso.com/chorus/chorus
```
The chart source is located in [deploy/chorus](./deploy/chorus).

See Helm example values in [deploy/chorus/examples/](./deploy/chorus/examples/).

Proxy and worker expose Prometheus metrics - see [exposed metrics](./deploy/chorus/README.md#exposed-metrics).

## Develop

Tutti is written in Go—no other dependencies are required to build, run, or test.

Want to contribute? See our [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Test
[Test](./test) package contains e2e tests for replications between S3 storages.
It starts:
- 3 embedded [gofakes3](https://github.com/johannesboyne/gofakes3) S3 storages
- embedded Redis [miniredis](https://github.com/alicebob/miniredis)
- Chorus Worker and Proxy services

All listed tools are written in go so test can be run without external dependencies just by:
```shell
go test ./test/...
```

Tests emulate real-world scenarios like:
- replication of existing buckets and objects with metadata
- capturing and syncing live bucket/object changes during migration
- switching between S3 endpoints with and without downtime
- access S3 with Chorus Proxy
- data migration integrity check

### Add APIs
Chorus worker [implements gRPC server](./pkg/api/) defined in [proto/chorus/chorus.proto](./proto/chorus/chorus.proto). It also provides REST API which is automatically generated from the proto file with [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway). GRPC<->REST mappings are defined in [proto/http.yaml](./proto/http.yaml).

Steps to add new API:
1. Add new service or endpoint definition to [proto/chorus/chorus.proto](./proto/chorus/chorus.proto)
2. Add corresponding mapping to [proto/http.yaml](./proto/http.yaml)
3. Run `make proto-gen` to generate go client, server, and openapi definitions to [proto/gen](./proto/gen) directory.
4. Implement new service or endpoint in [pkg/api](./pkg/api/)
5. Support new API in [chorctl](./tools/chorctl) and [WebUI](./ui) clients.

## Relationship to Chorus

Tutti is a fork of [Chorus](https://github.com/clyso/chorus) by Clyso GmbH. Component names (`chorctl`, the `chorus` all-in-one binary, the `chorus.*` protobuf package) are unchanged to stay wire compatible with Chorus. Upstream documentation at [docs.clyso.com](https://docs.clyso.com/docs/products/chorus/overview) applies to everything Tutti does not change; the differences are listed in [Upgrading from Chorus](./docs/tutti/upgrading-from-chorus.md).

## License

- Backend (everything outside `ui/`): [Apache License 2.0](./LICENSE). Original code © Clyso GmbH.
- Web UI (`ui/`): GNU AGPL-3.0, © Clyso GmbH, with Tutti modifications.
- Parts derived from [ReplicaT4](https://github.com/barreeeiroo/ReplicaT4): MIT, © 2025 Diego Barreiro Perez.

See [NOTICE](./NOTICE) for details.
