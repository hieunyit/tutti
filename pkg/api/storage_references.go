// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/s3users"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	referenceScanBatch    = 200
	bucketReplIndexPrefix = "p:repl_bucket_status_index:"
	bucketRoutePrefix     = "p:route_bucket:"
)

// StorageReferencesFromRepos checks all control-plane repositories (plus optional metaRedis diff reports) for
// references to storage name. Can be called inside a controlplane.Tx for atomic reference check + delete.
func StorageReferencesFromRepos(ctx context.Context, repos controlplane.Repos, metaRedis redis.Cmdable, name string) ([]string, error) {
	refs, err := storageReferenceIndexFromRepos(ctx, repos, metaRedis)
	if err != nil {
		return nil, err
	}
	return refs[name], nil
}

type storageReferenceIndex map[string][]string

// Add a reference once per named storage, even when it occurs at both ends.
func (r storageReferenceIndex) add(ref string, names ...string) {
	for i, name := range names {
		if name != "" && !slices.Contains(names[:i], name) {
			r[name] = append(r[name], ref)
		}
	}
}

func (r storageReferenceIndex) sort() {
	for _, refs := range r {
		sort.Strings(refs)
	}
}

func addDiffReferences(ctx context.Context, metaRedis redis.Cmdable, refs storageReferenceIndex) error {
	if metaRedis == nil {
		return nil
	}
	diffs, err := store.NewDiffIDStore(metaRedis).Get(ctx, struct{}{})
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	for _, d := range diffs {
		seen := map[string]bool{}
		for _, l := range d.Locations {
			if !seen[l.Storage] {
				refs.add("diff report "+l.Storage+"/"+l.Bucket, l.Storage)
				seen[l.Storage] = true
			}
		}
	}
	return nil
}

