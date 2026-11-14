// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

// The record types below mirror the JSON documents the Redis stores write today, field for field (same json tags),
// so that documents can be copied between the stores without conversion and projected back to Redis byte-identical.
// They are duplicated here instead of imported to keep this package free of dependencies on the packages that will
// later depend on it. Tests in pkg/controlplane/migrate compare them with the real stores.

// Versioned wraps a record with the row metadata of the PostgreSQL store.
type Versioned[T any] struct {
	Value T
	// Version starts at 1 and is incremented by every change; pass it to Update for optimistic concurrency.
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StorageRecord is a runtime-managed storage (Redis HASH tutti:storages, pkg/objstore.managedDoc). The credentials are
// sealed with the dynamic credentials master key and are never opened by this package.
type StorageRecord struct {
	Name                string    `json:"name"`
	Address             string    `json:"address"`
	Provider            string    `json:"provider"`
	DefaultRegion       string    `json:"defaultRegion,omitempty"`
	IsSecure            bool      `json:"isSecure"`
	HealthCheckInterval int64     `json:"healthCheckIntervalNs,omitempty"`
	HttpTimeout         int64     `json:"httpTimeoutNs,omitempty"`
	RateEnabled         bool      `json:"rateLimitEnabled,omitempty"`
	RateRPM             int       `json:"rateLimitRpm,omitempty"`
	RateMetadata        bool      `json:"rateLimitIncludeMetadataApi,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
	// ConnectionAccount and PreviousAccount are sealed JSON of s3.CredentialsV4.
	ConnectionAccount string    `json:"connectionAccount,omitempty"`
	PreviousAccount   string    `json:"previousAccount,omitempty"`
	RotatedAt         time.Time `json:"rotatedAt,omitzero"`
	LastForcedSaveBy  string    `json:"lastForcedSaveBy,omitempty"`
	LastForcedSaveAt  time.Time `json:"lastForcedSaveAt,omitzero"`
}

// S3Identity is a Tutti user record (Redis HASH tutti:s3users, field = name).
type S3Identity struct {
	Name      string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

// DynamicCredential is a sealed per-user storage credential (Redis HASH tutti:dynamic_creds, field
// "<S3|SWIFT>:<storage>:<user>"). Sealed is stored exactly as in Redis (AES-GCM base64, or plain JSON if encryption
// is disabled in the config).
type DynamicCredential struct {
	StorageType string
	Storage     string
	User        string
	Sealed      string
}

// RedisField is the field name in the Redis hash.
func (c DynamicCredential) RedisField() string { return c.StorageType + ":" + c.Storage + ":" + c.User }

// ProxyAccountRecord is a proxy access key (Redis HASH tutti:proxy-accounts, field = access key id). Secret and
// PreviousSecret are sealed.
type ProxyAccountRecord struct {
	AccessKeyID       string    `json:"-"`
	User              string    `json:"user"`
	Description       string    `json:"description,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	RotatedAt         time.Time `json:"rotatedAt,omitempty"`
	Secret            string    `json:"secret"`
	PreviousSecret    string    `json:"previousSecret,omitempty"`
	PreviousExpiresAt time.Time `json:"previousExpiresAt,omitempty"`
}

// AccessPolicy restricts one proxy user (Redis HASH tutti:access-policies, field = user).
type AccessPolicy struct {
	User        string   `json:"user"`
	Permission  string   `json:"permission"`
	Buckets     []string `json:"buckets"`
	Description string   `json:"description,omitempty"`
}

// ConsoleUser is a managed console user (Redis HASH tutti:console-users). LastLoginAt is not stored in the document:
// it lives in its own column (Redis: tutti:console-users:last-login) and is merged on reads.
type ConsoleUser struct {
	Username           string    `json:"username"`
	Role               string    `json:"role"`
	PasswordHash       string    `json:"passwordHash"`
	Disabled           bool      `json:"disabled"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	LastLoginAt        time.Time `json:"lastLoginAt,omitempty"`
	PasswordChangedAt  time.Time `json:"passwordChangedAt"`
	MustChangePassword bool      `json:"mustChangePassword"`
}

// FailoverPolicy is the failover policy of one source storage (Redis HASH tutti:failover-policies).
type FailoverPolicy struct {
	Source      string        `json:"source"`
	Enabled     bool          `json:"enabled"`
	Targets     []string      `json:"targets"`
	After       time.Duration `json:"after"`
	MaxEventLag *int          `json:"maxEventLag,omitempty"`
}

// FailoverEvent is one performed failover (Redis STRING tutti:failover:user:<u> / tutti:failover:bucket:<u>:<b>).
type FailoverEvent struct {
	User     string    `json:"user"`
	Bucket   string    `json:"bucket,omitempty"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	Time     time.Time `json:"time"`
	EventLag int       `json:"eventLag"`
}

// Key is the Redis key suffix after "tutti:failover:".
func (e FailoverEvent) Key() string { return FailoverEventKey(e.User, e.Bucket) }

// FailoverEventKey builds the event key for a scope (bucket empty for user-level).
func FailoverEventKey(user, bucket string) string {
	if bucket == "" {
		return "user:" + user
	}
	return "bucket:" + user + ":" + bucket
}

// RawDoc is an opaque JSON document with an identity, used for families whose Go type lives in a package that
// must not be imported here (protection groups) or is opaque to the store (storage connection tests).
type RawDoc struct {
	ID   string
	User string
	JSON json.RawMessage
}

// MarshalJSON returns the document unchanged.
func (d RawDoc) MarshalJSON() ([]byte, error) {
	if len(d.JSON) == 0 {
		return []byte("null"), nil
	}
	return d.JSON, nil
}

// UnmarshalJSON captures the document and extracts "id" and "user" when present.
func (d *RawDoc) UnmarshalJSON(b []byte) error {
	d.JSON = append(d.JSON[:0], bytes.TrimSpace(b)...)
	var head struct {
		ID   string `json:"id"`
		User string `json:"user"`
	}
	_ = json.Unmarshal(b, &head)
	if d.ID == "" {
		d.ID = head.ID
	}
	if d.User == "" {
		d.User = head.User
	}
	return nil
}

// ReplicationID identifies one replication. FromBucket and ToBucket are empty for a user-level replication.
type ReplicationID struct {
	User        string
	FromStorage string
	ToStorage   string
	FromBucket  string
	ToBucket    string
}

// IsUserLevel reports whether the replication covers all buckets of the user.
func (r ReplicationID) IsUserLevel() bool { return r.FromBucket == "" }

// String is the Redis status/universal ID: user:from:to[:fromBucket:toBucket].
func (r ReplicationID) String() string {
	if r.IsUserLevel() {
		return strings.Join([]string{r.User, r.FromStorage, r.ToStorage}, ":")
	}
	return strings.Join([]string{r.User, r.FromStorage, r.ToStorage, r.FromBucket, r.ToBucket}, ":")
}

// ParseReplicationID parses String().
func ParseReplicationID(s string) (ReplicationID, bool) {
	p := strings.Split(s, ":")
	switch len(p) {
	case 3:
		return ReplicationID{User: p[0], FromStorage: p[1], ToStorage: p[2]}, p[0] != "" && p[1] != "" && p[2] != ""
	case 5:
		return ReplicationID{User: p[0], FromStorage: p[1], ToStorage: p[2], FromBucket: p[3], ToBucket: p[4]}, p[0] != "" && p[1] != "" && p[2] != "" && p[3] != "" && p[4] != ""
	}
	return ReplicationID{}, false
}

// ReplicationStatus holds the CONFIG part of the Redis replication status hash (p:repl_status_*). The runtime part
// (failover_pending, queue stats, pause state) stays in Redis.
type ReplicationStatus struct {
	CreatedAt   time.Time
	Archived    bool
	ArchivedAt  *time.Time
	AgentURL    string
	EventSource string
}

// AuditEntry is one row of the audit log.
type AuditEntry struct {
	ID         int64
	At         time.Time
	Actor      string
	RequestID  string
	Action     string // create, update, delete, set, ...
	ObjectType string
	ObjectID   string
	// Before and After are the JSON documents with secrets redacted ("***"). nil when absent.
	Before json.RawMessage
	After  json.RawMessage
}

// AuditFilter narrows Audit.List. Zero values match everything; results are newest first.
type AuditFilter struct {
	ObjectType string
	ObjectID   string
	Actor      string
	// Action matches the action case-insensitively (applied in SQL, so Limit and BeforeID paginate the filtered rows).
	Action string
	Since  time.Time
	Until  time.Time
	// BeforeID returns only entries with a smaller ID (pagination).
	BeforeID int64
	Limit    int
}

// SystemSettings holds dynamic cluster features, worker performance tuning, and operational parameters.
type SystemSettings struct {
	ID       string                 `json:"-"`
	Features SystemFeaturesSettings `json:"features"`
	Worker   SystemWorkerSettings   `json:"worker"`
	Lock     SystemLockSettings     `json:"lock"`
	Webhook  SystemWebhookSettings  `json:"webhook"`
}

type SystemFeaturesSettings struct {
	Tagging           bool `json:"tagging"`
	ACL               bool `json:"acl"`
	Lifecycle         bool `json:"lifecycle"`
	Policy            bool `json:"policy"`
	PreserveACLGrants bool `json:"preserveACLGrants"`
	Versioning        bool `json:"versioning"`
}

type SystemWorkerSettings struct {
	Concurrency              int           `json:"concurrency"`
	ShutdownTimeout          time.Duration `json:"shutdownTimeout"`
	QueueUpdateInterval      time.Duration `json:"queueUpdateInterval"`
	TaskCheckInterval        time.Duration `json:"taskCheckInterval"`
	DelayedTaskCheckInterval time.Duration `json:"delayedTaskCheckInterval"`
	SwiftRetryInterval       time.Duration `json:"swiftRetryInterval"`
	PauseRetryInterval       time.Duration `json:"pauseRetryInterval"`
	SwitchRetryInterval      time.Duration `json:"switchRetryInterval"`
	BacklogHighWatermark     int           `json:"backlogHighWatermark"`
	BacklogLowWatermark      int           `json:"backlogLowWatermark"`
	BandwidthLimitBytes      int64         `json:"bandwidthLimitBytes"`
	BufferBudgetBytes        int64         `json:"bufferBudgetBytes"`
	// BulkConcurrencyPercent is the share (5..95, 0 = default 20) of Concurrency given to bulk migration.
	BulkConcurrencyPercent int `json:"bulkConcurrencyPercent,omitempty"`
	// MultipartThreads is the number of parallel parts for objects above the multipart threshold (1..16, 0 = 1).
	MultipartThreads int `json:"multipartThreads,omitempty"`
	// MultipartPartSizeMiB is the multipart part size in MiB (min 5, 0 = 16).
	MultipartPartSizeMiB int `json:"multipartPartSizeMiB,omitempty"`
}

type SystemLockSettings struct {
	Overlap time.Duration `json:"overlap"`
}

type SystemWebhookSettings struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"baseUrl"`
}
