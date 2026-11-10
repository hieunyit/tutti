# Storage accounts

How to prepare the account Tutti uses to talk to each storage.

> **Status.** The *connection account* model described here is **planned (Phase 2 of the [scale plan](./scale-plan.md))**. It is not implemented. Today every (Tutti user, storage) pair needs its own storage credentials, as described in [Today (before Phase 2)](#today-before-phase-2). Use the planned sections to prepare accounts and policies now; they are written to match the plan so the change will not require new accounts.

## Purpose

Tutti sits between clients and storages. Clients should not need storage keys, and a failover must not require client changes. The connection account gives Tutti one dedicated S3 user per storage that it uses for:

- the proxy, when it re-signs client requests for the storage;
- the worker, for replication, diff and migration;
- health checks and the storage connection test.

Because any storage in a protection group can become the primary after a failover, the connection account on **every** storage needs read and write rights, not only the storage that is currently primary.

## Request flow (planned)

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

Scope: `Resource` may be limited to a bucket prefix (for example `app-*`) when Tutti manages only part of a storage. Tutti users' access policies must then stay inside that prefix. Never use the root account (`minioadmin`, a Ceph admin user); the connection test flags it.

## MinIO

### Full-storage policy

This is the scale-plan example for a storage where Tutti manages every bucket and all features are enabled. Save it as `tutti-rw.json`.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["s3:ListAllMyBuckets", "s3:GetBucketLocation"], "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:CreateBucket", "s3:DeleteBucket", "s3:ListBucket", "s3:ListBucketVersions",
                "s3:ListBucketMultipartUploads", "s3:GetBucketVersioning", "s3:PutBucketVersioning",
                "s3:GetBucketTagging", "s3:PutBucketTagging", "s3:GetBucketAcl", "s3:PutBucketAcl",
                "s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration",
                "s3:GetBucketPolicy", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy"],
     "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion",
                "s3:ListMultipartUploadParts", "s3:AbortMultipartUpload",
                "s3:GetObjectTagging", "s3:PutObjectTagging", "s3:GetObjectAcl", "s3:PutObjectAcl"],
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
                "s3:GetBucketTagging", "s3:PutBucketTagging", "s3:GetBucketAcl", "s3:PutBucketAcl",
                "s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration",
                "s3:GetBucketPolicy", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy"],
     "Resource": ["arn:aws:s3:::app-*"]},
    {"Effect": "Allow",
     "Action": ["s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion",
                "s3:ListMultipartUploadParts", "s3:AbortMultipartUpload",
                "s3:GetObjectTagging", "s3:PutObjectTagging", "s3:GetObjectAcl", "s3:PutObjectAcl"],
     "Resource": ["arn:aws:s3:::app-*/*"]}
  ]
}
```

```bash
mc admin policy create myminio tutti-app-rw tutti-app-rw.json
mc admin policy attach myminio tutti-app-rw --user tutti-svc
```

Caveats:

- The connection test creates a probe bucket named `tutti-probe-<random>` (scale plan 2.4). That name is outside `app-*`, so with this policy the create-bucket check fails until the probe name can be chosen inside the allowed prefix (open item for Phase 2). Until then, the test reports the step as not allowed, and the bucket-access checks use a bucket you select.
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

## Test connection (planned, scale plan 2.4)

Runs on add, on edit, and on demand, with the entered settings and the connection account. Each step reports pass, fail, warning or skipped. A fail blocks saving unless an admin ticks "save anyway", which is recorded in the audit log. All probes use a temporary bucket and clean up afterwards.

| # | Check | Passes when | Typical failure and fix |
|---|---|---|---|
| 1 | DNS and TCP to host:port | connected within `httpTimeout` | DNS or refused or timeout: fix the endpoint, port, firewall |
| 2 | TLS (https) | valid chain for the host name, expiry more than 14 days | certificate error: install a valid cert or provide the CA |
| 3 | S3 endpoint | S3 XML response to unauthenticated `GET /` | HTML or redirect: the endpoint is not S3 (check path and port) |
| 4 | Clock skew | difference under 5 minutes (warning above 1 minute) | sync NTP on the proxy or worker host; SigV4 fails otherwise |
| 5 | Authentication (`ListBuckets` signed) | 200 | `InvalidAccessKeyId`: key does not exist on the storage; `SignatureDoesNotMatch`: wrong secret or a proxy that changes the Host header |
| 6 | Root or admin account | not the root user | use the dedicated account from this page |
| 7 | Region | matches `defaultRegion`, or storage reports none | set `defaultRegion` to the reported value |
| 8 | Create bucket | 200 | `AccessDenied` on `s3:CreateBucket`; or skipped if bucket creation is disabled |
| 9 | Write and read (`PutObject`, `GetObject`, `HeadObject`) | content matches | missing `s3:PutObject` or `s3:GetObject` |
| 10 | List (`ListObjectsV2`) | probe object listed | missing `s3:ListBucket` |
| 11 | Multipart (create, put one 5 MiB part, list, abort) | all succeed | missing the multipart action named in the error |
| 12 | Versioning (if provider supports it and used) | two versions written and listed, one deleted | `NotImplemented`: provider has no versioning, disable the feature for this storage |
| 13 | Enabled features (tagging, ACL, lifecycle, policy) | get and put succeed | missing action for that feature; or disable the feature for this storage |
| 14 | Delete (`DeleteObject`, `DeleteBucket`) | 200 or 204 | missing `s3:DeleteObject` or `s3:DeleteBucket`; the test lists any probe left behind |
| 15 | Existing buckets referenced by users, routings or groups | `HeadBucket` and `ListObjectsV2 max-keys=1` succeed | "owned by another account": add the bucket policy above, or link the bucket |
| 16 | Throughput (optional, on demand) | warning only | none required |

Today, the nearest checks are `POST /tutti/storages/test` (runtime storages; `ListBuckets` with the given credential) and `POST /tutti/users/test` (stored credentials of one user on one storage). See [runtime storages](./runtime-storages.md) and [API](./api.md#s3-users-api).

## Today (before Phase 2)

Available now. Each Tutti user needs its own storage credentials on **every** storage used for replication or failover (`complete: true` in `GET /tutti/users`).

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

### What changes in Phase 2

- New users do not need storage keys. Existing per-user credentials become **overrides** and keep working.
- Storages without a connection account keep the per-user behaviour and show "legacy credentials" in the UI.

## Checklist before enabling Phase 2

- [ ] One connection account per storage, not root or admin.
- [ ] Policy attached on every storage, covering the features you enable.
- [ ] Bucket prefix decided; users' policies stay inside it.
- [ ] Existing buckets owned by others have a bucket policy (or are linked, after owner sign-off).
- [ ] Clock synchronised on proxy and worker hosts.
- [ ] Secrets stored sealed with the master password, not in YAML or shell history.
