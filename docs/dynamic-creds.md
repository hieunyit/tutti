# Dynamic credentials

Tutti supports **dynamic** and **static** configurations.

**Static** config is provided on startup and does not change until the next restart. It can be provided as path to yaml file config, envars or cli arguments. **Static** config includes things like server port, logging level, redis connection and list of storages in storage config.
Since main Tutti use case is data migration and backup from one storage to another, where storage endpoints are known and prepared in advance, **static** config is usually sufficient for storage endpoints.

Migration or backup process itself is configured via Tutti management API. It can be started/stopped per bucket or per user (all user buckets) at runtime.
This makes all API configs like routings and replicaitons **dynamic** by nature. **Dynamic** configs are managed via Tutti management API and stored in redis.

Unlike storage endpoints, credentials may change more often. Credentials can be rotated, new users can be added or removed during tutti migration process.
If credentials are part of **static** config, tutti users will have to access deployment environment to update yaml files and restart tutti services to apply new credentials.
To address this issue, Tutti supports **dynamic** credentials that can be updated at runtime via management API without service restarts.

There are two key challenges with **dynamic** credentials:
1. **Security** - credentials should not be stored in plain text in redis. They should be encrypted.
2. **Distribution** - rotated and new credentials should be distributed to all tutti workers and proxy replicas in the cluster.

Below is description of how Tutti addresses these challenges.

## Security

Tutti needs to be able to read credentials in plain text to access storages. 
This eliminates secure options like one-way hashing for credentials storage.

To address the security challenge, Tutti uses symmetric encryption to seal secrets before storing them in redis in the following way:
1. User provides `master password` on tutti service startup via envar or cli argument.
2. Tutti generates random `128-bit Salt` and stores it in Redis or reads it from Redis if it already exists.
3. Tutti derives `256-bit Secret Key` from `master password` and `salt` using [Argon2](https://en.wikipedia.org/wiki/Argon2).
4. Tutti evicts master password from memory after secret key is derived and uses only the derived secret key for sealing/unsealing secrets.
5. Tutti seals/unseals secrets using `AES-GCM` with the derived secret key before storing them in redis and after reading them from redis.

## Distribution

Tutti is a distributed system with multiple `worker` and `proxy` replicas.

As soon as user adds/rotates credentials via management API, Tutti `proxy` service should be able immediately use new credentials without failing requests.
Also, all `worker` services should be able to pick up new credentials on task retry instead of dropping tasks or retry indefinitely with stale credentials.

There are two common approaches to notify about updates: **push-based** and **pull-based**.

**Push-based** approach notifes all tutti serives about credentials changes as soon as they happen.
It can be implemented via Redis Pub/Sub. The downside of this approach is that if some services miss the notification (e.g. due to network issue or restart), they will continue using stale credentials until next rotation.

**Pull-based** approach makes tutti services check for credentials updates periodically and on failure.
This approach is more resilient to missed notifications since services will eventually pick up new credentials but users will face at least some failed requests until new credentials are picked up. 

If tutti credentials are updated in advance of their actual expiration, both approaches can work well. But push-based approach is more complex and less resilient to failures.

Pull-based approach:
1. On startup, tutti services read credentials along with their integer `version` from redis and cache them in memory.
2. When user rotates or adds new credentials via management API, tutti increments `version` in redis.
3. Every `N` seconds and on `401 Unauthorized` response from storage endpoints, tutti services trigger credentials refresh process:
   1. Read credentials and their `version` from redis.
   2. If `version` is greater than cached `version`, unseal new credentials and update cached credentials and `version`.
   3. If `version` is equal to cached `version`, do nothing.



## Runtime-managed proxy accounts

Besides storage credentials (`auth.useStorage`) and static `auth.custom` credentials, the proxy authenticates
**dynamic proxy accounts** created through the management API. An account maps a generated access key to an existing
storage user: the proxy verifies the SigV4 (or presigned/V2) signature with the account secret and then acts as the
mapped user (storage credentials, routing, replication and access policies are applied by user as usual).

* Requires `storage.dynamicCredentials` with `enabled: true` and a `masterPassword` (encryption on) in **both** the
  worker and the proxies. Secrets are sealed with the same AES-GCM key as dynamic storage credentials; without a master
  password `CreateProxyAccount` fails with `FailedPrecondition` and plaintext is never stored.
* Storage: Redis config DB hash `tutti:proxy-accounts` (field = access key id, value = JSON record with sealed
  `secret` / `previousSecret`).
* Access key id: `TUT` + 17 uppercase alphanumerics; secret: 40 random alphanumerics (crypto/rand).
* API (admin only), service `tutti.Resilience`:
  * `CreateProxyAccount{user, description}` `POST /tutti/proxy-accounts` - returns `accessKeyId` + `secretAccessKey` once.
  * `RotateProxyAccountSecret{accessKeyId, gracePeriod}` `PUT /tutti/proxy-accounts/rotate` - new secret returned once;
    the previous secret keeps working for `gracePeriod` (default 1h, `0s` = immediate).
  * `SetProxyAccountStatus{accessKeyId, status}` `PUT /tutti/proxy-accounts/status` - `active` or `disabled`.
  * `DeleteProxyAccount{accessKeyId}` `PUT /tutti/proxy-accounts/delete`.
  * `ListProxyAccounts` `GET /tutti/proxy-accounts` - dynamic accounts appear with source `dynamic`, status,
    `createdAt`, `rotatedAt` and `previousSecretExpiresAt` in `dynamicKeys`. Secrets are never returned.
* Credential precedence for an access key id: `auth.custom` > `auth.useStorage` credentials > dynamic accounts.
  A disabled dynamic account is rejected like an unknown key (`InvalidAccessKeyId`).
* Propagation: proxies cache accounts for at most 5 seconds, so create, rotate, disable and delete take effect on all
  proxies within about 5 seconds (rotation grace expiry is evaluated per request, so it is exact).
