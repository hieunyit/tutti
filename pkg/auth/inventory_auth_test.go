package auth

import (
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestAnonymousMustNotReadRuntimeStorageInventory(t *testing.T) {
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	for _, method := range []string{"/tutti.Resilience/ListRuntimeStorages", "/tutti.Resilience/GetRuntimeStorage"} {
		_, err := svc.check(t.Context(), method)
		require.Equal(t, codes.Unauthenticated, status.Code(err), "a health probe must not make management storage inventory public")
	}
}

func TestViewerCanReadStorageDetailsWithoutManagingStorage(t *testing.T) {
	// The users tab now fetches a single storage, with the same inventory fields
	// viewers could already obtain from ListRuntimeStorages.
	require.True(t, Allowed(RoleViewer, "/tutti.Resilience/GetRuntimeStorage"))
	for _, method := range []string{"CreateRuntimeStorage", "UpdateRuntimeStorage", "DeleteRuntimeStorage", "RotateStorageConnectionAccount", "TestStorageConnection"} {
		require.False(t, Allowed(RoleViewer, "/tutti.Resilience/"+method), method)
	}
	require.False(t, Allowed(RoleViewer, "/tutti.Tutti/GetProxyCredentials"))
}
