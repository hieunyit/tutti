// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package resilience

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

// A legacy failover policy main -> [f1] is migrated to an infrastructure protection group that adopts the
// existing replications (no re-copy). The legacy policy is disabled and, with main blackholed, the group (not the
// legacy controller) fails the storage over to f1.
func TestFailoverMigration_LegacyPolicyToProtectionGroups(t *testing.T) {
	e := startGroupEnv(t, []string{"p1", "p2", "p3"}, nil)
	ctx := t.Context()
	b1, b2 := bucketName("mig-one"), bucketName("mig-two")
	e.replicateBucket(t, b1, f1Key)
	e.replicateBucket(t, b2, f1Key)
	for _, b := range []string{b1, b2} {
		_, err := direct[mainKey].PutObject(ctx, b, "before", strings.NewReader("before"), 6, mclient.PutObjectOptions{})
		require.NoError(t, err)
	}
	e.setPolicy(t, mainKey, true, f1Key)

	plan := e.restJSON(t, http.MethodGet, "/tutti/failover/migration/plan?source="+mainKey, nil)
	scopes, _ := plan["scopes"].([]any)
	require.Len(t, scopes, 1, "%v", plan)
	sc := scopes[0].(map[string]any)
	require.Equal(t, "ok", sc["status"], "%v", sc)
	require.Equal(t, "", sc["group"].(map[string]any)["user"])
	require.Equal(t, []any{mainKey, f1Key}, sc["group"].(map[string]any)["members"])
	require.Equal(t, true, plan["complete"])

	res := e.restJSON(t, http.MethodPost, "/tutti/failover/migration/apply", map[string]any{"source": mainKey})
	require.Len(t, res["created"], 1, "%v", res)
	require.Equal(t, true, res["policyDisabled"])
	pols := e.restJSON(t, http.MethodGet, "/tutti/failover/policies", nil)["policies"].([]any)
	require.Len(t, pols, 1)
	require.NotEqual(t, true, pols[0].(map[string]any)["enabled"], "legacy policy must be disabled")

	id := "storage-" + mainKey
	e.waitGroup(t, id, 90*time.Second, func(g map[string]any) bool { return g["state"] == "ready" && g["frozen"] != true }, "group did not become ready")
	// idempotent re-run
	res = e.restJSON(t, http.MethodPost, "/tutti/failover/migration/apply", map[string]any{"source": mainKey})
	require.Empty(t, res["created"])
	require.Len(t, res["migrated"], 1)

	// the group owns the storage: main down -> automatic group failover to f1
	e.fwd[mainKey].setMode(modeBlackhole)
	g := e.waitGroup(t, id, 90*time.Second, func(g map[string]any) bool { return g["primary"] == f1Key }, "no group failover")
	require.Equal(t, "2", fmt.Sprint(g["generation"]))

	require.Equal(t, f1Key, e.routeOf(t, b1))
	require.Equal(t, f1Key, e.routeOf(t, b2))
	putEventually(t, e.clientOf(t, "p1"), b1, "after", "on f1")
	require.True(t, existsDirect(t, f1Key, b1, "after"))
}
