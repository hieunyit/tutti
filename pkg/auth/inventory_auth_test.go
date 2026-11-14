package auth

import (
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestAnonymousMustNotReadRuntimeStorageInventory(t *testing.T) {
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	_, err := svc.check(t.Context(), "/tutti.Resilience/ListRuntimeStorages")
	require.Equal(t, codes.Unauthenticated, status.Code(err), "a health probe must not make management storage inventory public")
}
