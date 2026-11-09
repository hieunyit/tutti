package failover

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func known(names ...string) func(string) bool {
	return func(s string) bool {
		for _, n := range names {
			if n == s {
				return true
			}
		}
		return false
	}
}

func TestStore_PolicyCRUD(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	s := NewStore(testutil.SetupRedis(t), known("main", "f1", "f2"))

	_, err := s.GetFailoverPolicy(ctx, "main")
	r.ErrorIs(err, dom.ErrNotFound)

	lag := 3
	p := Policy{Source: "main", Enabled: true, Targets: []string{"f2", "f1"}, After: time.Minute, MaxEventLag: &lag}
	r.NoError(s.SetFailoverPolicy(ctx, p))
	got, err := s.GetFailoverPolicy(ctx, "main")
	r.NoError(err)
	r.Equal(p, got)

	r.NoError(s.SetFailoverPolicy(ctx, Policy{Source: "f1", Enabled: false}))
	list, err := s.ListFailoverPolicies(ctx)
	r.NoError(err)
	r.Len(list, 2)
	r.Equal("f1", list[0].Source)
	r.Equal("main", list[1].Source)

	r.NoError(s.DeleteFailoverPolicy(ctx, "main"))
	r.NoError(s.DeleteFailoverPolicy(ctx, "main"))
	_, err = s.GetFailoverPolicy(ctx, "main")
	r.ErrorIs(err, dom.ErrNotFound)
}

func TestStore_PolicyValidation(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	s := NewStore(testutil.SetupRedis(t), known("main", "f1", "f2"))
	neg := -1
	for name, p := range map[string]Policy{
		"no source":         {Targets: []string{"f1"}},
		"unknown source":    {Source: "x", Targets: []string{"f1"}},
		"unknown target":    {Source: "main", Enabled: true, Targets: []string{"x"}},
		"target is source":  {Source: "main", Targets: []string{"main"}},
		"duplicate target":  {Source: "main", Targets: []string{"f1", "f1"}},
		"empty target":      {Source: "main", Targets: []string{""}},
		"negative after":    {Source: "main", Targets: []string{"f1"}, After: -time.Second},
		"negative lag":      {Source: "main", Targets: []string{"f1"}, MaxEventLag: &neg},
		"enabled no target": {Source: "main", Enabled: true},
	} {
		r.ErrorIs(s.SetFailoverPolicy(ctx, p), dom.ErrInvalidArg, name)
	}
	list, err := s.ListFailoverPolicies(ctx)
	r.NoError(err)
	r.Empty(list)
}

func TestStore_Events(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	s := NewStore(testutil.SetupRedis(t), nil)
	e1 := Event{User: "u1", From: "main", To: "f1", Time: time.Unix(100, 0).UTC(), EventLag: 2}
	e2 := Event{User: "u1", Bucket: "b", From: "main", To: "f1", Time: time.Unix(200, 0).UTC()}
	r.NoError(s.SaveFailoverEvent(ctx, e1))
	r.NoError(s.SaveFailoverEvent(ctx, e2))
	list, err := s.ListFailoverEvents(ctx)
	r.NoError(err)
	r.Equal([]Event{e2, e1}, list) // sorted by key: bucket:... < user:...
	r.NoError(s.DeleteFailoverEvent(ctx, "u1", "b"))
	list, err = s.ListFailoverEvents(ctx)
	r.NoError(err)
	r.Equal([]Event{e1}, list)
}
