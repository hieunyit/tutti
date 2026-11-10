// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"context"
	"time"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func values[T any](vs []controlplane.Versioned[T]) []T {
	res := make([]T, len(vs))
	for i, v := range vs {
		res[i] = v.Value
	}
	return res
}

// ReadPG reads all control-plane data through the repositories.
func ReadPG(ctx context.Context, r controlplane.Repos) (*Data, error) {
	d := &Data{ConsoleLogins: map[string]time.Time{}}
	var err error
	st, err := r.Storages.List(ctx)
	if err != nil {
		return nil, err
	}
	d.Storages = values(st)
	ids, err := r.S3Identities.List(ctx)
	if err != nil {
		return nil, err
	}
	d.S3Identities = values(ids)
	if d.DynamicCreds, err = r.DynamicCredentials.List(ctx); err != nil {
		return nil, err
	}
	if salt, ok, err := r.DynamicCredentials.Salt(ctx); err != nil {
		return nil, err
	} else if ok {
		d.DynamicSalt = salt
	}
	pa, err := r.ProxyAccounts.List(ctx)
	if err != nil {
		return nil, err
	}
	d.ProxyAccounts = values(pa)
	ap, err := r.AccessPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	d.AccessPolicies = values(ap)
	cu, err := r.ConsoleUsers.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range values(cu) {
		if !u.LastLoginAt.IsZero() {
			d.ConsoleLogins[u.Username] = u.LastLoginAt.UTC()
		}
		u.LastLoginAt = time.Time{}
		d.ConsoleUsers = append(d.ConsoleUsers, u)
	}
	if d.Routing, err = r.Routing.Export(ctx); err != nil {
		return nil, err
	}
	if d.ReplPolicies, err = r.Replications.ListAll(ctx); err != nil {
		return nil, err
	}
	if d.ReplStatuses, err = r.Replications.ListStatuses(ctx, ""); err != nil {
		return nil, err
	}
	if d.ReplGenerations, err = r.Replications.ListGenerations(ctx); err != nil {
		return nil, err
	}
	fp, err := r.FailoverPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	d.FailoverPolicies = values(fp)
	fe, err := r.FailoverEvents.List(ctx)
	if err != nil {
		return nil, err
	}
	d.FailoverEvents = values(fe)
	pg, err := r.ProtectionGroups.List(ctx)
	if err != nil {
		return nil, err
	}
	d.ProtectionGroups = values(pg)
	tests, err := r.StorageTests.List(ctx)
	if err != nil {
		return nil, err
	}
	d.StorageTests = values(tests)
	return d, nil
}