func storageReferenceIndexFromRepos(ctx context.Context, repos controlplane.Repos, metaRedis redis.Cmdable) (storageReferenceIndex, error) {
	refs := storageReferenceIndex{}
	pgDocs, err := repos.ProtectionGroups.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range pgDocs {
		var g protectiongroups.Group
		if err := json.Unmarshal(d.Value.JSON, &g); err != nil {
			continue
		}
		refs.add("protection group "+g.ID, g.Members...)
	}
	statuses, err := repos.Replications.ListStatuses(ctx, "")
	if err != nil {
		return nil, err
	}
	for id := range statuses {
		if id.IsUserLevel() {
			refs.add(fmt.Sprintf("user replication %s: %s → %s", id.User, id.FromStorage, id.ToStorage), id.FromStorage, id.ToStorage)
		} else {
			refs.add(fmt.Sprintf("bucket replication %s/%s", id.User, id.FromBucket), id.FromStorage, id.ToStorage)
		}
	}
	routeExp, err := repos.Routing.Export(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range routeExp.Routes {
		if r.Bucket == "" {
			refs.add("user routing "+r.User, r.Storage)
		} else {
			refs.add(fmt.Sprintf("bucket routing %s/%s", r.User, r.Bucket), r.Storage)
		}
	}
	fPolicies, err := repos.FailoverPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range fPolicies {
		refs.add("failover policy "+p.Value.Source, append([]string{p.Value.Source}, p.Value.Targets...)...)
	}
	fEvents, err := repos.FailoverEvents.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range fEvents {
		refs.add(fmt.Sprintf("failover event %s/%s", e.Value.User, e.Value.Bucket), e.Value.From, e.Value.To)
	}
	if err := addDiffReferences(ctx, metaRedis, refs); err != nil {
		return nil, err
	}
	refs.sort()
	return refs, nil
}

// RuntimeStorageReferences checks durable data, including orphaned bucket scopes and historical diffs/events.
// Any lookup error is returned: deletion must never proceed with an incomplete reference scan.
func RuntimeStorageReferences(pol s3users.PolicyService, creds objstore.CredsService, configRedis redis.UniversalClient, metaRedis redis.Cmdable) StorageReferences {
	load := RuntimeStorageReferenceIndex(pol, creds, configRedis, metaRedis)
	return func(ctx context.Context, name string) ([]string, error) {
		refs, err := load(ctx)
		if err != nil {
			return nil, err
		}
		return refs[name], nil
	}
}

// RuntimeStorageReferenceIndex scans each reference family once per call. No
// result is cached across requests: update/delete checks always see fresh data.
func RuntimeStorageReferenceIndex(pol s3users.PolicyService, creds objstore.CredsService, configRedis redis.UniversalClient, metaRedis redis.Cmdable) StorageReferencesBatch {
	return func(ctx context.Context) (map[string][]string, error) {
		if cpHolder, ok := creds.(interface{ ControlPlane() controlplane.Store }); ok && cpHolder.ControlPlane() != nil {
			refs, err := storageReferenceIndexFromRepos(ctx, cpHolder.ControlPlane().Repos(), metaRedis)
			if err == nil || !controlplane.IsUnavailable(err) {
				return refs, err
			}
			zerolog.Ctx(ctx).Warn().Err(err).Msg("storage references: PostgreSQL read failed, falling back to Redis")
		}
		refs := storageReferenceIndex{}
		groups, err := protectiongroups.NewStore(configRedis).List(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			refs.add("protection group "+g.ID, g.Members...)
		}
		users := creds.AllOverrideUsers()
		if users == nil {
			users = map[string]struct{}{}
		}
		for _, u := range creds.Identities() {
			users[u] = struct{}{}
		}
		scanned, err := ScanStorageUsers(ctx, configRedis)
		if err != nil {
			return nil, err
		}
		for _, u := range scanned {
			users[u] = struct{}{}
		}
		ur, err := pol.ListUserReplicationsInfo(ctx)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		for id := range ur {
			users[id.User] = struct{}{}
			refs.add(fmt.Sprintf("user replication %s: %s → %s", id.User, id.FromStorage, id.ToStorage), id.FromStorage, id.ToStorage)
		}
		routes, err := pol.GetUserRoutings(ctx)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		for user, target := range routes {
			users[user] = struct{}{}
			refs.add("user routing "+user, target)
		}
		// Bucket replications and routings of all users are read in pipelined batches (one round trip per batch
		// instead of two or more per user). The per-user Redis structures are documented in pkg/store:
		// p:repl_bucket_status_index:<user> (set of replication ids) and p:route_bucket:<user> (hash bucket → storage).
		userList := make([]string, 0, len(users))
		for u := range users {
			userList = append(userList, u)
		}
		sort.Strings(userList)
		for start := 0; start < len(userList); start += referenceScanBatch {
			batch := userList[start:min(start+referenceScanBatch, len(userList))]
			pipe := configRedis.Pipeline()
			repls := make([]*redis.StringSliceCmd, len(batch))
			routes := make([]*redis.MapStringStringCmd, len(batch))
			for i, user := range batch {
				repls[i] = pipe.SMembers(ctx, bucketReplIndexPrefix+user)
				routes[i] = pipe.HGetAll(ctx, bucketRoutePrefix+user)
			}
			if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
				return nil, err
			}
			for i, user := range batch {
				members, err := repls[i].Result()
				if err != nil && !errors.Is(err, redis.Nil) {
					return nil, err
				}
				for _, m := range members {
					id, err := store.StringToBucketReplicationPolicyConverter(m)
					if err != nil {
						return nil, fmt.Errorf("invalid bucket replication index entry %q of user %s: %w", m, user, err)
					}
					refs.add(fmt.Sprintf("bucket replication %s/%s", user, id.FromBucket), id.FromStorage, id.ToStorage)
				}
				bs, err := routes[i].Result()
				if err != nil && !errors.Is(err, redis.Nil) {
					return nil, err
				}
				for b, target := range bs {
					refs.add(fmt.Sprintf("bucket routing %s/%s", user, b), target)
				}
			}
		}
		fs := failover.NewStore(configRedis, nil)
		policies, err := fs.ListFailoverPolicies(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range policies {
			refs.add("failover policy "+p.Source, append([]string{p.Source}, p.Targets...)...)
		}
		events, err := fs.ListFailoverEvents(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range events {
			refs.add(fmt.Sprintf("failover event %s/%s", e.User, e.Bucket), e.From, e.To)
		}
		if err := addDiffReferences(ctx, metaRedis, refs); err != nil {
			return nil, err
		}
		refs.sort()
		return refs, nil
	}
}
