# Upstream branches

Branches on the `upstream` remote that are not part of Tutti's main line. Read the status before merging or cherry-picking anything.

| Branch | Status | Notes |
|--------|--------|-------|
| `upstream/feat/proxy-alias-credentials` | **DO NOT MERGE** | Argon2 parallelism is derived from `runtime.NumCPU`. Nodes with different CPU counts derive different keys, so credentials encrypted on one node fail to decrypt on another. Above 255 CPUs it panics. Superseded by connection accounts and proxy dynamic accounts. |
| `upstream/consistency-check` | **DO NOT MERGE** | Draft. Known defects: `KEYS` used inside a Lua script, wrong-case `keys`/`ARGS` in Lua, a goroutine leak when leaving a minio `ListObjects` range early, and a wrong error variable (`object.Err`). |
| `upstream/fix-switch-multipart-upload-key` | Review before merging | Touches read routing during the zero-downtime switch. Verify against Tutti's routing snapshot before taking it. |
| `upstream/61-expose-version-in-management-api` | Cherry-pick only | Contains the consistency-check code. Take only the version API commits if wanted. |
| `upstream/bucket-sharding-design-doc` | Design reference | Design document only. |
| `upstream/fast-ceph-container` | Reference | Container setup for Ceph. |
| `upstream/ipv6-test` | Safe to evaluate | Test and docs changes only. |

See [Operations](./operations.md) for the operational guidance that applies when evaluating these branches.
