// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package failover

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

// ScopeSource is the subset of the policy service used to discover failover scopes.
type ScopeSource interface {
	GetUserRoutings(ctx context.Context) (map[string]string, error)
	GetBucketRoutings(ctx context.Context, user string) (map[string]string, error)
	ListUserReplicationsInfo(ctx context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error)
	ListBucketReplicationsInfo(ctx context.Context, user string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error)
}

// Candidate is one replication of a scope that may receive the scope on a failover.
type Candidate struct {
	ID        entity.UniversalReplicationID
	To        string
	Status    entity.ReplicationStatusExtended
	ToBucket  string // destination bucket (bucket scope)
	SrcBucket string
}

// Scope is a user-level or bucket-level routing target that has a replication.
type Scope struct {
	User, Bucket string // Bucket is empty for a user scope
	From         string
	Candidates   []Candidate
}

// Key identifies the scope.
func (s Scope) Key() string { return s.User + "/" + s.Bucket }

// DiscoverOptions parameterize DiscoverScopes.
type DiscoverOptions struct {
	// MainStorage is the implicit routing target of users without explicit routing.
	MainStorage string
	// Users are additional users to inspect for bucket-level replications (in addition to the users of user-level
	// replications and routings).
	Users []string
	// Skip, when set, excludes the scopes for which it returns true (e.g. group-owned scopes).
	Skip func(ctx context.Context, user, bucket string) bool
}

// DiscoverScopes returns the scopes currently routed to storage which have replications from it. It is the single
// definition of "the scopes the legacy failover controller acts on", shared by the controller and by the migration
// to protection groups.
func DiscoverScopes(ctx context.Context, src ScopeSource, storage string, o DiscoverOptions) ([]Scope, error) {
	userRoutes, err := src.GetUserRoutings(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("get user routings: %w", err)
	}
	userRepls, err := src.ListUserReplicationsInfo(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("list user replications: %w", err)
	}
	effective := func(user string) string {
		if to, ok := userRoutes[user]; ok {
			return to
		}
		return o.MainStorage
	}
	skip := func(user, bucket string) bool { return o.Skip != nil && o.Skip(ctx, user, bucket) }
	users := map[string]struct{}{}
	byUser := map[string]*Scope{}
	for pol, st := range userRepls {
		users[pol.User] = struct{}{}
		if pol.FromStorage != storage || effective(pol.User) != storage {
			continue
		}
		if skip(pol.User, "") {
			continue
		}
		s := byUser[pol.User]
		if s == nil {
			s = &Scope{User: pol.User, From: storage}
			byUser[pol.User] = s
		}
		s.Candidates = append(s.Candidates, Candidate{ID: entity.UniversalFromUserReplication(pol), To: pol.ToStorage, Status: st})
	}
	for u := range userRoutes {
		users[u] = struct{}{}
	}
	for _, u := range o.Users {
		users[u] = struct{}{}
	}
	var res []Scope
	for _, s := range byUser {
		res = append(res, *s)
	}
	for user := range users {
		if _, isUserScope := byUser[user]; isUserScope {
			continue // bucket-level and user-level replications are mutually exclusive
		}
		bRepls, err := src.ListBucketReplicationsInfo(ctx, user)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, fmt.Errorf("list bucket replications of %s: %w", user, err)
		}
		if len(bRepls) == 0 {
			continue
		}
		bRoutes, err := src.GetBucketRoutings(ctx, user)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, fmt.Errorf("get bucket routings of %s: %w", user, err)
		}
		byBucket := map[string]*Scope{}
		for pol, st := range bRepls {
			routed := effective(user)
			if to, ok := bRoutes[pol.FromBucket]; ok {
				routed = to
			}
			if pol.FromStorage != storage || routed != storage {
				continue
			}
			if skip(user, pol.FromBucket) {
				continue
			}
			s := byBucket[pol.FromBucket]
			if s == nil {
				s = &Scope{User: user, Bucket: pol.FromBucket, From: storage}
				byBucket[pol.FromBucket] = s
			}
			s.Candidates = append(s.Candidates, Candidate{
				ID: entity.UniversalFromBucketReplication(pol), To: pol.ToStorage, Status: st,
				ToBucket: pol.ToBucket, SrcBucket: pol.FromBucket,
			})
		}
		for _, s := range byBucket {
			res = append(res, *s)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Key() < res[j].Key() })
	return res, nil
}
