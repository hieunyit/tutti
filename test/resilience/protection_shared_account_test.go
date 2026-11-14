// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package resilience

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/s3"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
)

// Users on one shared connection account: alice owns a protection group on [f1, f2] for one bucket. bob is routed
// to main (not a member) and must keep writing there with his own (empty) replication set: never alice's
// routing or replications. carol, routed to the group's primary, must not write the managed bucket at all.
// Legacy failover policies and groups never manage the same storages.
func TestProtectionGroup_SharedAccountUsersNeverInheritGroup(t *testing.T) {
	ctx := t.Context()
	accounts := map[string]s3.CredentialsV4{}
	for i, key := range []string{mainKey, f1Key, f2Key} {
		accounts[key] = createAccount(t, minioAdmin(t, key), "tutti-pgs", "tutti-pgs-secret-"+string(rune('1'+i)), "tutti-pgs-rw", tuttiRW)
	}
	shared := func(sts map[string]*s3.Storage) {
		for key, st := range sts {
			acc := accounts[key]
			st.Credentials, st.ConnectionAccount = nil, &acc
		}
	}
	e := startEnv(t, envOpts{dynamicCreds: true,
		proxyHook: func(c *proxy.Config) {
			c.Status.Interval = 500 * time.Millisecond
			sts := map[string]*s3.Storage{}
			for k, v := range c.Storage.Storages {
				sts[k] = v.S3
			}
			shared(sts)
		},
		workerHook: func(c *worker.Config) {
			sts := map[string]*s3.Storage{}
			for k, v := range c.Storage.Storages {
				sts[k] = v.S3
			}
			shared(sts)
		},
	})
	newUser := func(name string) *mclient.Client {
		res := e.restJSON(t, http.MethodPost, "/tutti/users", map[string]any{"name": name, "createAccessKey": true,
			"policy": map[string]any{"permission": accesspolicy.ReadWrite, "buckets": []string{"pgs-*"}}})
		key := res["accessKey"].(map[string]any)
		return e.keyClient(t, key["accessKeyId"].(string), key["secretAccessKey"].(string))
	}
	alice, bob, carol := newUser("alice"), newUser("bob"), newUser("carol")

	suffix := bucketName("")
	managed, carolB := "pgs-managed"+suffix, "pgs-carol"+suffix
	t.Cleanup(func() {
		for _, b := range []string{managed, carolB} {
			for _, st := range []string{mainKey, f1Key, f2Key} {
				removeBucket(t, st, b)
			}
		}
	})
	for _, st := range []string{f1Key, f2Key} {
		require.NoError(t, direct[st].MakeBucket(ctx, managed, mclient.MakeBucketOptions{}))
	}
	for _, u := range []string{"alice", "carol"} {
		_, err := e.PolicyClient.AddRouting(ctx, &pb.AddRoutingRequest{User: u, ToStorage: f1Key})
		require.NoError(t, err)
	}

	group := map[string]any{"id": "alice-g", "user": "alice", "buckets": []string{managed}, "members": []string{f1Key, f2Key}, "copies": 2}
	// an enabled legacy failover policy on a member blocks the group, and the other way round
	e.setPolicy(t, f1Key, true, f2Key)
	e.restStatus(t, http.MethodPost, "/tutti/protection-groups", map[string]any{"group": group}, http.StatusBadRequest)
	e.setPolicy(t, f1Key, false, f2Key)
	// a group needs an explicit user
	e.restStatus(t, http.MethodPost, "/tutti/protection-groups", map[string]any{"group": map[string]any{
		"id": "no-user", "buckets": []string{managed}, "members": []string{f1Key, f2Key}, "copies": 2}}, http.StatusBadRequest)
	e.restJSON(t, http.MethodPost, "/tutti/protection-groups", map[string]any{"group": group})
	require.Eventually(t, func() bool {
		res := e.restJSON(t, http.MethodGet, "/tutti/protection-groups/alice-g", nil)
		g := res["group"].(map[string]any)
		return g["state"] == "ready" && g["frozen"] != true
	}, 90*time.Second, 250*time.Millisecond, "group did not become ready")
	body, err := json.Marshal(map[string]any{"source": f1Key, "enabled": true, "targets": []string{f2Key}, "after": "1s"})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.UrlHttpApi+"/tutti/failover/policies", bytes.NewReader(body))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "a legacy policy over a group member must be refused")

	put := func(c *mclient.Client, bucket, key string) error {
		_, err := c.PutObject(ctx, bucket, key, strings.NewReader(key), int64(len(key)), mclient.PutObjectOptions{})
		return err
	}
	// alice writes through her group: primary f1, copied to f2
	require.Eventually(t, func() bool { return put(alice, managed, "alice-obj") == nil }, 30*time.Second, 250*time.Millisecond)
	require.True(t, existsDirect(t, f1Key, managed, "alice-obj"))
	require.Eventually(t, func() bool { return existsDirect(t, f2Key, managed, "alice-obj") }, 60*time.Second, 250*time.Millisecond)

	// bob keeps his own routing (main) and replications (none): his writes never reach the group members
	res, err := e.PolicyClient.TestProxy(ctx, &pb.TestProxyRequest{User: "bob", Bucket: managed})
	require.NoError(t, err)
	require.Equal(t, mainKey, res.RouteToStorage)
	require.Empty(t, res.Replications, "bob must not use alice's replications")
	require.Eventually(t, func() bool {
		return bob.MakeBucket(ctx, managed, mclient.MakeBucketOptions{Region: "us-east-1"}) == nil
	}, 20*time.Second, 200*time.Millisecond)
	require.NoError(t, put(bob, managed, "bob-obj"))
	require.True(t, existsDirect(t, mainKey, managed, "bob-obj"))
	require.Never(t, func() bool {
		return existsDirect(t, f1Key, managed, "bob-obj") || existsDirect(t, f2Key, managed, "bob-obj")
	}, 3*time.Second, 250*time.Millisecond, "bob's write must not be routed or replicated like alice's")

	// carol reaches the group's primary through the shared account: the managed bucket is read-only for her
	err = put(carol, managed, "carol-obj")
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, mclient.ToErrorResponse(err).StatusCode, "%v", err)
	require.False(t, existsDirect(t, f1Key, managed, "carol-obj"))
	_, err = readObject(t, carol, managed, "alice-obj")
	require.NoError(t, err, "reads stay allowed")
	// her other buckets on the same storage are not managed
	require.Eventually(t, func() bool {
		return carol.MakeBucket(ctx, carolB, mclient.MakeBucketOptions{Region: "us-east-1"}) == nil
	}, 20*time.Second, 200*time.Millisecond)
	require.NoError(t, put(carol, carolB, "obj"))
	require.True(t, existsDirect(t, f1Key, carolB, "obj"))
}

