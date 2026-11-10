// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	var refs []string
	pgDocs, err := repos.ProtectionGroups.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range pgDocs {
		var g protectiongroups.Group
		if err := json.Unmarshal(d.Value.JSON, &g); err != nil {
			continue
		}
		for _, m := range g.Members {
			if m == name {
				refs = append(refs, "protection group "+g.ID)
				break
			}
		}
	}
	statuses, err := repos.Replications.ListStatuses(ctx, "")
	if err != nil {
		return nil, err
	}
	for id := range statuses {
		if id.FromStorage != name && id.ToStorage != name {
			continue
		}
		if id.IsUserLevel() {
			refs = append(refs, fmt.Sprintf("user replication %s: %s → %s", id.User, id.FromStorage, id.ToStorage))
		} else {
			refs = append(refs, fmt.Sprintf("bucket replication %s/%s", id.User, id.FromBucket))
		}
	}
	routeExp, err := repos.Routing.Export(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range routeExp.Routes {
		if r.Storage != name {
			continue
		}
		if r.Bucket == "" {
			refs = append(refs, "user routing "+r.User)
		} else {
			refs = append(refs, fmt.Sprintf("bucket routing %s/%s", r.User, r.Bucket))
		}
	}
	fPolicies, err := repos.FailoverPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range fPolicies {
		matches := p.Value.Source == name
		for _, t := range p.Value.Targets {
			matches = matches || t == name
		}
		if matches {
			refs = append(refs, "failover policy "+p.Value.Source)
		}
	}
	fEvents, err := repos.FailoverEvents.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range fEvents {
		if e.Value.From == name || e.Value.To == name {
			refs = append(refs, fmt.Sprintf("failover event %s/%s", e.Value.User, e.Value.Bucket))
		}
	}
	if metaRedis != nil {
		diffs, err := store.NewDiffIDStore(metaRedis).Get(ctx, struct{}{})
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		for _, d := range diffs {
			for _, l := range d.Locations {
				if l.Storage == name {
					refs = append(refs, "diff report "+name+"/"+l.Bucket)
					break
				}
			}
		}
	}
	sort.Strings(refs)
	return refs, nil
}

// RuntimeStorageReferences checks durable data, including orphaned bucket scopes and historical diffs/events.
// Any lookup error is returned: deletion must never proceed with an incomplete reference scan.
func RuntimeStorageReferences(pol s3users.PolicyService, creds objstore.CredsService, configRedis redis.UniversalClient, metaRedis redis.Cmdable) StorageReferences {
	return func(ctx context.Context, name string) ([]string, error) {
		if cpHolder, ok := creds.(interface{ ControlPlane() controlplane.Store }); ok && cpHolder.ControlPlane() != nil {
			refs, err := StorageReferencesFromRepos(ctx, cpHolder.ControlPlane().Repos(), metaRedis, name)
			if err == nil || !controlplane.IsUnavailable(err) {
				return refs, err
			}
			zerolog.Ctx(ctx).Warn().Err(err).Str("storage", name).Msg("storage references: PostgreSQL read failed, falling back to Redis")
		}
		var refs []string
		groups, err := protectiongroups.NewStore(configRedis).References(ctx, "", name)
		if err != nil {
			return nil, err
		}
		refs = append(refs, groups...)
		users := map[string]struct{}{}
		for st := range creds.Storages() {
			for _, u := range creds.ListUsers(st) {
				users[u] = struct{}{}
			}
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
			if id.FromStorage == name || id.ToStorage == name {
				refs = append(refs, fmt.Sprintf("user replication %s: %s → %s", id.User, id.FromStorage, id.ToStorage))
			}
		}
		routes, err := pol.GetUserRoutings(ctx)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		for user, target := range routes {
			users[user] = struct{}{}
			if target == name {
				refs = append(refs, "user routing "+user)
			}
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
					if id.FromStorage == name || id.ToStorage == name {
						refs = append(refs, fmt.Sprintf("bucket replication %s/%s", user, id.FromBucket))
					}
				}
				bs, err := routes[i].Result()
				if err != nil && !errors.Is(err, redis.Nil) {
					return nil, err
				}
				for b, target := range bs {
					if target == name {
						refs = append(refs, fmt.Sprintf("bucket routing %s/%s", user, b))
					}
				}
			}
		}
		fs := failover.NewStore(configRedis, nil)
		policies, err := fs.ListFailoverPolicies(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range policies {
			matches := p.Source == name
			for _, t := range p.Targets {
				matches = matches || t == name
			}
			if matches {
				refs = append(refs, "failover policy "+p.Source)
			}
		}
		events, err := fs.ListFailoverEvents(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range events {
			if e.From == name || e.To == name {
				refs = append(refs, fmt.Sprintf("failover event %s/%s", e.User, e.Bucket))
			}
		}
		diffs, err := store.NewDiffIDStore(metaRedis).Get(ctx, struct{}{})
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		for _, d := range diffs {
			for _, l := range d.Locations {
				if l.Storage == name {
					refs = append(refs, "diff report "+name+"/"+l.Bucket)
					break
				}
			}
		}
		sort.Strings(refs)
		return refs, nil
	}
}
