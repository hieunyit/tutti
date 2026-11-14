package auth

import (
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"testing"

	"github.com/stretchr/testify/require"
)

// The audit log carries before/after documents of every control-plane change; only admins may read it.
func TestAuditLogIsAdminOnly(t *testing.T) {
	require.True(t, Allowed(RoleAdmin, "/tutti.Resilience/ListAuditLog"))
	require.False(t, Allowed(RoleViewer, "/tutti.Resilience/ListAuditLog"))
}

// AvailableBuckets without a user lists every bucket the storage connection account sees (and the owner ids of
// covered buckets): admin-only. Viewers keep it when they pass a user.
func TestAvailableBucketsWithoutUserIsAdminOnly(t *testing.T) {
	const m = "/tutti.Policy/AvailableBuckets"
	empty := &pb.AvailableBucketsRequest{FromStorage: "a", ToStorage: "b"}
	named := &pb.AvailableBucketsRequest{User: "alice", FromStorage: "a", ToStorage: "b"}
	require.True(t, AllowedRequest(RoleAdmin, m, empty))
	require.False(t, AllowedRequest(RoleViewer, m, empty))
	require.True(t, AllowedRequest(RoleViewer, m, named))
	require.True(t, AllowedRequest(RoleAdmin, m, named))
	require.False(t, AllowedRequest(RoleViewer, "/tutti.Resilience/ListAuditLog", named))
}
