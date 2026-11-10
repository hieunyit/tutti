# Storage accounts

How to prepare the account Tutti uses to talk to each storage.

> **Status.** The *connection account* model (Phase 2 of the [scale plan](./scale-plan.md), backend) is **available**: connection account per storage, credential resolution (override, then connection account), users without storage keys, the connection test, rotation and the periodic re-check. Storages without a connection account keep the per-user behaviour ([Per-user overrides](#per-user-overrides-and-legacy-credentials)). UI support is a separate change; everything below is reachable through the REST API.

## Purpose

Tutti sits between clients and storages. Clients should not need storage keys, and a failover must not require client changes. The connection account gives Tutti one dedicated S3 user per storage that it uses for:

- the proxy, when it re-signs client requests for the storage;
- the worker, for replication, diff and migration;
- health checks and the storage connection test.

Because any storage in a protection group can become the primary after a failover, the connection account on **every** storage needs read and write rights, not only the storage that is currently primary.

## Request flow

```
client --SigV4 with Tutti key (TUT...)--> proxy
proxy: 1) find key, reject disabled or unknown (403 InvalidAccessKeyId)
       2) verify signature
       3) resolve user, check access policy (permission + bucket glob) -> 403 AccessDenied
       4) route: storage S = routing(user, bucket)
       5) re-sign with the connection account of S (or a per-user override) and forward
```

Consequences:

- Revoking a client means disabling or rotating its Tutti key.
- Adding a storage or failing over never changes client configuration.
- Storage keys are never handed to clients.

## Permissions

Copied from the scale plan (section 2.3). The required set depends on the features you enable.

| Needed for | S3 actions | Required when |
|---|---|---|
| Health check, list buckets, user-level replication | `s3:ListAllMyBuckets`, `s3:GetBucketLocation` | always |
| Bucket listing, replication scans | `s3:ListBucket`, `s3:ListBucketVersions` (if versioning) | always |
| Bucket create/delete (client and replication target buckets) | `s3:CreateBucket`, `s3:DeleteBucket` | always, unless buckets are pre-created |
| Object read/write/delete, replication | `s3:GetObject`, `s3:PutObject`, `s3:DeleteObject` | always |
| Versioned objects | `s3:GetObjectVersion`, `s3:DeleteObjectVersion`, `s3:GetBucketVersioning`, `s3:PutBucketVersioning` | provider supports versioning and versioned buckets are used |
| Multipart uploads | `s3:ListBucketMultipartUploads`, `s3:ListMultipartUploadParts`, `s3:AbortMultipartUpload` | always |
| Tag sync | `s3:GetObjectTagging`, `s3:PutObjectTagging`, `s3:GetBucketTagging`, `s3:PutBucketTagging` | `features.tagging` |
| ACL sync | `s3:GetObjectAcl`, `s3:PutObjectAcl`, `s3:GetBucketAcl`, `s3:PutBucketAcl` | `features.acl` |
| Lifecycle sync | `s3:GetLifecycleConfiguration`, `s3:PutLifecycleConfiguration` | `features.lifecycle` |
| Bucket policy sync | `s3:GetBucketPolicy`, `s3:PutBucketPolicy`, `s3:DeleteBucketPolicy` | `features.policy` |

Scope: `Resource` may be limited to a bucket prefix (for example `app-*`) when Tutti manages only part of a storage. Tutti users' access policies must then stay inside that prefix. Never use the root account (`minioadmin`, a Ceph admin user); the connection test flags it (by name only, see step 6). The ACL actions exist on Ceph RGW and AWS S3 but **not on MinIO**.

## MinIO

### Full-storage policy

This is the scale-plan example for a storage where Tutti manages every bucket and all features are enabled, without the ACL actions: **MinIO has no `s3:GetBucketAcl`, `s3:PutBucketAcl`, `s3:GetObjectAcl` or `s3:PutObjectAcl` actions and rejects policies that name them** (`unsupported action`). ACL calls are not policy-controlled on MinIO. Ceph RGW and AWS S3 do have them: add them to the policy there if `features.acl` is enabled. Save it as `tutti-rw.json`.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["s3:ListAllMyBuckets", "s3:GetBucketLocation"], "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:CreateBucket", "s3:DeleteBucket", "s3:ListBucket", "s3:ListBucketVersions",
                "s3:ListBucketMultipartUploads", "s3:GetBucketVersioning", "s3:PutBucketVersioning",
                "s3:GetBucketTagging", "s3:PutBucketTagging",
                "s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration",
                "s3:GetBucketPolicy", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy"],
     "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion",
                "s3:ListMultipartUploadParts", "s3:AbortMultipartUpload",
                "s3:GetObjectTagging", "s3:PutObjectTagging"],
     "Resource": ["arn:aws:s3:::*/*"]}
  ]
}
```

Commands (run with an admin alias; `mc alias set` once):

```bash
mc alias set myminio http://minio.example.internal:9000 <admin-user> '<admin-secret>'
mc admin user add myminio tutti-svc '<new-secret>'          # secret must be at least 8 characters
mc admin policy create myminio tutti-rw tutti-rw.json
mc admin policy attach myminio tutti-rw --user tutti-svc
mc admin user info myminio tutti-svc                        # check the policy is attached
```

Repeat on every storage. Use a different secret per storage.

### Least privilege: bucket prefix `app-*`

Use this when Tutti manages only buckets named `app-*` on a shared storage. Save as `tutti-app-rw.json`. Remove the lines for features you do not enable.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["s3:ListAllMyBuckets"], "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:CreateBucket", "s3:DeleteBucket", "s3:GetBucketLocation", "s3:ListBucket",
                "s3:ListBucketVersions", "s3:ListBucketMultipartUploads",
                "s3:GetBucketVersioning", "s3:PutBucketVersioning",
                "s3:GetBucketTagging", "s3:PutBucketTagging",
                "s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration",
                "s3:GetBucketPolicy", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy"],
     "Resource": ["arn:aws:s3:::app-*"]},
    {"Effect": "Allow",
     "Action": ["s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion",
                "s3:ListMultipartUploadParts", "s3:AbortMultipartUpload",
                "s3:GetObjectTagging", "s3:PutObjectTagging"],
     "Resource": ["arn:aws:s3:::app-*/*"]}
  ]
}
```

