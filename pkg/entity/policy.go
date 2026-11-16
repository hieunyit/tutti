// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entity

import (
	"errors"
	"fmt"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
)

type BucketReplicationPolicyID struct {
	User       string
	FromBucket string
}

func NewBucketReplicationPolicyID(user string, fromBucket string) BucketReplicationPolicyID {
	return BucketReplicationPolicyID{
		User:       user,
		FromBucket: fromBucket,
	}
}

type BucketReplicationPolicyDestination struct {
	ToStorage string
	ToBucket  string
}

type UserReplicationPolicy struct {
	User        string
	FromStorage string
	ToStorage   string
}

type ReplicationOptions struct {
	AgentURL    string
	EventSource dom.EventSource
	// Name is an optional display label, validated with NormalizeReplicationName.
	Name string
	// Since is the optional cutoff of the initial copy (source objects last modified before it are skipped).
	// Nil copies everything. Validated with NormalizeReplicationSince.
	Since *time.Time
}

func (p UserReplicationPolicy) LookupID() string {
	return p.User
}

func (p UserReplicationPolicy) RoutingID() string {
	return p.User
}

func (p UserReplicationPolicy) Validate() error {
	if p.FromStorage == "" {
		return fmt.Errorf("%w: from storage is required", dom.ErrInvalidArg)
	}
	if p.ToStorage == "" {
		return fmt.Errorf("%w: to storage is required", dom.ErrInvalidArg)
	}
	if p.FromStorage == p.ToStorage {
		return fmt.Errorf("%w: from/to storages should differ", dom.ErrInvalidArg)
	}
	return nil
}

func NewUserReplicationPolicy(user string, fromStorage string, toStorage string) UserReplicationPolicy {
	return UserReplicationPolicy{
		User:        user,
		FromStorage: fromStorage,
		ToStorage:   toStorage,
	}
}

type BucketRoutingPolicyID struct {
	User   string
	Bucket string
}

func NewBucketRoutingPolicyID(user string, bucket string) BucketRoutingPolicyID {
	return BucketRoutingPolicyID{
		User:   user,
		Bucket: bucket,
	}
}

type ReplicationStatus struct {
	CreatedAt time.Time `redis:"created_at"`

	ArchivedAt *time.Time `redis:"archived_at,omitempty"`

	// Options belongs to status to store in the same hash in Redis.
	// Embedded stucts are not supported by redis tag parser.
	// TODO: add other replication options here, like copy origin timestamp, etc.
	AgentURL    string `redis:"agent_url,omitempty"`
	EventSource string `redis:"event_source,omitempty"`
	// Name is an optional display label. It is not part of the replication identity.
	Name string `redis:"name,omitempty"`
	// Since is the optional cutoff of the initial copy. It is not part of the replication identity.
	Since *time.Time `redis:"since,omitempty"`

	IsArchived bool `redis:"archived"`
	// FailoverPending identifies a pause owned by an unfinished automatic failover.
	FailoverPending bool `redis:"failover_pending"`
}

func StatusFromOptions(opts ReplicationOptions) ReplicationStatus {
	// TODO: add other replication options here, like copy origin timestamp, etc.
	return ReplicationStatus{
		AgentURL:    opts.AgentURL,
		EventSource: string(opts.EventSource),
		Name:        opts.Name,
		Since:       opts.Since,
	}
}

type ReplicationStatusExtended struct {
	*ReplicationStatus

	Switch *ReplicationSwitchInfo `redis:"-"`

	// True if at least one of the queues is paused.
	IsPaused bool
	// Aggregated stats for initial migration list buckets/object tasks.
	InitMigrationListing QueueStats
	// Aggregated stats for initial migration queues.
	InitMigration QueueStats
	// Aggregated stats for event migration queues.
	EventMigration QueueStats
	// Initial migration progress by bytes (non-versioned objects only).
	InitBytes InitBytesStats
}

// InitBytesStats is initial replication progress measured in bytes.
type InitBytesStats struct {
	// Total is the sum of sizes of objects enumerated by the listing so far. A lower bound until ListingComplete.
	Total int64
	// Done is the sum of sizes of objects copied successfully.
	Done int64
	// BytesPerSecond is completion-based throughput over a rolling window, 0 if idle/unknown.
	BytesPerSecond int64
	// ListingComplete is true when the listing of the source bucket finished, so Total is final.
	ListingComplete bool
}

