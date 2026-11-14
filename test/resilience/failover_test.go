/*
 * Copyright © 2026 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package resilience

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/service/proxy"
)

func failoverOn(c *proxy.Config) {
	c.Failover = proxy.FailoverConfig{Enabled: true, After: time.Second, CheckInterval: 200 * time.Millisecond}
}

func (e *testEnv) failoverEvents(t testing.TB) []any {
	t.Helper()
	res := e.restJSON(t, http.MethodGet, "/tutti/failover/events", nil)
	events, _ := res["events"].([]any)
	return events
}

func (e *testEnv) setPolicy(t testing.TB, source string, enabled bool, targets ...string) {
	t.Helper()
	e.restJSON(t, http.MethodPost, "/tutti/failover/policies", map[string]any{
		"source": source, "enabled": enabled, "targets": targets, "after": "1s",
	})
}

func (e *testEnv) routeOf(t testing.TB, bucket string) string {
	t.Helper()
	res, err := e.PolicyClient.TestProxy(t.Context(), &pb.TestProxyRequest{User: user, Bucket: bucket})
	require.NoError(t, err)
	return res.RouteToStorage
}

// waitDrained waits until the replication has no unprocessed events (failover requires zero lag by default).
func (e *testEnv) waitDrained(t *testing.T, bucket, to string) {
	t.Helper()
	require.Eventually(t, func() bool {
		repl, err := e.PolicyClient.GetReplication(t.Context(), replID(bucket, to))
		return err == nil && repl.IsInitDone && repl.EventsDone == repl.Events
	}, e.WaitLong, e.RetryLong)
}

func (e *testEnv) requireNoFailoverFor(t *testing.T, bucket string, d time.Duration) {
	t.Helper()
	require.Never(t, func() bool {
		return len(e.failoverEvents(t)) != 0 || e.routeOf(t, bucket) != mainKey
	}, d, 200*time.Millisecond)
}

// The whole chain: no policy -> never; disabled policy -> never; policy set through the REST API -> the proxy
// controller switches routing to the listed target (not to the other healthy replica), pauses the replication
// from the failed storage and records an event.
func TestFailover_PolicyViaAPIDrivesAutomaticFailover(t *testing.T) {
	e := startEnv(t, envOpts{forwarded: []string{mainKey}, proxyHook: failoverOn})
	bucket := bucketName("failover")
	e.replicateBucket(t, bucket, f1Key, f2Key)

	code, _, body := e.presignedPut(t, bucket, "before", "data before the outage")
	require.Equal(t, http.StatusOK, code, body)
	e.waitDrained(t, bucket, f1Key)
	e.waitDrained(t, bucket, f2Key)
	for _, st := range []string{f1Key, f2Key} {
		require.Eventually(t, func() bool { return existsDirect(t, st, bucket, "before") }, e.WaitLong, e.RetryLong)
	}

	// the proxy publishes its state and effective settings through the resilience API
	var status map[string]any
	require.Eventually(t, func() bool { status = e.proxyStatus(t); return status != nil }, e.WaitLong, 200*time.Millisecond)
	settings := status["settings"].(map[string]any)
	require.Equal(t, true, settings["failover"].(map[string]any)["enabled"])
	require.Equal(t, float64(2), settings["health"].(map[string]any)["failureThreshold"])
	require.NotEmpty(t, status["storages"])

	e.fwd[mainKey].setMode(modeBlackhole)
	waitBreakerOpen(t, e, bucket, "before")

	// 1. storage without policy never fails over (3x the configured "after")
	e.requireNoFailoverFor(t, bucket, 3*time.Second)
	// 2. neither does a storage whose policy is disabled
	e.setPolicy(t, mainKey, false, f2Key)
	e.requireNoFailoverFor(t, bucket, 2500*time.Millisecond)

	// 3. enabling the policy via the API: only f2 may be promoted, although f1 has a healthy replication too
	e.setPolicy(t, mainKey, true, f2Key)
	require.Eventually(t, func() bool { return len(e.failoverEvents(t)) == 1 }, e.WaitLong, 200*time.Millisecond)
	ev := e.failoverEvents(t)[0].(map[string]any)
	require.Equal(t, mainKey, ev["from"])
	require.Equal(t, f2Key, ev["to"])
	require.Equal(t, user, ev["user"])
	require.Equal(t, bucket, ev["bucket"])

	require.Equal(t, f2Key, e.routeOf(t, bucket), "routing must be switched to the policy target")
	paused, err := e.PolicyClient.GetReplication(t.Context(), replID(bucket, f2Key))
	require.NoError(t, err)
	require.True(t, paused.IsPaused, "replication from the failed storage to the promoted one must be paused")
	other, err := e.PolicyClient.GetReplication(t.Context(), replID(bucket, f1Key))
	require.NoError(t, err)
	require.False(t, other.IsPaused)

	// the data is served from the follower
	pc := e.proxyClient(t)
	require.Eventually(t, func() bool {
		got, err := readObject(t, pc, bucket, "before")
		return err == nil && got == "data before the outage"
	}, e.WaitLong, 200*time.Millisecond)

	// writes keep working after failover with another active replication from the failed
	// storage (main->f1): the request context only carries replications from the routed
	// storage, so recording events for main->f1 must not fail the write (regression: 503)
	putStatus, _, putBody := e.presignedPut(t, bucket, "after-failover", "written after failover")
	require.Equal(t, http.StatusOK, putStatus, putBody)
	require.True(t, existsDirect(t, f2Key, bucket, "after-failover"), "write must land on the promoted storage")
	require.Never(t, func() bool { return existsDirect(t, f1Key, bucket, "after-failover") },
		2*time.Second, 200*time.Millisecond, "main->f1 must not replicate writes made on f2")
	require.Len(t, e.failoverEvents(t), 1, "failover happens once")
}

// Only the targets listed in the policy may be chosen, and only when they have a replication from the source.
func TestFailover_UnlistedTargetNeverChosen(t *testing.T) {
	e := startEnv(t, envOpts{forwarded: []string{mainKey}, proxyHook: failoverOn})
	bucket := bucketName("failover-unlisted")
	e.replicateBucket(t, bucket, f1Key) // replication only to f1
	code, _, body := e.presignedPut(t, bucket, "obj", "data")
	require.Equal(t, http.StatusOK, code, body)
	e.waitDrained(t, bucket, f1Key)

	// f2 is the only listed target but main has no replication to it
	e.setPolicy(t, mainKey, true, f2Key)
	e.fwd[mainKey].setMode(modeBlackhole)
	waitBreakerOpen(t, e, bucket, "obj")
	e.requireNoFailoverFor(t, bucket, 3*time.Second)

	// listing f1 makes the same controller promote it
	e.setPolicy(t, mainKey, true, f2Key, f1Key)
	require.Eventually(t, func() bool { return len(e.failoverEvents(t)) == 1 }, e.WaitLong, 200*time.Millisecond)
	require.Equal(t, f1Key, e.failoverEvents(t)[0].(map[string]any)["to"])
	require.Equal(t, f1Key, e.routeOf(t, bucket))

	// writes now go to the promoted storage; the replication towards it is paused/dropped, so they are acknowledged
	code, _, body = e.presignedPut(t, bucket, "after", "written after failover")
	require.Equal(t, http.StatusOK, code, body)
	require.True(t, existsDirect(t, f1Key, bucket, "after"))
	require.False(t, existsDirect(t, mainKey, bucket, "after"))
	require.False(t, existsDirect(t, f2Key, bucket, "after"))
}