```bash
mc admin policy create myminio tutti-app-rw tutti-app-rw.json
mc admin policy attach myminio tutti-app-rw --user tutti-svc
```

Caveats:

- The connection test creates a probe bucket named `tutti-probe-<random>` by default. That name is outside `app-*`: pass `probeBucketPrefix: "app-tutti-probe-"` (the probe bucket becomes `app-tutti-probe-<random>`), or choose an existing bucket inside the prefix with `probeBucket`.
- Tutti users' access policies must use buckets inside `app-*`.

## Ceph RGW

### Create the connection account

```bash
radosgw-admin user create --uid=tutti-svc --display-name="Tutti connection account"
# note keys[0].access_key and keys[0].secret_key in the output
radosgw-admin user info --uid=tutti-svc
```

Buckets that this account creates through Tutti are owned by it.

### Existing buckets owned by someone else

Buckets owned by another account need a bucket policy that grants the connection account access. Save as `tutti-bucket-policy.json`, replacing `BUCKET` and the uid:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["arn:aws:iam:::user/tutti-svc"]},
      "Action": [
        "s3:ListBucket", "s3:ListBucketVersions", "s3:ListBucketMultipartUploads", "s3:GetBucketLocation",
        "s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion",
        "s3:ListMultipartUploadParts", "s3:AbortMultipartUpload",
        "s3:GetObjectTagging", "s3:PutObjectTagging", "s3:GetObjectAcl", "s3:PutObjectAcl"
      ],
      "Resource": ["arn:aws:s3:::BUCKET", "arn:aws:s3:::BUCKET/*"]
    }
  ]
}
```

Apply it with the **owner's** credentials (not the connection account):

```bash
aws --endpoint-url https://rgw.example.internal s3api put-bucket-policy \
  --bucket BUCKET --policy file://tutti-bucket-policy.json
