// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"fmt"
	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
)

func CredentialValidator(creds objstore.CredsService, policies *accesspolicy.Store) func(context.Context, Group) error {
	return func(ctx context.Context, g Group) error {
		known := creds.HasIdentity(g.User)
		for _, m := range g.Members {
			for _, u := range creds.ListOverrideUsers(m) {
				if u == g.User {
					known = true
				}
			}
		}
		if !known {
			return fmt.Errorf("%w: unknown protection-group user %q: use a configured S3 identity or a user with override credentials on a member", dom.ErrInvalidArg, g.User)
		}
		for _, m := range g.Members {
			if creds.Storages()[m] != dom.S3 {
				return fmt.Errorf("%w: group members must be configured S3 storages", dom.ErrInvalidArg)
			}
			_, mode, err := creds.ResolveS3Credentials(m, g.User)
			if err != nil {
				return err
			}
			if mode == objstore.ModeConnection {
				p, err := policies.Get(ctx, g.User)
				if err != nil {
					return fmt.Errorf("%w: shared-account group needs an access policy on every member: %v", dom.ErrFailedPrecondition, err)
				}
				for _, bucket := range g.Buckets {
					if !p.BucketAllowed(bucket) {
						return fmt.Errorf("%w: group bucket %s is outside the user's access policy", dom.ErrInvalidArg, bucket)
					}
				}
			}
		}
		return nil
	}
}