// With the policy snapshot disabled, the legacy failover controller still recognizes group-owned scopes (Redis
// lookup) and never fails them over, while it does fail over an unprotected scope of the same storage. The
// legacy policy is written behind the API (as left by an older version): the API refuses it.
func TestProtectionGroup_LegacyFailoverSkipsProtectedScopeWithoutSnapshot(t *testing.T) {
	ctx := t.Context()
	rdb := newFailingRedis(t)
	e := startEnv(t, envOpts{forwarded: []string{mainKey}, redis: rdb, proxyHook: func(c *proxy.Config) {
		failoverOn(c)
		c.Snapshot.Disabled = true
		c.Status.Interval = 500 * time.Millisecond
	}})
	protected, plain := bucketName("pg-legacy-protected"), bucketName("pg-legacy-plain")
	require.NoError(t, direct[mainKey].MakeBucket(ctx, protected, mclient.MakeBucketOptions{}))
	_, err := direct[mainKey].PutObject(ctx, protected, "obj", strings.NewReader("p"), 1, mclient.PutObjectOptions{})
	require.NoError(t, err)
	e.restJSON(t, http.MethodPost, "/tutti/protection-groups", map[string]any{"group": map[string]any{
		"id": "legacy-g", "user": user, "buckets": []string{protected}, "members": []string{mainKey, f1Key}, "copies": 2}})
	require.Eventually(t, func() bool {
		res := e.restJSON(t, http.MethodGet, "/tutti/protection-groups/legacy-g", nil)
		g := res["group"].(map[string]any)
		return g["state"] == "ready" && g["frozen"] != true
	}, 90*time.Second, 250*time.Millisecond, "group did not become ready")
	e.replicateBucket(t, plain, f1Key)
	code, _, body := e.presignedPut(t, plain, "obj", "data")
	require.Equal(t, http.StatusOK, code, body)
	e.waitDrained(t, plain, f1Key)

	// refused through the API, so plant it as an older version would have left it
	e.restStatus(t, http.MethodPost, "/tutti/failover/policies", map[string]any{"source": mainKey, "enabled": true, "targets": []string{f1Key}, "after": "1s"}, http.StatusBadRequest)
	raw, err := json.Marshal(map[string]any{"source": mainKey, "enabled": true, "targets": []string{f1Key}, "after": int64(time.Second)})
	require.NoError(t, err)
	rdb.DB(3).HSet("tutti:failover-policies", mainKey, string(raw))

	e.fwd[mainKey].setMode(modeBlackhole)
	waitBreakerOpen(t, e, plain, "obj")
	require.Eventually(t, func() bool { return e.routeOf(t, plain) == f1Key }, 30*time.Second, 200*time.Millisecond,
		"the unprotected scope fails over")
	require.Never(t, func() bool { return e.routeOf(t, protected) != mainKey }, 4*time.Second, 250*time.Millisecond,
		"the group-owned scope must never be failed over by the legacy controller")
	for _, ev := range e.failoverEvents(t) {
		require.NotEqual(t, protected, ev.(map[string]any)["bucket"])
	}
}