```

Bucket policies do not grant bucket-level administration (create, delete, bucket policy or ACL changes). If Tutti must manage a bucket's policy, ACL or lifecycle, grant those actions explicitly or link the bucket (below).

Caveat for `bucket link`:

```bash
radosgw-admin bucket link --bucket=BUCKET --uid=tutti-svc
```

This changes the bucket owner to `tutti-svc`. The previous owner loses ownership-based management (quota accounting, ownership-based ACLs). Confirm with the storage owner before linking. Prefer the bucket policy when the owner must keep the bucket.

Ceph RGW `ListBuckets` returns only buckets the account owns. The health check still works with a connection account that owns nothing, but the "existing buckets" check (step 16 below) is where the bucket policy matters.

## Test connection

`POST /tutti/storages/test` runs the checks below against **unsaved settings** (`storage` + `connectionAccount`) or an **existing storage** (`name`, optionally with another `connectionAccount` to try it before a rotation). It never answers with an error because a check failed: the result lists every step. Create, update and rotation run the same test and **refuse to save when a step fails**, unless `force: true` (then who and when are stored in the storage document as `lastForcedSaveBy` / `lastForcedSaveAt`). The last result of an existing storage is stored with it (`lastTest`) and returned by `GET /tutti/storages` and `GET /tutti/storages/{name}`.

Request:

```json
{
  "name": "archive",
  "connectionAccount": {"accessKeyId": "tutti-svc", "secretAccessKey": "..."},
  "options": {
    "probeBucketPrefix": "app-tutti-probe-",
    "probeBucket": "",
    "features": {"versioning": true, "tagging": false, "acl": false, "lifecycle": false, "policy": false},
    "throughput": false
  }
}
```

- `probeBucketPrefix`: prefix of the temporary bucket (default `tutti-probe-`; lowercase letters, digits, `-`, `.`). Use a prefix inside the bucket prefix your account policy is limited to.
- `probeBucket`: use this existing bucket instead of creating one (bucket creation not allowed). The bucket is never deleted or reconfigured; only the probe objects under `tutti-probe/<run id>/` are written and removed. Bucket-level feature checks are skipped on it.
- `features`: which optional steps run. Omitted: versioning runs for the Minio and Ceph providers, no other feature.
- `throughput`: also run step 17.

Result (abridged):

```json
{
  "testedAt": "2026-11-10T09:51:08Z", "ok": false, "hasWarnings": true, "durationMs": "812",
  "accessKeyId": "tutti-nodel", "probeBucket": "tutti-probe-9f2c61aa", "leftovers": [], "kind": "full",
  "steps": [
    {"id": "dns-tcp", "number": 1, "name": "DNS + TCP", "status": "pass", "call": "resolve host, connect to port",
     "latencyMs": "1", "message": "connected to 10.0.0.12:9000", "missingPermission": "", "hint": ""},
    {"id": "authentication", "number": 5, "name": "Authentication", "status": "pass", "call": "ListBuckets", "latencyMs": "4",
     "message": "authenticated as tutti-nodel (3 bucket(s) visible)", "missingPermission": "", "hint": ""},
    {"id": "write", "number": 9, "name": "Write", "status": "pass", "call": "PutObject", "latencyMs": "6",
     "message": "wrote 21 bytes", "missingPermission": "", "hint": ""},
    {"id": "delete", "number": 15, "name": "Delete", "status": "fail", "call": "DeleteObject, DeleteBucket", "latencyMs": "31",
     "message": "DeleteObject failed: AccessDenied: Access Denied.; DeleteObjectVersion failed: AccessDenied: Access Denied.",
     "missingPermission": "s3:DeleteObject, s3:DeleteObjectVersion",
     "hint": "Remove the leftovers manually and grant the missing permissions."}
  ]
}
```

`status` is `pass`, `fail`, `warn` or `skip`. A failed early step skips the steps that depend on it (for example a TLS failure skips steps 3 to 16). Cleanup always runs; whatever could not be removed is listed in `leftovers` and in the message of step 15.

| # | id | Check | Passes when | Typical failure and fix |
|---|---|---|---|---|
| 1 | `dns-tcp` | resolve host, connect to port | connected within `httpTimeout` (max 30s) | DNS, refused or timeout: fix the endpoint, port, firewall |
| 2 | `tls` | TLS handshake (https) | valid chain for the host name; expiry more than 14 days (else `warn`) | certificate error text and expiry date, "custom CA required": install the CA on the Tutti hosts |
| 3 | `endpoint` | unauthenticated `GET /` | S3 XML response, not HTML or a redirect | "Endpoint does not look like S3 (got status/content-type)": use the S3 API address |
| 4 | `clock-skew` | `Date` header of that response | skew under 1 minute (`warn` up to 5 minutes, `fail` above) | sync NTP; SigV4 fails otherwise |
| 5 | `authentication` | `ListBuckets` signed with the account | 200 | `InvalidAccessKeyId`, `SignatureDoesNotMatch`, `AccessDenied`... each with a hint |
| 6 | `root-account` | heuristic on the account | not a well-known administrator name | `warn`: use the dedicated account. Only the access key id and the owner name/id (`minioadmin`, `root`, `admin`, `rgwadmin`) are checked: MinIO reports the same fixed owner for every user, so it cannot be used, and the root user cannot be detected otherwise without admin credentials |
| 7 | `region` | `GetBucketLocation` on the probe (or an existing) bucket | equals `defaultRegion`, or none configured | `warn` "Storage reports region X": set `defaultRegion` |
| 8 | `create-bucket` | `CreateBucket` of the probe bucket | 200 | missing `s3:CreateBucket`; with `probeBucket` the step is `skip` |
| 9 | `write` | `PutObject` | 200 | missing `s3:PutObject` |
| 10 | `read` | `GetObject` + `HeadObject` | content matches | missing `s3:GetObject` |
| 11 | `list` | `ListObjectsV2` | probe object listed | missing `s3:ListBucket` |
| 12 | `multipart` | create, upload one 5 MiB part, `ListParts`, `ListMultipartUploads`, abort | all succeed | the missing action of the failing call (`s3:ListMultipartUploadParts`, `s3:ListBucketMultipartUploads`, `s3:AbortMultipartUpload`) |
| 13 | `versioning` | `GetBucketVersioning`, enable on the probe, two writes, `ListObjectVersions`, delete one version | all succeed | missing versioning action; `warn` "provider does not support versioning" on `NotImplemented` |
| 14 | `features` | tagging, ACL, lifecycle, policy get+put (one entry per enabled feature) | all succeed | missing action per feature; `skip` if none enabled |
| 15 | `delete` | `DeleteObject`, remaining versions, `DeleteBucket` | all succeed | missing `s3:DeleteObject`, `s3:DeleteObjectVersion` or `s3:DeleteBucket`; leftovers listed |
| 16 | `existing-buckets` | `HeadBucket` + `ListObjectsV2 max-keys=1` on every bucket that bucket routings and bucket replications place on this storage, and on every visible bucket matching the bucket globs of the access policies of users that reach it (users with a bucket routing or replication to it; every user for the main storage) | accessible | `fail` "Account cannot access bucket B (owned by another account)" with a copyable bucket policy; `warn` if the bucket does not exist yet |
| 17 | `throughput` | 3 x PUT and GET of 1 MiB (only with `throughput: true`) | always (informational: MiB/s, p95) | warning only |

**Periodic re-check.** For every storage with a connection account the proxy health checker runs steps 1 to 5 on its normal `healthCheckInterval`. A problem appears as the storage health error with the step name, for example `authentication: SignatureDoesNotMatch` or `dns-tcp: connection refused` (`lastError` of the storage in `GET /tutti/proxies`). Host, TLS and endpoint failures count as outages like before (circuit breaker, failover). Authentication and clock problems are **only reported**: the storage still answers, so the breaker stays closed and no failover starts; the error clears with the next clean check. Storages without a connection account keep the old `ListBuckets` probe with the first per-user credential.

Other API calls: `POST /tutti/users/test` (stored credentials of one user on one storage; with a connection account it tests the account) and `POST /tutti/storages/test-credential` (the former `/tutti/storages/test`: one `ListBuckets` with a given credential).

## Connection account: set up and manage

### Config storage (read-only in the API)

```yaml
storage:
  storages:
    main:
      type: S3
      address: "minio.example.internal:9000"
      provider: Minio
      defaultRegion: us-east-1
      connectionAccount:
        accessKeyID: tutti-svc
        secretAccessKey: "<secret>"
