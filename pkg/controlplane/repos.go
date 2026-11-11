// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"time"
)

// KV is the repository shape shared by the families that Redis keeps as a HASH of JSON documents (field = identity).
// All errors wrap dom errors: dom.ErrNotFound, dom.ErrAlreadyExists, dom.ErrInvalidArg, ErrVersionConflict.
//
// Mapping to the Redis stores: Create = HSETNX / "if hexists then 0 else hset" script, Put = HSET, Update = the Lua
// compare-and-set (luaCAS, consoleMutation, WATCH) with the version column instead of the old value, Mutate = the
// read-modify-write retry loops (the row is locked, so there are no retries), Delete = HDEL (dom.ErrNotFound if
// absent), Get = HGET, List = HGETALL sorted by key.
type KV[T any] interface {
	Create(ctx context.Context, v T) (Versioned[T], error)
	Get(ctx context.Context, key string) (Versioned[T], error)
	List(ctx context.Context) ([]Versioned[T], error)
	// Put creates or replaces the record without a version check.
	Put(ctx context.Context, v T) (Versioned[T], error)
	// Update replaces the record if its stored version equals expectedVersion, else ErrVersionConflict.
	Update(ctx context.Context, v T, expectedVersion int64) (Versioned[T], error)
	// Mutate applies fn to the locked current record and stores the result; fn's error aborts without changes.
	Mutate(ctx context.Context, key string, fn func(*T) error) (Versioned[T], error)
	Delete(ctx context.Context, key string) error
	// Fingerprint is a cheap digest of the table (keys and versions, no documents): it changes whenever a row is
	// created, updated or deleted. Pollers compare it to skip a full List when nothing changed.
	Fingerprint(ctx context.Context) (string, error)
}

// Storages: runtime-managed storages (pkg/objstore managed.go). Key: storage name.
type Storages interface{ KV[StorageRecord] }

// S3Identities: Tutti user records (pkg/objstore connection.go AddIdentity/RemoveIdentity/Identities). Key: user.
type S3Identities interface{ KV[S3Identity] }

// ProxyAccounts: proxy access keys (pkg/proxyaccount.Store). Key: access key id. Create is Store.Create without the
// random id and secret generation (those stay in the service), Mutate is Rotate/SetStatus.
type ProxyAccounts interface{ KV[ProxyAccountRecord] }

// AccessPolicies: pkg/accesspolicy.Store (Set = Put). Key: user.
type AccessPolicies interface{ KV[AccessPolicy] }

// FailoverPolicies: pkg/failover.Store policies (Set/Get/List/DeleteFailoverPolicy). Key: source storage.
type FailoverPolicies interface{ KV[FailoverPolicy] }

// FailoverEvents: pkg/failover.Store events (Save = Put, List, Delete). Key: FailoverEventKey(user, bucket).
type FailoverEvents interface{ KV[FailoverEvent] }

// ProtectionGroups: pkg/protectiongroups.Store group documents (Save/Get/List/Delete). Key: group id.
// The scope index, writers, leases and the freeze flag are runtime state and stay in Redis.
type ProtectionGroups interface{ KV[RawDoc] }

// StorageTests: last connection test result of a storage (SaveStorageTest/StorageTests). Key: storage name.
type StorageTests interface{ KV[RawDoc] }

// ConsoleUserGuard configures the "keep one enabled admin" check of ConsoleUsers.MutateGuarded (pkg/auth
// consoleMutation).
type ConsoleUserGuard struct {
	// ConfigAdmins is the number of enabled admins defined in the static config.
	ConfigAdmins int
	// ConfigNames are config user names that shadow managed users of the same name (not counted).
	ConfigNames []string
}

// ConsoleUsers: pkg/auth.UserStore. Key: username.
type ConsoleUsers interface {
	KV[ConsoleUser]
	// MutateGuarded is Mutate with the last-admin check. fn may ask to delete the user (remove) and to verify that
	// another enabled admin remains (checkOtherAdmin); if none does the change is rejected with
	// dom.ErrFailedPrecondition. Concurrent guarded mutations are serialized.
	MutateGuarded(ctx context.Context, username string, guard ConsoleUserGuard, fn func(u *ConsoleUser) (remove, checkOtherAdmin bool, err error)) error
	// TouchLogin records the last login time. Best effort: no audit, no version change, never creates users.
	TouchLogin(ctx context.Context, username string, at time.Time) error
}

// DynamicCredentials: pkg/objstore dynamic credentials (Redis HASH chorus:dynamic_creds).
type DynamicCredentials interface {
	// Set stores the sealed credential (HSET + version bump).
	Set(ctx context.Context, c DynamicCredential) error
	Get(ctx context.Context, storageType, storage, user string) (DynamicCredential, error)
	// Delete removes the credentials of the user on the storage for both storage types; absent is not an error.
	Delete(ctx context.Context, storage, user string) error
	List(ctx context.Context) ([]DynamicCredential, error)
	// Fingerprint is a cheap digest of the credential rows (keys and versions, no secrets); see KV.Fingerprint.
	Fingerprint(ctx context.Context) (string, error)
	// Salt returns the key derivation salt ("<parallelism>:<base64>"), false if not set yet.
	Salt(ctx context.Context) (string, bool, error)
	// SetSaltIfAbsent stores the salt (HSETNX) and reports whether it was stored.
	SetSaltIfAbsent(ctx context.Context, salt string) (bool, error)
}