// ETA estimates the time left for the initial copy. ok is false when it cannot be estimated
// (listing still running, no throughput yet, or nothing known).
func (b InitBytesStats) ETA() (eta time.Duration, ok bool) {
	if !b.ListingComplete || b.BytesPerSecond <= 0 || b.Total <= 0 {
		return 0, false
	}
	remaining := b.Total - b.Done
	if remaining <= 0 {
		return 0, true
	}
	return time.Duration(remaining/b.BytesPerSecond) * time.Second, true
}

func (r *ReplicationStatusExtended) InitDone() bool {
	// all listing tasks done, all listed objects processed, and nothing permanently failed.
	// Archived tasks keep this false by contract (see TestBulkLifecycle_FalseDoneWhenArchived);
	// operators must clear them via the diff-fix / retry flow.
	return r.InitMigrationListing.Unprocessed == 0 &&
		r.InitMigration.Unprocessed == 0 &&
		r.InitMigrationListing.Archived == 0 &&
		r.InitMigration.Archived == 0
}

func (r *ReplicationStatusExtended) HasFailures() bool {
	return r.InitMigration.Archived > 0 || r.InitMigrationListing.Archived > 0 || r.EventMigration.Archived > 0
}

func (r *ReplicationStatusExtended) Latency() time.Duration {
	if r.InitDone() {
		return r.EventMigration.Latency
	}
	return r.InitMigration.Latency
}

type QueueStats struct {
	// Number of tasks left. Includes, new, in_progress, and retried tasks.
	Unprocessed int
	// Total number of successfully processed tasks.
	Done int
	// Number of permanently failed (archived) tasks.
	Archived int
	// Total failed task attempts (retries + archived).
	Failed int
	// Age of the oldest pending task in the queue.
	Latency time.Duration
	// Approx bytes used by the queue and its tasks in Redis.
	MemoryUsage int64
}

type BucketReplicationPolicy struct {
	User        string
	FromStorage string
	FromBucket  string
	ToStorage   string
	ToBucket    string
}

func (p BucketReplicationPolicy) LookupID() BucketReplicationPolicyID {
	return BucketReplicationPolicyID{
		User:       p.User,
		FromBucket: p.FromBucket,
	}
}

func (p BucketReplicationPolicy) Destination() BucketReplicationPolicyDestination {
	return BucketReplicationPolicyDestination{
		ToStorage: p.ToStorage,
		ToBucket:  p.ToBucket,
	}
}

func (p BucketReplicationPolicy) RoutingID() BucketRoutingPolicyID {
	return BucketRoutingPolicyID{
		User:   p.User,
		Bucket: p.FromBucket,
	}
}

func (p BucketReplicationPolicy) Validate() error {
	errs := make([]error, 0)
	if p.FromStorage == "" {
		err := fmt.Errorf("%w: from storage is required", dom.ErrInvalidArg)
		errs = append(errs, err)
	}
	if p.FromBucket == "" {
		err := fmt.Errorf("%w: from bucket is required", dom.ErrInvalidArg)
		errs = append(errs, err)
	}
	if p.ToStorage == "" {
		err := fmt.Errorf("%w: to storage is required", dom.ErrInvalidArg)
		errs = append(errs, err)
	}
	if p.ToBucket == "" {
		err := fmt.Errorf("%w: to bucket is required", dom.ErrInvalidArg)
		errs = append(errs, err)
	}
	if p.FromStorage == p.ToStorage && p.FromBucket == p.ToBucket {
		err := fmt.Errorf("%w: from/to storages and/or buckets should differ", dom.ErrInvalidArg)
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}
	return nil
}

func NewBucketReplicationPolicy(user string, fromStorage string, fromBucket string, toStorage string, toBucket string) BucketReplicationPolicy {
	return BucketReplicationPolicy{
		User:        user,
		FromStorage: fromStorage,
		FromBucket:  fromBucket,
		ToStorage:   toStorage,
		ToBucket:    toBucket,
	}
}
