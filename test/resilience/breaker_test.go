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

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

// waitBreakerOpen polls until the proxy answers 503 for the storage within a fraction of the HTTP timeout.
func waitBreakerOpen(t *testing.T, e *testEnv, bucket, key string) {
	t.Helper()
	require.Eventually(t, func() bool {
		code, took, err := e.presignedHead(t, bucket, key, time.Second)
		return err == nil && code == http.StatusServiceUnavailable && took < time.Second
	}, e.WaitLong, 100*time.Millisecond, "breaker did not open")
}

// A blackholed storage (connections accepted, never answered) must fail fast once its circuit breaker is open,
// instead of making every request wait for the HTTP timeout (5s here), and recover when the storage is back.
func TestCircuitBreaker_BlackholedStorageFailsFastAndRecovers(t *testing.T) {
	e := startEnv(t, envOpts{forwarded: []string{mainKey}})
	bucket := bucketName("breaker")
	key := "obj"
	require.NoError(t, direct[mainKey].MakeBucket(t.Context(), bucket, mclient.MakeBucketOptions{}))
	code, _, body := e.presignedPut(t, bucket, key, "payload")
	require.Equal(t, http.StatusOK, code, body)

	// healthy: all storages are reported available
	require.Eventually(t, func() bool {
		st := e.proxyStatus(t)
		if st == nil {
			return false
		}
		for _, name := range []string{mainKey, f1Key, f2Key} {
			if ok, found := storageAvailable(st, name); !found || !ok {
				return false
			}
		}
		return true
	}, e.WaitLong, 300*time.Millisecond)

	e.fwd[mainKey].setMode(modeBlackhole)
	waitBreakerOpen(t, e, bucket, key)

	// the breaker is open: every kind of request is rejected immediately
	for range 5 {
		start := time.Now()
		code, _, body := e.presignedPut(t, bucket, "other", "x")
		took := time.Since(start)
		require.Equal(t, http.StatusServiceUnavailable, code, body)
		require.Contains(t, body, "ServiceUnavailable")
		require.Less(t, took, time.Second, "must fail fast, not wait for the http timeout")
	}
	code, took, err := e.presignedHead(t, bucket, key, 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, code)
	require.Less(t, took, time.Second)

	// the proxy status published for the UI/API shows the unavailable storage and the healthy ones
	require.Eventually(t, func() bool {
		st := e.proxyStatus(t)
		if st == nil {
			return false
		}
		mainOK, _ := storageAvailable(st, mainKey)
		f1OK, _ := storageAvailable(st, f1Key)
		return !mainOK && f1OK
	}, e.WaitLong, 500*time.Millisecond, "proxy status must report main as unavailable")

	// recovery
	e.fwd[mainKey].setMode(modeForward)
	require.Eventually(t, func() bool {
		code, _, err := e.presignedHead(t, bucket, key, 2*time.Second)
		return err == nil && code == http.StatusOK
	}, e.WaitLong, 300*time.Millisecond, "storage must be used again after it recovered")
	code, _, body = e.presignedPut(t, bucket, "after", "ok")
	require.Equal(t, http.StatusOK, code, body)
}

// A storage that resets connections (up but not serving) opens the breaker too.
func TestCircuitBreaker_ConnectionResetOpensBreaker(t *testing.T) {
	e := startEnv(t, envOpts{forwarded: []string{mainKey}})
	bucket := bucketName("breaker-reset")
	require.NoError(t, direct[mainKey].MakeBucket(t.Context(), bucket, mclient.MakeBucketOptions{}))
	code, _, body := e.presignedPut(t, bucket, "obj", "payload")
	require.Equal(t, http.StatusOK, code, body)

	e.fwd[mainKey].setMode(modeReset)
	waitBreakerOpen(t, e, bucket, "obj")
	e.fwd[mainKey].setMode(modeForward)
	require.Eventually(t, func() bool {
		code, _, err := e.presignedHead(t, bucket, "obj", 2*time.Second)
		return err == nil && code == http.StatusOK
	}, e.WaitLong, 300*time.Millisecond)
}
