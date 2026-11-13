package accesspolicy

import (
	"context"
	"errors"
	"fmt"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Scope restricts the buckets that background jobs (user-level replication, switch) see for a user. A user on a
// shared connection account sees every bucket of the storage when listing with that account, so the listing must be
// cut down to the buckets its access policy allows. A user with its own storage credentials is not restricted:
// listing with them already returns only its buckets. A nil *Scope restricts nothing.
type Scope struct {
	// Shared reports whether requests of the user on the storage are signed with a shared connection account.
	Shared func(storage, user string) bool
	// Get returns the policy of the user or an error wrapping dom.ErrNotFound.
	Get func(ctx context.Context, user string) (Policy, error)
}

// NewScope builds a Scope from the credential mode check and the policy store.
func NewScope(shared func(storage, user string) bool, store *Store) *Scope {
	return &Scope{Shared: shared, Get: store.Get}
}

// Filter returns the buckets of the list the user may see on the storage. An empty user (storage-level
// replication with the connection account) sees every bucket.
func (s *Scope) Filter(ctx context.Context, storage, user string, buckets []string) ([]string, error) {
	if s == nil || user == "" || !s.Shared(storage, user) {
		return buckets, nil
	}
	pol, err := s.Get(ctx, user)
	if err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			return nil, nil // no policy: no buckets
		}
		return nil, err
	}
	res := make([]string, 0, len(buckets))
	for _, b := range buckets {
		if pol.BucketAllowed(b) {
			res = append(res, b)
		}
	}
	return res, nil
}

// CheckUserReplication refuses a user-level replication of a user on a shared account without a policy: it would
// have nothing to replicate. A storage-level replication (empty user) replicates every bucket and needs no policy.
func (s *Scope) CheckUserReplication(ctx context.Context, storage, user string) error {
	if s == nil || user == "" || !s.Shared(storage, user) {
		return nil
	}
	if _, err := s.Get(ctx, user); err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			return fmt.Errorf("%w: user %q uses the shared connection account of storage %q and has no access policy: there are no buckets to replicate; set an access policy first or replicate individual buckets",
				dom.ErrFailedPrecondition, user, storage)
		}
		return err
	}
	return nil
}
