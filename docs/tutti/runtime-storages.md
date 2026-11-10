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
| GET | `/tutti/storages/{name}` | None |
| POST | `/tutti/storages` | `{storage: <object>, connectionAccount?, credentials?, testOptions?, force?}` |
| PUT | `/tutti/storages` | `{storage: <object>, connectionAccount?, clearConnectionAccount?, credentials?, confirm?, testOptions?, force?}` |
| PUT | `/tutti/storages/delete` | `{name}` |
| POST | `/tutti/storages/test` | `{name}` (existing storage) or `{storage: <object>, connectionAccount}` (unsaved settings), `options?`: [connection test](./storage-accounts.md#test-connection) |
| PUT | `/tutti/storages/connection-account` | `{name, newAccount, options?, force?}`: [rotation](./storage-accounts.md#rotate-the-connection-account) |
| POST | `/tutti/storages/test-credential` | `{storage: <object>, credential?}`: one `ListBuckets` with a credential. This is the old `/tutti/storages/test`, which now runs the connection test. |

### Request examples

`storage` is a JSON object (the `RuntimeStorage` message), not a string. Field names are the protobuf JSON names (camelCase). Durations are protobuf JSON seconds.

Create (`POST /tutti/storages`):

```json
{
  "storage": {
    "name": "archive",
    "address": "https://rgw.example.internal",
    "provider": "Ceph",
    "defaultRegion": "us-east-1",
    "isSecure": true,
    "httpTimeout": "60s",
    "healthCheckInterval": "10s",
    "rateLimit": { "enabled": true, "rpm": 6000, "includeMetadataApi": false }
  },
  "connectionAccount": { "accessKeyId": "<key id>", "secretAccessKey": "<secret>" },
  "credentials": [
    { "user": "user1", "accessKeyId": "<key id>", "secretAccessKey": "<secret>" }
  ],
  "force": false
}
```

Test unsaved settings (`POST /tutti/storages/test`, runs the connection test; `options` is the `StorageTestOptions` message):

```json
{
  "storage": { "name": "archive", "address": "https://rgw.example.internal", "provider": "Ceph", "defaultRegion": "us-east-1", "isSecure": true },
  "connectionAccount": { "accessKeyId": "<key id>", "secretAccessKey": "<secret>" },
  "options": { "probeBucketPrefix": "app-tutti-probe-" }
}
```

Test an existing storage by name: `{"name": "archive"}`.

Test one credential (`POST /tutti/storages/test-credential`, one `ListBuckets` call):

```json
{
  "storage": { "name": "archive", "address": "https://rgw.example.internal", "provider": "Ceph", "defaultRegion": "us-east-1", "isSecure": true },
  "credential": { "user": "user1", "accessKeyId": "<key id>", "secretAccessKey": "<secret>" }
}
```

Rotate the connection account (`PUT /tutti/storages/connection-account`):

```json
{ "name": "archive", "newAccount": { "accessKeyId": "<new key id>", "secretAccessKey": "<new secret>" }, "force": false }
```

Delete (`PUT /tutti/storages/delete`): `{"name": "archive"}`.

The test endpoints do not save settings (the connection test of an existing storage stores its result as `lastTest`). When editing an existing storage, it can use an existing stored credential. Normal create/update operations verify on the server again, so client-side test results are not trusted as authorization. With a connection account, create/update run the full connection test and refuse on a failing step; `force` saves anyway and records `lastForcedSaveBy/At` (for unreferenced storages only; referenced updates cannot bypass the test).

Durations use protobuf JSON seconds (`"60s"`); the UI also accepts Go-style input such as `1m30s` and converts it. Endpoint URLs cannot contain embedded credentials, query strings, fragments or non-root paths.

## Verification

`TestRuntimeStorageWithoutRestart` in `test/resilience` starts three real MinIO containers, configures only two initially, adds `archive` through REST, waits at most five seconds for proxy discovery without process restart, replicates an object to it, verifies deletion is blocked, removes the replication and deletes the storage.

```bash
go test -run '^TestRuntimeStorageWithoutRestart$' -count=1 ./test/resilience/
go test -race -count=1 ./pkg/objstore ./pkg/api ./service/proxy ./pkg/ratelimit
```

Unit tests also cover cross-instance add/update/delete propagation, encryption capability, URL validation, reference protection and an in-flight request completing on endpoint A while new requests use endpoint B.
