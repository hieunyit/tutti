// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package resilience

import (
	"net/http"
	"strings"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
)

// TestStorageGroup_MultiTenantIsolationAndFailover tests Option B:
// 3 tenants (alice, bob, carol) share a connection account across main and f1.
// An infrastructure storage-level protection group covers [main, f1] with primary=main.
// 1. Tenants can write to their own buckets on main; tenant isolation prevents cross-tenant access.
// 2. Storage group switches / fails over to f1.
// 3. All 3 tenants route to f1, writes land on f1, and tenant isolation is preserved.
// 4. A 4th tenant (dave) created AFTER failover also routes to f1 and remains isolated.
func TestStorageGroup_MultiTenantIsolationAndFailover(t *testing.T) {
	ctx := t.Context()
	mainAcc := createAccount(t, minioAdmin(t, mainKey), "tutti-storage-iso", "tutti-storage-iso-sec-1", "tutti-iso-rw", tuttiRW)
	f1Acc := createAccount(t, minioAdmin(t, f1Key), "tutti-storage-iso", "tutti-storage-iso-sec-2", "tutti-iso-rw", tuttiRW)
	e := startEnv(t, envOpts{
		dynamicCreds: true,
		proxyHook: func(c *proxy.Config) {
			delete(c.Storage.Storages, f2Key)
			c.Status.Interval = 500 * time.Millisecond
			c.ProtectionGroups.WriterLease = 3 * time.Second
			st := c.Storage.Storages[mainKey].S3
			st.Credentials, st.ConnectionAccount = nil, &mainAcc
			st1 := c.Storage.Storages[f1Key].S3
			st1.Credentials, st1.ConnectionAccount = nil, &f1Acc
		},
		workerHook: func(c *worker.Config) {
			delete(c.Storage.Storages, f2Key)
			st := c.Storage.Storages[mainKey].S3
			st.Credentials, st.ConnectionAccount = nil, &mainAcc
			st1 := c.Storage.Storages[f1Key].S3
			st1.Credentials, st1.ConnectionAccount = nil, &f1Acc
		},
	})

	newUser := func(name string, glob string) *mclient.Client {
		res := e.restJSON(t, http.MethodPost, "/tutti/users", map[string]any{
			"name":            name,
			"createAccessKey": true,
			"policy": map[string]any{
				"permission": accesspolicy.ReadWrite,
				"buckets":    []string{glob},
			},
		})
		key := res["accessKey"].(map[string]any)
		return e.keyClient(t, key["accessKeyId"].(string), key["secretAccessKey"].(string))
	}

	alice := newUser("alice", "iso-alice-*")
	bob := newUser("bob", "iso-bob-*")
	carol := newUser("carol", "iso-carol-*")

	suffix := bucketName("")
	aliceB := "iso-alice-" + suffix
	bobB := "iso-bob-" + suffix
	carolB := "iso-carol-" + suffix
	daveB := "iso-dave-" + suffix

	t.Cleanup(func() {
		for _, b := range []string{aliceB, bobB, carolB, daveB} {
			for _, st := range []string{mainKey, f1Key} {
				removeBucket(t, st, b)
			}
		}
	})

	for _, b := range []string{aliceB, bobB, carolB} {
		for _, st := range []string{mainKey, f1Key} {
			require.NoError(t, direct[st].MakeBucket(ctx, b, mclient.MakeBucketOptions{}))
		}
	}

	group := map[string]any{
		"id":      "infra-sg-main",
		"members": []string{mainKey, f1Key},
		"copies":  2,
		"mode":    "manual",
	}
	e.restJSON(t, http.MethodPost, "/tutti/protection-groups", map[string]any{"group": group})

	waitGroup := func(id string, cond func(g map[string]any) bool, msg string) map[string]any {
		var g map[string]any
		require.Eventually(t, func() bool {
			res := e.restJSON(t, http.MethodGet, "/tutti/protection-groups/"+id, nil)
			g, _ = res["group"].(map[string]any)
			return cond(g)
		}, 60*time.Second, 250*time.Millisecond, msg)
		return g
	}

	waitGroup("infra-sg-main", func(g map[string]any) bool {
		return g["state"] == "ready" && g["frozen"] != true
	}, "group did not become ready")

	put := func(c *mclient.Client, bucket, key, val string) error {
		_, err := c.PutObject(ctx, bucket, key, strings.NewReader(val), int64(len(val)), mclient.PutObjectOptions{})
		return err
	}
	denied := func(err error) {
		t.Helper()
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, mclient.ToErrorResponse(err).StatusCode, "%v", err)
	}

	// 1. Initial writes on mainKey
	require.Eventually(t, func() bool {
		return put(alice, aliceB, "obj1", "alice-v1") == nil
	}, 20*time.Second, 200*time.Millisecond)
	require.NoError(t, put(bob, bobB, "obj1", "bob-v1"))
	require.NoError(t, put(carol, carolB, "obj1", "carol-v1"))

	require.True(t, existsDirect(t, mainKey, aliceB, "obj1"))
	require.True(t, existsDirect(t, mainKey, bobB, "obj1"))
	require.True(t, existsDirect(t, mainKey, carolB, "obj1"))

	// Verify Tenant Isolation before failover:
	denied(put(alice, bobB, "evil", "attack"))
	denied(put(bob, carolB, "evil", "attack"))
	denied(put(carol, aliceB, "evil", "attack"))

	// 2. Switchover to f1Key
	e.restJSON(t, http.MethodPost, "/tutti/protection-groups/switchover", map[string]any{
		"id":     "infra-sg-main",
		"target": f1Key,
	})

	waitGroup("infra-sg-main", func(g map[string]any) bool {
		return g["primary"] == f1Key && g["frozen"] != true && g["state"] == "ready"
	}, "switchover to f1 did not complete")

	// 3. Writes now land on f1Key
	require.Eventually(t, func() bool {
		return put(alice, aliceB, "obj2", "alice-v2") == nil
	}, 20*time.Second, 200*time.Millisecond)
	require.NoError(t, put(bob, bobB, "obj2", "bob-v2"))
	require.NoError(t, put(carol, carolB, "obj2", "carol-v2"))

	require.True(t, existsDirect(t, f1Key, aliceB, "obj2"))
	require.True(t, existsDirect(t, f1Key, bobB, "obj2"))
	require.True(t, existsDirect(t, f1Key, carolB, "obj2"))

	// Verify Tenant Isolation AFTER failover:
	denied(put(alice, bobB, "evil2", "attack2"))
	denied(put(bob, carolB, "evil2", "attack2"))
	denied(put(carol, aliceB, "evil2", "attack2"))

	// 4. Create 4th tenant dave AFTER failover
	for _, st := range []string{mainKey, f1Key} {
		require.NoError(t, direct[st].MakeBucket(ctx, daveB, mclient.MakeBucketOptions{}))
	}
	dave := newUser("dave", "iso-dave-*")

	require.Eventually(t, func() bool {
		return put(dave, daveB, "obj1", "dave-v1") == nil
	}, 20*time.Second, 200*time.Millisecond)

	// Dave routes to the storage group's current primary (f1Key)
	require.True(t, existsDirect(t, f1Key, daveB, "obj1"))

	// Dave cannot access alice, bob, carol
	denied(put(dave, aliceB, "dave-evil", "attack"))
	denied(put(dave, bobB, "dave-evil", "attack"))
	denied(put(alice, daveB, "alice-evil", "attack"))
}
