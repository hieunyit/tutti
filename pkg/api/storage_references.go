// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3users"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/redis/go-redis/v9"
)

// RuntimeStorageReferences checks durable data, including orphaned bucket scopes and historical diffs/events.
// Any lookup error is returned: deletion must never proceed with an incomplete reference scan.
func RuntimeStorageReferences(pol s3users.PolicyService, creds objstore.CredsService, configRedis redis.UniversalClient, metaRedis redis.Cmdable) StorageReferences {
	return func(ctx context.Context, name string) ([]string, error) {
		var refs []string
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
		for user := range users {
			br, err := pol.ListBucketReplicationsInfo(ctx, user)
			if err != nil && !errors.Is(err, dom.ErrNotFound) {
				return nil, err
			}
			for id := range br {
				if id.FromStorage == name || id.ToStorage == name {
					refs = append(refs, fmt.Sprintf("bucket replication %s/%s", user, id.FromBucket))
				}
			}
			bs, err := pol.GetBucketRoutings(ctx, user)
			if err != nil && !errors.Is(err, dom.ErrNotFound) {
				return nil, err
			}
			for b, target := range bs {
				if target == name {
					refs = append(refs, fmt.Sprintf("bucket routing %s/%s", user, b))
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
