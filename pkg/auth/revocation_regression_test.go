package auth

import (
	"context"
	"encoding/json"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"time"
)

type comprehensiveUsers struct {
	controlplane.ConsoleUsers
	offline *bool
	current controlplane.ConsoleUser
}

func (c comprehensiveUsers) Get(context.Context, string) (controlplane.Versioned[controlplane.ConsoleUser], error) {
	if *c.offline {
		return controlplane.Versioned[controlplane.ConsoleUser]{}, io.EOF
	}
	return controlplane.Versioned[controlplane.ConsoleUser]{Value: c.current, Version: 2}, nil
}

type comprehensiveCP struct {
	controlplane.Store
	users controlplane.ConsoleUsers
}

func (c comprehensiveCP) Repos() controlplane.Repos { return controlplane.Repos{ConsoleUsers: c.users} }
func TestRegression_RevokedAdminMustNotReviveDuringPGOutage(t *testing.T) {
	svc, _, rc := newManagedSvc(t, testConf(t, true))
	now := time.Now().UTC()
	old := ManagedUser{Username: "revoked-admin", Role: RoleAdmin, PasswordHash: "hash", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), PasswordChangedAt: now.Add(-time.Hour)}
	raw, err := json.Marshal(old)
	require.NoError(t, err)
	require.NoError(t, rc.HSet(t.Context(), ConsoleUsersKey, old.Username, string(raw)).Err())
	session, err := json.Marshal(Session{Username: old.Username, Role: RoleAdmin, CreatedAt: now})
	require.NoError(t, err)
	token := "prior-session-token"
	require.NoError(t, rc.Set(t.Context(), sessionPrefix+hashToken(token), session, time.Hour).Err())
	// Authoritative PG has disabled the admin at revision 2; Redis projection remains revision 1.
	current := toCPUser(old)
	current.Disabled = true
	current.UpdatedAt = now
	offline := false
	svc.managed.WithControlPlane(comprehensiveCP{users: comprehensiveUsers{offline: &offline, current: current}})
	_, err = svc.Authenticate(t.Context(), token)
	require.ErrorIs(t, err, ErrInvalidSession, "healthy PG must enforce revocation")
	// Simulate another process with cold/expired lookup cache after PostgreSQL becomes unavailable.
	svc.managed.forget(old.Username)
	offline = true
	sess, err := svc.Authenticate(t.Context(), token)
	if err == nil {
		t.Fatalf("revoked administrator revived: username=%s role=%s permitsGetProxyCredentials=%v", sess.Username, sess.Role, Allowed(sess.Role, "/tutti.Tutti/GetProxyCredentials"))
	}
}

// Non-admin users may still be served from the Redis projection while PostgreSQL is
// unavailable (availability), unlike admins which fail closed.
func TestRegression_NonAdminFallsBackToRedisDuringPGOutage(t *testing.T) {
	svc, _, rc := newManagedSvc(t, testConf(t, true))
	now := time.Now().UTC()
	u := ManagedUser{Username: "viewer", Role: RoleViewer, PasswordHash: "hash", CreatedAt: now, UpdatedAt: now, PasswordChangedAt: now}
	raw, err := json.Marshal(u)
	require.NoError(t, err)
	require.NoError(t, rc.HSet(t.Context(), ConsoleUsersKey, u.Username, string(raw)).Err())
	offline := true
	svc.managed.WithControlPlane(comprehensiveCP{users: comprehensiveUsers{offline: &offline, current: toCPUser(u)}})
	svc.managed.forget(u.Username)
	got, err := svc.managed.Get(t.Context(), u.Username)
	require.NoError(t, err)
	require.Equal(t, u.Username, got.Username)
}
