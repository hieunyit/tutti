// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/redis/go-redis/v9"
)

type managedKey struct{}

// Managed is private control-plane authorization for mutations already coordinated by the
// group reconciler. It must never be derived from an HTTP header or request parameter.
func Managed(ctx context.Context) context.Context { return context.WithValue(ctx, managedKey{}, true) }
func CheckUnmanaged(ctx context.Context, r redis.Cmdable, user, bucket string) error {
	if yes, _ := ctx.Value(managedKey{}).(bool); yes {
		return nil
	}
	raw, err := r.HVals(ctx, GroupsKey).Result()
	if err != nil {
		return err
	}
	for _, v := range raw {
		var g Group
		if err = json.Unmarshal([]byte(v), &g); err != nil {
			return err
		}
		if g.User == user && (bucket == "" || g.Covers(user, bucket)) {
			return fmt.Errorf("%w: scope is managed by protection group %s; use its API", dom.ErrFailedPrecondition, g.ID)
		}
	}
	return nil
}
