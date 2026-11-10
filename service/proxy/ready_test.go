// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/policy"
)

func TestReadiness(t *testing.T) {
	st := policy.SnapshotStatus{Loaded: true, Version: "7", RedisReachable: true}
	storages := 2
	rd := &readiness{
		snapshot:     func() policy.SnapshotStatus { return st },
		storages:     func() int { return storages },
		maxStaleness: 30 * time.Second,
	}
	var nextCalls int
	h := rd.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { nextCalls++; w.WriteHeader(http.StatusTeapot) }))

	get := func(path string, mod func(*http.Request)) (*httptest.ResponseRecorder, readyResponse) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if mod != nil {
			mod(req)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var body readyResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec, body
	}

	rec, body := get("/ready", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.True(t, body.Ready)
	require.Equal(t, "ok", body.Mode)

	// redis down, snapshot fresh: ready for reads
	st = policy.SnapshotStatus{Loaded: true, RedisReachable: false, Age: 10 * time.Second}
	rec, body = get("/ready", nil)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "read-only", body.Mode)

	// redis down for too long
	st.Age = time.Minute
	rec, body = get("/ready", nil)
	require.Equal(t, 503, rec.Code)
	require.False(t, body.Ready)
	require.NotEmpty(t, body.Reasons)

	// snapshot not loaded
	st = policy.SnapshotStatus{}
	rec, body = get("/ready", nil)
	require.Equal(t, 503, rec.Code)
	require.Contains(t, body.Reasons[0], "not loaded")

	// no storages
	st = policy.SnapshotStatus{Loaded: true, RedisReachable: true}
	storages = 0
	rec, _ = get("/ready", nil)
	require.Equal(t, 503, rec.Code)
	storages = 1

	// draining
	rd.draining.Store(true)
	rec, body = get("/ready", nil)
	require.Equal(t, 503, rec.Code)
	require.True(t, body.Draining)
	rd.draining.Store(false)

	require.Equal(t, 0, nextCalls, "readiness never reaches the S3 chain")

	// signed / presigned requests to a bucket named "ready" and other paths go to the proxy
	rec, _ = get("/ready", func(r *http.Request) { r.Header.Set("Authorization", "AWS4-HMAC-SHA256 ...") })
	require.Equal(t, http.StatusTeapot, rec.Code)
	rec, _ = get("/ready?X-Amz-Signature=abc", nil)
	require.Equal(t, http.StatusTeapot, rec.Code)
	rec, _ = get("/ready/key", nil)
	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, 3, nextCalls)
}

func TestConfigReadyAndSnapshotDefaultsValidate(t *testing.T) {
	c := Config{}
	require.NoError(t, c.validateReady())
	c.DrainDelay = -1
	require.Error(t, c.validateReady())
}
