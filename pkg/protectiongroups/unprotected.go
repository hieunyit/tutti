// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"slices"
	"time"

	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/metrics"
)

// UnprotectedScope is a scope that has a replication from Storage (and is routed to it) but is neither covered by
// an enabled legacy failover policy of Storage nor by a protection group: an outage of Storage needs a manual
// failover for it.
type UnprotectedScope struct {
	Storage string   `json:"storage"`
	User    string   `json:"user"`
	Bucket  string   `json:"bucket,omitempty"`
	Targets []string `json:"targets"` // destinations of its replications
}

// UnprotectedOptions parameterize Unprotected.
type UnprotectedOptions struct {
	// Storages are the configured storages.
	Storages    []string
	MainStorage string
	Users       func() []string
	Groups      func(context.Context) ([]Group, error)
	Policies    func(context.Context) ([]failover.Policy, error)
}

// Unprotected lists the unprotected scopes per the definition of failover.DiscoverScopes (the scopes the legacy
// controller would act on), so it matches exactly what a legacy policy would cover.
func Unprotected(ctx context.Context, src failover.ScopeSource, o UnprotectedOptions) ([]UnprotectedScope, error) {
	groups, err := o.Groups(ctx)
	if err != nil {
		return nil, err
	}
	policies, err := o.Policies(ctx)
	if err != nil {
		return nil, err
	}
	legacy := map[string]bool{}
	for _, p := range policies {
		if p.Enabled && len(p.Targets) != 0 {
			legacy[p.Source] = true
		}
	}
	covered := func(_ context.Context, user, bucket string) bool {
		for _, g := range groups {
			if g.User == user && (len(g.Buckets) == 0 || (bucket != "" && slices.Contains(g.Buckets, bucket))) {
				return true
			}
		}
		return false
	}
	var out []UnprotectedScope
	do := failover.DiscoverOptions{MainStorage: o.MainStorage, Skip: covered}
	if o.Users != nil {
		do.Users = o.Users()
	}
	for _, st := range o.Storages {
		if legacy[st] {
			continue
		}
		scopes, err := failover.DiscoverScopes(ctx, src, st, do)
		if err != nil {
			return nil, err
		}
		for _, sc := range scopes {
			u := UnprotectedScope{Storage: st, User: sc.User, Bucket: sc.Bucket}
			for _, c := range sc.Candidates {
				if !slices.Contains(u.Targets, c.To) {
					u.Targets = append(u.Targets, c.To)
				}
			}
			slices.Sort(u.Targets)
			out = append(out, u)
		}
	}
	return out, nil
}

// CollectUnprotectedMetrics refreshes the replication_scopes_unprotected gauge (one series per configured storage).
func CollectUnprotectedMetrics(ctx context.Context, src failover.ScopeSource, o UnprotectedOptions) error {
	list, err := Unprotected(ctx, src, o)
	if err != nil {
		return err
	}
	counts := make(map[string]int, len(o.Storages))
	for _, s := range o.Storages {
		counts[s] = 0
	}
	for _, u := range list {
		counts[u.Storage]++
	}
	metrics.SetUnprotectedScopes(counts)
	return nil
}

// RunUnprotectedMetrics refreshes the metric until ctx is done.
func RunUnprotectedMetrics(ctx context.Context, src failover.ScopeSource, o func() UnprotectedOptions, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := CollectUnprotectedMetrics(ctx, src, o()); err != nil && onError != nil && ctx.Err() == nil {
			onError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