// Routing: pkg/store routing stores. An empty bucket means user-level routing.
// Get reports dom.ErrRoutingBlock if the scope is blocked and dom.ErrNotFound if there is no route.
type Routing interface {
	Get(ctx context.Context, user, bucket string) (string, error)
	// Set creates or replaces the route (HSET).
	Set(ctx context.Context, user, bucket, storage string) error
	// Update changes an existing route, dom.ErrNotFound if there is none (luaUpdateRoute).
	Update(ctx context.Context, user, bucket, storage string) error
	// DeleteRoute removes the route of the scope.
	DeleteRoute(ctx context.Context, user, bucket string) error
	// ListUserRoutes returns user->storage of the user-level routes, ListBucketRoutes bucket->storage of one user.
	ListUserRoutes(ctx context.Context) (map[string]string, error)
	ListBucketRoutes(ctx context.Context, user string) (map[string]string, error)
	Block(ctx context.Context, user, bucket string) error
	Unblock(ctx context.Context, user, bucket string) error
	ListUserBlocks(ctx context.Context) (map[string]bool, error)
	ListBucketBlocks(ctx context.Context, user string) (map[string]bool, error)
	// Export returns every route and block of every scope (verification and migration).
	Export(ctx context.Context) (RoutingExport, error)
	// DeleteAllUser removes the user-level route and block (UserRoutingStore.DeleteAllOp).
	DeleteAllUser(ctx context.Context, user string) error
}

// RouteEntry is one route (Storage set) or block (Storage empty); Bucket is empty for user-level scope.
type RouteEntry struct {
	User    string
	Bucket  string
	Storage string
}

// RoutingExport is the full routing state.
type RoutingExport struct {
	Routes []RouteEntry
	Blocks []RouteEntry
}

// Replications: pkg/store replication policy, status and generation stores in one repository, because the Redis
// layout couples them (lookup keys and generations are read in one transaction by the proxy).
type Replications interface {
	// Policies returns the active destinations of the scope (user-level when fromBucket is empty), sorted by
	// destination. dom.ErrNotFound if none.
	Policies(ctx context.Context, user, fromBucket string) ([]ReplicationID, error)
	// Add creates the policy (idempotent: SADD) and its status row (CreatedAt = now if zero, replaces an archived
	// status). The service validates before calling, as today. A source storage that differs from the source of the
	// other destinations of the scope is dom.ErrInvalidArg.
	Add(ctx context.Context, id ReplicationID, status ReplicationStatus) error
	// Remove deletes the policy but keeps its status row (RemoveOp); call Archive to mark the status archived.
	Remove(ctx context.Context, id ReplicationID) error
	// PutStatus writes only the status row (an archived status of a removed policy, migration).
	PutStatus(ctx context.Context, id ReplicationID, status ReplicationStatus) error
	// Archive marks the status archived now (ArchieveOp).
	Archive(ctx context.Context, id ReplicationID) error
	// DeleteStatus drops the status row (DeleteOp).
	DeleteStatus(ctx context.Context, id ReplicationID) error
	GetStatus(ctx context.Context, id ReplicationID) (ReplicationStatus, error)
	// ListStatuses returns all statuses; user filters the bucket-level ones and user-level ones when non-empty.
	ListStatuses(ctx context.Context, user string) (map[ReplicationID]ReplicationStatus, error)
	// BucketReplicationExistsForUser reports whether the user has a bucket-level replication.
	BucketReplicationExistsForUser(ctx context.Context, user string) (bool, error)
	// IsDestinationInUse reports whether (storage, bucket) is a destination of a bucket replication of the user.
	IsDestinationInUse(ctx context.Context, user, toStorage, toBucket string) (bool, error)
	Generation(ctx context.Context, id ReplicationID) (int64, error)
	// IncrementGeneration is the fencing token bump, done by the service on replication create and delete.
	IncrementGeneration(ctx context.Context, id ReplicationID) (int64, error)
	// SetGeneration is used by the migration only.
	SetGeneration(ctx context.Context, id ReplicationID, generation int64) error
	// ListAll returns every active policy (for verification and export).
	ListAll(ctx context.Context) ([]ReplicationID, error)
	// ListGenerations returns every stored generation.
	ListGenerations(ctx context.Context) (map[ReplicationID]int64, error)
}

// Audit reads the audit log. Rows are written by the repositories, or via Record for runtime operations.
type Audit interface {
	List(ctx context.Context, f AuditFilter) ([]AuditEntry, error)
	Record(ctx context.Context, action, objectType, objectID string, details any) error
}

// Repos groups all repositories. Inside Store.Tx they share one transaction.
type Repos struct {
	Storages           Storages
	S3Identities       S3Identities
	DynamicCredentials DynamicCredentials
	ProxyAccounts      ProxyAccounts
	AccessPolicies     AccessPolicies
	ConsoleUsers       ConsoleUsers
	Routing            Routing
	Replications       Replications
	FailoverPolicies   FailoverPolicies
	FailoverEvents     FailoverEvents
	ProtectionGroups   ProtectionGroups
	StorageTests       StorageTests
	Audit              Audit
}

// Store is a control-plane database.
type Store interface {
	Repos() Repos
	// Tx runs fn in one database transaction: all mutations, audit rows and outbox entries commit or roll back
	// together. Do not call Tx from inside fn.
	Tx(ctx context.Context, fn func(Repos) error) error
	Close()
}