```

Config-defined accounts show `connectionAccount.source: "config"`, cannot be changed or rotated through the API (`FailedPrecondition`) and are not sealed: protect the config file. `credentials.<user>` in the same storage keep working as per-user overrides.

### Managed storage (API)

Needs `storage.dynamicCredentials` with a `masterPassword` (the account is stored sealed, like dynamic credentials). `POST /tutti/storages`:

```json
{
  "storage": {"name": "archive", "address": "http://minio-b.example.internal:9000", "provider": "Minio", "defaultRegion": "us-east-1"},
  "connectionAccount": {"accessKeyId": "tutti-svc", "secretAccessKey": "<secret>"},
  "testOptions": {"probeBucketPrefix": "tutti-probe-"},
  "force": false
}
```

The storage is created only if the test passes (or `force`). The response is the storage: `connectionAccount` (`configured`, `accessKeyId`, `source`, never the secret), `lastTest`, `lastForcedSaveBy/At`, `legacyCredentials`. `PUT /tutti/storages` (update) takes `connectionAccount` to set one, omits it to keep the stored one, `clearConnectionAccount: true` to go back to legacy per-user credentials; it re-runs the test with the account that will be effective.

### Resolution: which key signs a request

For a request of user U to storage S, the proxy, the worker (replication, diff, migration, switch) and failover use the first that exists:

1. the **override** of U on S (config `credentials.<U>` or dynamic credentials): unchanged behaviour;
2. the **connection account** of S;
3. otherwise the error `storage S has no credentials for user U`. The proxy answers **503** `ServiceUnavailable` with that message in the S3 error XML and counts it (`noCredentials` in the proxy statistics); the worker fails the task with the same message.

Health checks and the connection test use the connection account when present.

### Users without storage keys

```bash
curl -s -X POST $API/tutti/users -H 'Content-Type: application/json' -d '{
  "name": "alice", "createAccessKey": true,
  "policy": {"permission": "read-write", "buckets": ["app-*"]}
}'
```

`credentials` may be empty when **every S3 storage has a connection account**; otherwise every storage without one needs an override in `credentials` (the main storage must always resolve). `GET /tutti/users` reports per storage `mode`: `connection-account`, `override` or `missing`, and `complete: true` when every storage resolves. A user created without any credentials exists as a lightweight record in the config DB (Redis hash `tutti:s3users`), so it is listed and can be used in replications and routings. An override can be added later with `PUT /tutti/users/credentials` (tenant isolation, per-user quota or billing on the storage side).

### Rotate the connection account

1. Create the new account on the storage (same policy). Keep the old one enabled.
2. `PUT /tutti/storages/connection-account`:

```json
{"name": "archive", "newAccount": {"accessKeyId": "tutti-svc-2", "secretAccessKey": "<secret>"}, "options": {}, "force": false}
```

The new account is tested first; the switch is refused if a step fails (unless `force`). The switch is one atomic write of the storage document that also bumps the storages version. The previous account is kept (sealed) in the document until the next rotation and returned as `connectionAccount.previousAccessKeyId`.

3. **Propagation delay.** Proxies and workers poll the storages version every **2 seconds at most** (the poll interval is capped at 2s when managed storages are enabled). Until a process has polled, it keeps signing with the old key, so there is no downtime as long as the old key stays valid. **Do not disable or delete the old account on the storage before `connectionAccount.safeToRevokePreviousAfter`** (rotation time + 60 seconds). Requests already in flight finish with the key they were signed with. Config storages are rotated by changing the config file and restarting.

## Per-user overrides and legacy credentials

A Tutti user may have its **own** storage credentials on a storage: an *override*. It wins over the connection account for that user and storage. This was the only model before the connection account, so existing setups keep working unchanged. A storage without a connection account needs an override for every user that reaches it, including for replication and failover (`complete: true` in `GET /tutti/users`, `mode: override` on every storage).

### Create the per-user account on the storage

MinIO (per user, same policy file as above):

```bash
mc admin user add myminio alice '<alice-secret>'
mc admin policy attach myminio tutti-rw --user alice
```

Ceph RGW (per user):

```bash
radosgw-admin user create --uid=alice --display-name="Alice"
# use keys[0].access_key and keys[0].secret_key
```

### Give the credentials to Tutti

Option A, through the API (needs `storage.dynamicCredentials.enabled: true` and a `masterPassword` on the worker and the proxies). Verifies each credential before saving:

```bash
curl -s -X POST http://localhost:9671/tutti/users \
  -H 'Content-Type: application/json' -d '{
    "name": "alice",
    "credentials": [
      {"storage": "main",     "accessKeyId": "ALICEKEY1", "secretAccessKey": "secret1"},
      {"storage": "follower", "accessKeyId": "ALICEKEY2", "secretAccessKey": "secret2"}
    ],
    "verify": true,
    "createAccessKey": true,
    "policy": {"permission": "read-write", "buckets": ["*"]}
  }'
