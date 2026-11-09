# Runtime storage management

Administrators can add S3 storages from **Storages → Add storage**, then edit them in the storage detail **Settings** tab. Storage definitions from configuration remain read-only. Changing `storage.main` still requires configuration and a restart.

## Enable

Enable `storage.dynamicCredentials` with encryption and the same master password on the worker and all proxies. Managed definitions live in the Redis configuration database (`tutti:storages`, with a version key); user credentials use the existing sealed dynamic-credentials store. List responses contain connection settings and access key IDs, never secret keys.

Worker and proxies poll managed changes at most every two seconds under healthy Redis connectivity. They update clients, health probes and rate-limit settings without restarting. Requests already using an old client finish on that client; subsequent requests use the new connection settings.

## Add and edit

1. Enter a unique name, S3 provider, endpoint/TLS, region, request timeout, health-check interval and optional rate limit.
2. Supply credentials for existing users (leave a user empty to skip it). At least one complete credential pair is required for verification. Test each supplied pair before saving. HTTPS checks the server certificate; verification requires listing buckets.

Settings show previous values for changed fields. Changing a field invalidates the previous connection-test result. A referenced storage requires explicit confirmation and successful verification before saving. Config-defined storages show instructions to edit configuration instead.

Deleting a managed storage requires typing its name. The backend blocks deletion of the main storage and any storage referenced by replication, user/bucket routing, failover policy/event or diff report. Reference lookup failures block deletion too.

## REST API

All mutation/test endpoints require an administrator session. The list is available to viewers.

| Method | Path | Request |
|---|---|---|
| GET | `/tutti/storages` | None |
| POST | `/tutti/storages` | `{storage, credentials}` |
| PUT | `/tutti/storages` | `{storage, credentials?, confirm?}` |
| PUT | `/tutti/storages/delete` | `{name}` |
| POST | `/tutti/storages/test` | `{storage, credential?}` |

The test endpoint does not save settings. When editing an existing storage, it can use an existing stored credential. Normal create/update operations verify on the server again, so client-side test results are not trusted as authorization. A `force` override is available to API administrators for unreferenced storages; referenced updates cannot bypass verification.

Durations use protobuf JSON seconds (`"60s"`); the UI also accepts Go-style input such as `1m30s` and converts it. Endpoint URLs cannot contain embedded credentials, query strings, fragments or non-root paths.

## Verification

`TestRuntimeStorageWithoutRestart` in `test/resilience` starts three real MinIO containers, configures only two initially, adds `archive` through REST, waits at most five seconds for proxy discovery without process restart, replicates an object to it, verifies deletion is blocked, removes the replication and deletes the storage.

```bash
go test -run '^TestRuntimeStorageWithoutRestart$' -count=1 ./test/resilience/
go test -race -count=1 ./pkg/objstore ./pkg/api ./service/proxy ./pkg/ratelimit
```

Unit tests also cover cross-instance add/update/delete propagation, encryption capability, URL validation, reference protection and an in-flight request completing on endpoint A while new requests use endpoint B.
