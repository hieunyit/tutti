// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRescanMethodsAreAdminOnly(t *testing.T) {
	for _, m := range []string{"/tutti.Policy/RescanReplication", "/tutti.Policy/UpdateReplicationRescanInterval"} {
		require.True(t, Allowed(RoleAdmin, m), m)
		require.False(t, Allowed(RoleViewer, m), "a viewer must not start scans or change the interval: %s", m)
	}
	// reading the rescan state stays available to viewers through the existing read RPCs
	require.True(t, Allowed(RoleViewer, "/tutti.Policy/GetReplication"))
	require.True(t, Allowed(RoleViewer, "/tutti.Policy/ListReplications"))
}
