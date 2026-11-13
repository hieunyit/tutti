// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package policy

import (
	"context"
	"errors"
	"strings"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/replscope"
	"github.com/hieunyit/tutti/pkg/store"
)

// ScopeSvc lists the owners of replication scope for the shared conflict rule (replscope).
type ScopeSvc interface {
	ListScopeOwners(ctx context.Context) ([]replscope.Owner, error)
}

var _ ScopeSvc = (*policySvc)(nil)

// bucketStatusIndexPrefix is the key prefix of the per-user bucket replication status index
// (see store.NewBucketReplicationStatusStore).
const bucketStatusIndexPrefix = "p:repl_bucket_status_index" + store.CKeyPartsDelimiter

// ListScopeOwners returns one owner per protection group (every ordered member pair, its buckets) and one per
// non-archived user-level or bucket-level replication of any user, including storage-level ones (empty user).
// Replications a group manages are represented by the group only. Bucket replications cover their source and
// destination bucket names.
func (r *policySvc) ListScopeOwners(ctx context.Context) ([]replscope.Owner, error) {
	groups, err := protectiongroups.NewStore(r.client).List(ctx)
	if err != nil {
		return nil, err
	}
	managed := map[string]bool{}
	for _, g := range groups {
		for _, m := range g.Members {
			for _, id := range g.Replications(m) {
				managed[id.AsString()] = true
			}
		}
	}
	owners := protectiongroups.GroupOwners(groups)

	ids, err := r.allReplicationStatuses(ctx)
	if err != nil {
		return nil, err
	}
	for id, archived := range ids {
		key := id.AsString()
		if archived || managed[key] {
			continue
		}
		var buckets []string
		if b, ok := id.AsBucketID(); ok {
			buckets = []string{b.FromBucket}
			if b.ToBucket != b.FromBucket {
				buckets = append(buckets, b.ToBucket)
			}
		}
		owners = append(owners, replscope.Owner{Kind: "replication", ID: key, Scope: replscope.ReplicationScope(id.FromStorage(), id.ToStorage(), buckets).WithUser(id.User())})
	}
	return owners, nil
}

// allReplicationStatuses returns every replication of every user (user-level and bucket-level) with its archived
// flag. It reads statuses only (no queue statistics), so it is cheap enough to run under the scope lock.
func (r *policySvc) allReplicationStatuses(ctx context.Context) (map[entity.UniversalReplicationID]bool, error) {
	out := map[entity.UniversalReplicationID]bool{}
	if r.cp != nil {
		statuses, err := r.cp.Repos().Replications.ListStatuses(ctx, "")
		if err == nil {
			for id, st := range statuses {
				if id.IsUserLevel() {
					out[entity.UniversalFromUserReplication(entity.UserReplicationPolicy{User: id.User, FromStorage: id.FromStorage, ToStorage: id.ToStorage})] = st.Archived
				} else {
					out[entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: id.User, FromStorage: id.FromStorage, ToStorage: id.ToStorage, FromBucket: id.FromBucket, ToBucket: id.ToBucket})] = st.Archived
				}
			}
			return out, nil
		}
		if !controlplane.IsUnavailable(err) {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("replication: PostgreSQL ListStatuses failed, falling back to Redis")
	}
	users, err := r.userReplicationStatusStore.List(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, err
	}
	for p, st := range users {
		out[entity.UniversalFromUserReplication(p)] = st.IsArchived
	}
	iter := r.client.Scan(ctx, 0, bucketStatusIndexPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		user := strings.TrimPrefix(iter.Val(), bucketStatusIndexPrefix)
		buckets, err := r.bucketReplicationStatusStore.List(ctx, user)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		for p, st := range buckets {
			out[entity.UniversalFromBucketReplication(p)] = st.IsArchived
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
