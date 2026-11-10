# Docker-compose

## Before start

**Requirements:**
- Docker
- S3 client (e.g. [s3cmd](https://github.com/s3tools/s3cmd))
    ```shell
    brew install s3cmd
    ```
- Chorus management CLI [chorctl](../tools/chorctl)
    ```shell
    brew install clyso/tap/chorctl
    ```
**Structure:**
```
├── docker-compose
│   ├── docker-compose.yml    # docker-compose file
│   ├── FakeS3Dockerfile      # Dockerfile for in-memory S3 endpoint
│   ├── proxy-conf.yaml       # example config for chorus-proxy
│   ├── README.md
│   ├── s3cmd-follower.conf   # s3cmd credentials for follower storage
│   ├── s3cmd-main.conf       # s3cmd credentials for main storage
│   ├── s3cmd-proxy.conf      # s3cmd credentials for proxy storage
│   ├── s3-credentials.yaml   # chorus common config with S3 credentials
│   └── worker-conf.yaml      # example config for chorus-worker
```

Please also review [docker-compose.yml](./docker-compose.yml) file. It contains comments about services and their configuration.

## Bucket replication example

To run chorus with docker compose:
1. Clone repo:
    ```shell 
    git clone https://github.com/clyso/chorus.git && cd chorus
    ```
2. Start chorus `worker`,`proxy` with fake main and follower S3 backends:
    ```shell
    docker-compose -f ./docker-compose/docker-compose.yml --profile fake --profile proxy up
    ```
3. Check chorus config with CLI:
    ```
    % chorctl storage
    NAME            ADDRESS                          PROVIDER     USERS
    follower        http://fake-s3-follower:9000     Other        user1
    main [MAIN]     http://fake-s3-main:9000         Other        user1
    ```
    And check that there are no ongoing replications with `chorctl dash`.
4. Create a bucket named `test` in the `main` storage:
    ```shell
    s3cmd mb s3://test -c ./docker-compose/s3cmd-main.conf
    ```
    Check that main bucket is empty:
    ```shell
    s3cmd ls s3://test -c ./docker-compose/s3cmd-main.conf
    ```
5. Upload contents of this directory into `main`:
    ```shell
    s3cmd sync ./docker-compose/  s3://test -c ./docker-compose/s3cmd-main.conf
    ```
    Check that follower bucket is still not exists:
    ```shell
    s3cmd ls s3://test -c ./docker-compose/s3cmd-follower.conf
    ERROR: Bucket 'test' does not exist
    ERROR: S3 error: 404 (NoSuchBucket): The specified bucket does not exist
    ```
6. See `test` bucket available for replication:
    ```shell
    % chorctl repl buckets -u user1 -f main -t follower
    BUCKET
    test
    ```
7. Enable replication for the bucket:
    ```shell
    chorctl repl add -u user1 -b test -f main -t follower
    ```
    > **Tip**: For user-level replication (all existing and future buckets), omit the `-b` flag:
    > ```shell
    > chorctl repl add -u user1 -f main -t follower
    > ```
8. Check replication progress with:
    ```shell
    chorctl dash
    ```
9. Check that data is actually replicated to the follower:
    ```shell
    s3cmd ls s3://test -c ./docker-compose/s3cmd-follower.conf
    ```
10. Now do some live changes to bucket using proxy:
    ```shell
    s3cmd del s3://test/worker-conf.yaml -c ./docker-compose/s3cmd-proxy.conf
    ```
11. List main and follower contents again to see that the file was removed from both. Feel free to play around with storages using `s3cmd` and preconfigured configs [s3cmd-main.conf](./s3cmd-main.conf) [s3cmd-follower.conf](./s3cmd-follower.conf) [s3cmd-proxy.conf](./s3cmd-proxy.conf).

## Demo sign-in, failover and proxy accounts

The demo config enables the features below. Everything here is for local demos only.

- **Sign-in:** the management API and web UI (http://localhost:8080) require login (`api.auth` in [worker-conf.yaml](./worker-conf.yaml)).
  Demo users: `admin` / `admin` (role admin) and `viewer` / `viewer` (read-only). **Change or remove these users before any real use.**
  Production hashes are bcrypt, generated as described in [service/worker/config.yaml](../service/worker/config.yaml).
- **Failover:** the proxy runs the automatic failover controller (`failover.enabled: true` in [proxy-conf.yaml](./proxy-conf.yaml)).
  A storage only fails over if it has an enabled failover policy with targets, set through the Resilience API, `chorctl`, or the UI `/failover` page.
  See [docs/tutti/operations.md](../docs/tutti/operations.md#automatic-failover).
- **Control-plane PostgreSQL & Audit log:** [docker-compose.yml](./docker-compose.yml) starts a `postgres` (PostgreSQL 16)
  service and [worker-conf.yaml](./worker-conf.yaml) enables `controlPlane.backend: postgres` with `autoMigrate: true`.
  All control-plane configuration and audit history (`/audit-log` in the UI) are persisted in PostgreSQL (`postgres-data` volume)
  and projected to Redis via the transactional outbox so `proxy` continues reading Redis without a direct PostgreSQL connection.
  The worker's DSN is built from `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` (default `tutti`) through
  `CFG_CONTROLPLANE_POSTGRES_DSN`, so overriding them needs no edit of `worker-conf.yaml` (URL-encode special characters
  in the password). Automatic failover decided by the proxy is applied by the worker through PostgreSQL, so keep the
  `worker` running when `failover.enabled` is set in `proxy-conf.yaml`.
- **Proxy accounts:** creating proxy accounts requires a master password. [s3-credentials.yaml](./s3-credentials.yaml) sets a demo
  `storage.dynamicCredentials.masterPassword` (at least 12 characters), shared by worker and proxy. Config values are not env-expanded,
  so to keep the password out of the file, set `CFG_STORAGE_DYNAMICCREDENTIALS_MASTERPASSWORD` in the environment of both services
  (the loader's `CFG_` override). Do not change the master password once accounts exist, because stored secrets can no longer be decrypted.

Start the stack with the fake S3 backends and the proxy, rebuilding images from source:
```shell
docker-compose -f ./docker-compose/docker-compose.yml --profile fake --profile proxy up --build
```

## Where to go next
Try to add more worker instances to speed up replication process and observe how they are balancing the load. To do this, duplicate existing `worker` service in [docker-compose.yml](./docker-compose.yml) with omitted ports to avoid conflicts:
```yaml
  worker2:
    depends_on:
      - redis
    build:
      context: ../
      args:
        SERVICE: worker
    volumes:
    - type: bind
      source: ./worker-conf.yaml
      target: /bin/config/config.yaml
    - type: bind
      source: ./s3-credentials.yaml
      target: /bin/config/override.yaml
```
The same can be done for `worker3`, `worker4`, etc. And for `proxy` service.

Replace S3 credentials in [./s3-credentials.yaml](./s3-credentials.yaml) with your own s3 storages and start docker-compose without fake backends:
```shell
docker-compose -f ./docker-compose/docker-compose.yml --profile proxy up
```

Explore more features with [chorctl](../tools/chorctl) (`chorctl --help`) and [WebUI](../ui).

## How-to
To tear-down:
```shell
docker-compose -f ./docker-compose/docker-compose.yml down
```
To tear down and wipe all replication metadata:
```
docker-compose -f ./docker-compose/docker-compose.yml down -v

```
Rebuild images with the latest source changes (`--build`):
```shell
docker-compose -f ./docker-compose/docker-compose.yml --profile fake --profile proxy --build up
```
To use chorus images from registry instead of building from source replace in [docker-compose.yml](./docker-compose.yml):
```yaml
  worker:
    build:
      context: ../
      args:
        SERVICE: worker
``` 
to:

```yaml
  worker:
    image: "harbor.clyso.com/chorus/worker:latest" 
```
And similar for `web-ui` and `proxy`.

