package accesspolicy

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestScope(t *testing.T) {
	ctx := context.Background()
	pols := map[string]Policy{"alice": {User: "alice", Permission: ReadWrite, Buckets: []string{"alice-*"}}}
	sc := &Scope{
		Shared: func(storage, user string) bool { return storage == "main" && user != "own" },
		Get: func(_ context.Context, u string) (Policy, error) {
			if p, ok := pols[u]; ok {
				return p, nil
			}
			return Policy{}, fmt.Errorf("%w: policy", dom.ErrNotFound)
		},
	}
	all := []string{"alice-1", "alice-2", "bob-1", "misc"}
	got, err := sc.Filter(ctx, "main", "alice", all)
	require.NoError(t, err)
	require.Equal(t, []string{"alice-1", "alice-2"}, got)
	// no policy on a shared account: nothing
	got, err = sc.Filter(ctx, "main", "nopol", all)
	require.NoError(t, err)
	require.Empty(t, got)
	// own credentials or another storage: unchanged
	got, _ = sc.Filter(ctx, "main", "own", all)
	require.Equal(t, all, got)
	got, _ = sc.Filter(ctx, "other", "nopol", all)
	require.Equal(t, all, got)
	// nil scope: unchanged
	got, err = (*Scope)(nil).Filter(ctx, "main", "nopol", all)
	require.NoError(t, err)
	require.Equal(t, all, got)

	require.ErrorIs(t, sc.CheckUserReplication(ctx, "main", "nopol"), dom.ErrFailedPrecondition)
	require.NoError(t, sc.CheckUserReplication(ctx, "main", "alice"))
	require.NoError(t, sc.CheckUserReplication(ctx, "main", "own"))
	require.NoError(t, (*Scope)(nil).CheckUserReplication(ctx, "main", "nopol"))
	// store errors surface
	sc.Get = func(context.Context, string) (Policy, error) { return Policy{}, fmt.Errorf("redis down") }
	_, err = sc.Filter(ctx, "main", "alice", all)
	require.Error(t, err)
}

// A storage-level replication (empty user) lists with the connection account and replicates every bucket: the
// shared-account policy filter must not apply (it used to look up a policy for "" and return no buckets).
func TestAccessPolicyFilterEmptyUser(t *testing.T) {
	ctx := context.Background()
	sc := &Scope{
		Shared: func(string, string) bool { return true },
		Get: func(context.Context, string) (Policy, error) {
			return Policy{}, fmt.Errorf("%w: policy", dom.ErrNotFound)
		},
	}
	all := []string{"a", "b", "c"}
	got, err := sc.Filter(ctx, "main", "", all)
	require.NoError(t, err)
	require.Equal(t, all, got)
	require.NoError(t, sc.CheckUserReplication(ctx, "main", ""))
}
