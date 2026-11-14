// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

// Redis keys of the existing stores. The proxies and workers keep reading these keys, so the outbox publisher
// writes the committed PostgreSQL state into them (pkg/controlplane/migrate tests compare the result with data
// written by the real Redis stores).
const (
	KeyStorages            = "tutti:storages" // pkg/objstore managedStoragesKey
	KeyStoragesVersion     = "tutti:storages:version"
	KeyS3Users             = "tutti:s3users" // pkg/objstore identitiesKey
	KeyS3UsersVersion      = "tutti:s3users:version"
	KeyStorageTests        = "tutti:storage-tests"
	KeyDynamicCreds        = "tutti:dynamic_creds" // pkg/objstore redisHashKey
	KeyProxyAccounts       = "tutti:proxy-accounts"
	KeyAccessPolicies      = "tutti:access-policies"
	KeyConsoleUsers        = "tutti:console-users"
	KeyConsoleLogins       = "tutti:console-users:last-login"
	KeyFailoverPolicies    = "tutti:failover-policies"
	KeyFailoverEventPrefix = "tutti:failover:"
	KeyProtectionGroups    = "tutti:protection:groups"
	KeySystemSettings      = "tutti:system-settings"
	KeySystemSettingsVersion = "tutti:system-settings:version"
	SystemSettingsChannel  = "tutti:system-settings:changed"

	// SnapshotVersionKey and SnapshotChannel are pkg/policy.SnapshotVersionKey / SnapshotChannel.
	SnapshotVersionKey = "tutti:cfg:version"
	SnapshotChannel    = "tutti:cfg:changed"
)

// Op names of RedisOp. Data ops are idempotent (absolute state); the bump ops are at-least-once counters, a repeated
// bump only makes the pollers reload once more.
const (
	OpHSet     = "hset"     // HSET key field value
	OpHSetNX   = "hsetnx"   // HSETNX key field value
	OpHDel     = "hdel"     // HDEL key field
	OpHMSet    = "hmset"    // HSET key fields...; HDEL key del...  (other fields of the hash are untouched)
	OpHReplace = "hreplace" // DEL key; HSET key fields... (atomic in MULTI)
	OpSet      = "set"      // SET key value
	OpDel      = "del"      // DEL key
	OpSAdd     = "sadd"     // SADD key members...
	OpSRem     = "srem"     // SREM key members...
	OpSReplace = "sreplace" // DEL key; SADD key members...
	OpIncr     = "incr"     // INCR key (version counters)
	OpHIncr    = "hincr"    // HINCRBY key field 1 (version field of the dynamic credentials hash)
	OpSnapshot = "snapshot" // INCR tutti:cfg:version; PUBLISH tutti:cfg:changed <version> (proxy snapshot)
	OpPublish  = "publish"  // PUBLISH key value
)

// RedisOp is one projection step stored in an outbox entry.
type RedisOp struct {
	Op      string            `json:"op"`
	Key     string            `json:"key,omitempty"`
	Field   string            `json:"field,omitempty"`
	Value   string            `json:"value,omitempty"`
	Members []string          `json:"members,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
	Del     []string          `json:"del,omitempty"`
}

func opHSet(key, field string, value []byte) RedisOp {
	return RedisOp{Op: OpHSet, Key: key, Field: field, Value: string(value)}
}
func opHDel(key, field string) RedisOp { return RedisOp{Op: OpHDel, Key: key, Field: field} }
func opIncr(key string) RedisOp        { return RedisOp{Op: OpIncr, Key: key} }
func opSnapshot() RedisOp              { return RedisOp{Op: OpSnapshot} }
