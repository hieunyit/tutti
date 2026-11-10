// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/redis/go-redis/v9"
)

// Legacy failover policies (pkg/failover) and protection groups must never manage the same storages: both would
// reroute the same scopes on an outage (double failover). Creating or enabling a legacy policy whose source or a
// target is a group member is refused, and so is creating a group over a storage with an enabled legacy policy.
// The legacy controller additionally skips every scope a group manages, and every scope whose protection
// cannot be determined.

// MemberGroups returns, for every storage that is a member of a protection group, the IDs of those groups.
// It reads the authoritative Redis documents.
func MemberGroups(ctx context.Context, r redis.Cmdable) (map[string][]string, error) {
	raw, err := r.HGetAll(ctx, GroupsKey).Result()
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for id, v := range raw {
		var g Group
		if err := json.Unmarshal([]byte(v), &g); err != nil {
			return nil, err
		}
		for _, m := range g.Members {
			out[m] = append(out[m], id)
		}
	}
	for _, ids := range out {
		sort.Strings(ids)
	}
	return out, nil
}

// CheckLegacyFailoverPolicy rejects an enabled legacy failover policy whose source or targets are members of a
// protection group. Disabled policies are allowed (they are never acted upon).
func CheckLegacyFailoverPolicy(ctx context.Context, r redis.Cmdable, p failover.Policy) error {
	if !p.Enabled {
		return nil
	}
	members, err := MemberGroups(ctx, r)
	if err != nil {
		return fmt.Errorf("%w: unable to check protection groups: %v", dom.ErrUnavailable, err)
	}
	for _, s := range append([]string{p.Source}, p.Targets...) {
		if ids := members[s]; len(ids) != 0 {
			return fmt.Errorf("%w: storage %s is a member of protection group %s; protection groups handle its failover, so a legacy failover policy cannot be enabled for it",
				dom.ErrFailedPrecondition, s, ids[0])
		}
	}
	return nil
}

// LegacyFailoverValidator returns a group validator that rejects a group whose members are the source or a
// target of an enabled legacy failover policy. list returns the legacy policies.
func LegacyFailoverValidator(list func(context.Context) ([]failover.Policy, error)) func(context.Context, Group) error {
	return func(ctx context.Context, g Group) error {
		policies, err := list(ctx)
		if err != nil {
			return fmt.Errorf("%w: unable to check legacy failover policies: %v", dom.ErrUnavailable, err)
		}
		for _, p := range policies {
			if !p.Enabled {
				continue
			}
			for _, s := range append([]string{p.Source}, p.Targets...) {
				if slices.Contains(g.Members, s) {
					return fmt.Errorf("%w: storage %s is used by the enabled legacy failover policy of source %s; disable or delete that policy before protecting the storage with a group",
						dom.ErrFailedPrecondition, s, p.Source)
				}
			}
		}
		return nil
	}
}
