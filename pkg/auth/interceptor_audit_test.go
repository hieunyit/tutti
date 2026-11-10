package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The audit log carries before/after documents of every control-plane change; only admins may read it.
func TestAuditLogIsAdminOnly(t *testing.T) {
	require.True(t, Allowed(RoleAdmin, "/chorus.Resilience/ListAuditLog"))
	require.False(t, Allowed(RoleViewer, "/chorus.Resilience/ListAuditLog"))
}