```

Option B, in the worker config (read-only in the API; changes need a restart):

```yaml
storage:
  storages:
    main:
      type: S3
      address: "minio.example.internal:9000"
      provider: Minio
      credentials:
        alice:
          accessKeyID: ALICEKEY1
          secretAccessKey: secret1
```

Rotate a dynamic user's credentials on one storage with `PUT /tutti/users/credentials` (`{"user", "credential": {"storage", "accessKeyId", "secretAccessKey"}, "verify": true}`). Config-defined credentials can only be changed in the config file.

Check the result: `GET /tutti/users` shows `complete` and each storage's `accessKeyId` (never the secret).

### Migration

- Existing per-user credentials (config `credentials.<user>` and dynamic) are overrides and keep working. Nothing needs to change when a connection account is added to a storage; users without an override start to resolve through it.
- Storages without a connection account keep the per-user behaviour and report `legacyCredentials: true`.

## Checklist before adding a connection account

- [ ] One connection account per storage, not root or admin.
- [ ] Policy attached on every storage, covering the features you enable.
- [ ] Bucket prefix decided; users' policies stay inside it.
- [ ] Existing buckets owned by others have a bucket policy (or are linked, after owner sign-off).
- [ ] Clock synchronised on proxy and worker hosts.
- [ ] Secrets stored sealed with the master password, not in YAML or shell history.
