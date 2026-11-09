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

	"github.com/hieunyit/tutti/service/proxy"
)

// A write that succeeded on the main storage but whose replication event could not be recorded must not be
// acknowledged (503), so the client retries; the retry records the event and the follower gets the object.
func TestEventRecordingFailure_Returns503ThenRetrySucceeds(t *testing.T) {
	redis := newFailingRedis(t)
	e := startEnv(t, envOpts{redis: redis, proxyHook: func(c *proxy.Config) { c.Replication.OnEventError = "fail" }})
	bucket := bucketName("event-fail")
	e.replicateBucket(t, bucket, f1Key)

	// redis rejects the writes that enqueue replication tasks (like OOM with maxmemory + noeviction)
	redis.setFailing(true)
	code, _, body := e.presignedPut(t, bucket, "obj", "payload")
	require.Equal(t, http.StatusServiceUnavailable, code, body)
	require.Contains(t, body, "ServiceUnavailable")
	// documented behavior: the change itself was applied on the main storage
	require.True(t, existsDirect(t, mainKey, bucket, "obj"))

	redis.setFailing(false)
	code, _, body = e.presignedPut(t, bucket, "obj", "payload")
	require.Equal(t, http.StatusOK, code, body)
	require.Eventually(t, func() bool {
		got, err := readObject(t, direct[f1Key], bucket, "obj")
		return err == nil && got == "payload"
	}, e.WaitLong, e.RetryLong, "retried write must be replicated to the follower")
}

// With onEventError=log the legacy behavior is kept: the storage response is returned although the event is lost.
func TestEventRecordingFailure_LogModeAcknowledges(t *testing.T) {
	redis := newFailingRedis(t)
	e := startEnv(t, envOpts{redis: redis, proxyHook: func(c *proxy.Config) { c.Replication.OnEventError = "log" }})
	bucket := bucketName("event-log")
	e.replicateBucket(t, bucket, f1Key)

	redis.setFailing(true)
	code, _, body := e.presignedPut(t, bucket, "obj", "payload")
	require.Equal(t, http.StatusOK, code, body)
}

func syncOn(quorumFailure string, timeout time.Duration) func(c *proxy.Config) {
	return func(c *proxy.Config) {
		c.Replication.Sync = proxy.SyncConfig{Enabled: true, Timeout: timeout, OnQuorumFailure: quorumFailure}
	}
}

// After PUT/DELETE returned, the followers must already have the change and the header reports the copies.
func TestSyncReplication_ImmediateCopiesAndHeader(t *testing.T) {
	e := startEnv(t, envOpts{proxyHook: syncOn("fail", 20*time.Second)})
	bucket := bucketName("sync-ok")
	e.replicateBucket(t, bucket, f1Key, f2Key)

	code, hdr, body := e.presignedPut(t, bucket, "obj", "sync payload")
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "3/3", hdr.Get("X-Tutti-Replicas"))
	// no polling: the copies exist as soon as the proxy answered
	for _, st := range []string{mainKey, f1Key, f2Key} {
		got, err := readObject(t, direct[st], bucket, "obj")
		require.NoError(t, err, st)
		require.Equal(t, "sync payload", got, st)
	}
}

func TestSyncReplication_FollowerUnreachable(t *testing.T) {
	t.Run("quorum-fail-returns-503-then-recovers", func(t *testing.T) {
		e := startEnv(t, envOpts{forwarded: []string{f1Key}, proxyHook: syncOn("fail", time.Second)})
		bucket := bucketName("sync-fail")
		e.replicateBucket(t, bucket, f1Key)

		code, hdr, body := e.presignedPut(t, bucket, "healthy", "x")
		require.Equal(t, http.StatusOK, code, body)
		require.Equal(t, "2/2", hdr.Get("X-Tutti-Replicas"))

		e.fwd[f1Key].setMode(modeBlackhole)
		code, _, body = e.presignedPut(t, bucket, "down", "y")
		require.Equal(t, http.StatusServiceUnavailable, code, body)
		// written to main, and the safety-net task replicates it once the worker (which reaches f1 directly) runs
		require.True(t, existsDirect(t, mainKey, bucket, "down"))
		require.Eventually(t, func() bool { return existsDirect(t, f1Key, bucket, "down") }, e.WaitLong, e.RetryLong)

		// when the follower is back (breaker closes after openDuration), the guarantee is given again
		e.fwd[f1Key].setMode(modeForward)
		require.Eventually(t, func() bool {
			code, hdr, _ := e.presignedPut(t, bucket, "recovered", "z")
			return code == http.StatusOK && hdr.Get("X-Tutti-Replicas") == "2/2"
		}, e.WaitLong, 300*time.Millisecond)
		require.True(t, existsDirect(t, f1Key, bucket, "recovered"))
	})

	t.Run("quorum-accept-returns-200-with-partial-count", func(t *testing.T) {
		e := startEnv(t, envOpts{forwarded: []string{f1Key}, proxyHook: syncOn("accept", time.Second)})
		bucket := bucketName("sync-accept")
		e.replicateBucket(t, bucket, f1Key)

		e.fwd[f1Key].setMode(modeBlackhole)
		code, hdr, body := e.presignedPut(t, bucket, "obj", "y")
		require.Equal(t, http.StatusOK, code, body)
		require.Equal(t, "1/2", hdr.Get("X-Tutti-Replicas"))
		require.True(t, existsDirect(t, mainKey, bucket, "obj"))
	})
}
