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
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	mclient "github.com/minio/minio-go/v7"
	mcredentials "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
	"github.com/hieunyit/tutti/test/app"
)

// Forwarder modes.
const (
	modeForward int = iota
	// modeBlackhole accepts connections and never answers (a host that silently drops packets).
	modeBlackhole
	// modeReset accepts connections and closes them at once (a host that is up but not serving).
	modeReset
)

// forwarder is a TCP forwarder in front of a real MinIO that can be switched to behave like a dead host.
type forwarder struct {
	ln     net.Listener
	target string

	mu    sync.Mutex
	mode  int
	conns map[net.Conn]struct{}
}

func newForwarder(t testing.TB, target string) *forwarder {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := &forwarder{ln: ln, target: target, conns: map[net.Conn]struct{}{}}
	go f.serve()
	t.Cleanup(func() {
		_ = ln.Close()
		f.closeAll()
	})
	return f
}

func (f *forwarder) addr() string { return f.ln.Addr().String() }

// setMode switches the mode and drops all open connections, so keep-alive connections of the client do not survive it.
func (f *forwarder) setMode(mode int) {
	f.mu.Lock()
	f.mode = mode
	f.mu.Unlock()
	f.closeAll()
}

func (f *forwarder) closeAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for c := range f.conns {
		_ = c.Close()
	}
	f.conns = map[net.Conn]struct{}{}
}

func (f *forwarder) serve() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		mode := f.mode
		if mode == modeReset {
			f.mu.Unlock()
			_ = c.Close()
			continue
		}
		f.conns[c] = struct{}{}
		f.mu.Unlock()
		if mode == modeBlackhole {
			continue // hold the connection open, never read or answer
		}
		up, err := net.Dial("tcp", f.target)
		if err != nil {
			_ = c.Close()
			continue
		}
		f.mu.Lock()
		f.conns[up] = struct{}{}
		f.mu.Unlock()
		go func() { _, _ = io.Copy(up, c); _ = up.Close() }()
		go func() { _, _ = io.Copy(c, up); _ = c.Close() }()
	}
}

// failingRedis is a miniredis whose write path used by replication event recording (Lua scripts of the queue) can be broken.
type failingRedis struct {
	*miniredis.Miniredis
	mu   sync.Mutex
	fail bool
}

func newFailingRedis(t testing.TB) *failingRedis {
	t.Helper()
	r := &failingRedis{Miniredis: miniredis.RunT(t)}
	r.Server().SetPreHook(func(c *server.Peer, cmd string, _ ...string) bool {
		r.mu.Lock()
		fail := r.fail
		r.mu.Unlock()
		switch strings.ToLower(cmd) {
		case "eval", "evalsha":
			if fail {
				c.WriteError("OOM command not allowed when used memory > 'maxmemory'.")
				return true
			}
		}
		return false
	})
	return r
}

func (r *failingRedis) setFailing(v bool) {
	r.mu.Lock()
	r.fail = v
	r.mu.Unlock()
}

// testEnv is a running proxy + worker over the MinIO storages.
type testEnv struct {
	app.Chorus
	// fwd are the forwarders of the storages that the proxy reaches through a forwarder.
	fwd map[string]*forwarder
}

type envOpts struct {
	redis *failingRedis
	// proxyHook mutates the proxy config before start.
	proxyHook  func(c *proxy.Config)
	workerHook func(c *worker.Config)
	// dynamicCreds enables dynamic credentials with encryption (master password) in proxy and worker.
	dynamicCreds bool
	// forwarded lists storages the proxy reaches through a forwarder.
	forwarded []string
}

func bucketName(name string) string { return name + "-" + xid.New().String() }

// startEnv starts proxy and worker. The proxy uses short health intervals; the worker always uses the real addresses.
func startEnv(t *testing.T, o envOpts) *testEnv {
	t.Helper()
	r := require.New(t)
	te := &testEnv{fwd: map[string]*forwarder{}}

	pStorages := map[string]s3.Storage{}
	for name, st := range storages {
		cp := st
		if slicesContains(o.forwarded, name) {
			f := newForwarder(t, strings.TrimPrefix(st.Address, "http://"))
			te.fwd[name] = f
			cp.Address = "http://" + f.addr()
		}
		cp.HealthCheckInterval = 500 * time.Millisecond
		cp.HttpTimeout = 5 * time.Second
		pStorages[name] = cp
	}
	pc, err := app.DeepCopyStruct(proxyConf)
	r.NoError(err)
	pc.Storage = app.ProxyS3Config(mainKey, pStorages)
	pc.Auth.UseStorage = mainKey
	pc.Health.FailureThreshold = 2
	pc.Health.OpenDuration = 3 * time.Second
	pc.Replication.EventRetryBudget = 300 * time.Millisecond
	if o.proxyHook != nil {
		o.proxyHook(pc)
	}
	wc, err := app.DeepCopyStruct(workerConf)
	r.NoError(err)
	wc.Storage = app.WorkerS3Config(mainKey, storages)
	if o.workerHook != nil {
		o.workerHook(wc)
	}
	if o.dynamicCreds {
		dc := objstore.DynamicCredentialsConfig{Enabled: true, MasterPassword: "test-master-password", PollInterval: time.Second}
		pc.Storage.DynamicCredentials = dc
		wc.Storage.DynamicCredentials = dc
	}

	var redisAddr string
	if o.redis != nil {
		redisAddr = o.redis.Addr()
	} else {
		redisAddr = miniredis.RunT(t).Addr()
	}
	te.Chorus = app.SetupChorusRedis(t, wc, pc, redisAddr)
	return te
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (e *testEnv) proxyClient(t testing.TB, opts ...func(*mclient.Options)) *mclient.Client {
	t.Helper()
	// fixed region: no bucket location lookup through the proxy (it would fail while main is down)
	o := &mclient.Options{Creds: mcredentials.NewStaticV4(mainCreds.AccessKeyID, mainCreds.SecretAccessKey, ""), Region: "us-east-1"}
	for _, f := range opts {
		f(o)
	}
	c, err := mclient.New(e.ProxyAddr, o)
	require.NoError(t, err)
	return c
}

// replicateBucket creates the bucket on the main storage and a bucket replication main -> to, and waits for its initial sync.
func (e *testEnv) replicateBucket(t *testing.T, bucket string, to ...string) {
	t.Helper()
	ctx := t.Context()
	require.NoError(t, direct[mainKey].MakeBucket(ctx, bucket, mclient.MakeBucketOptions{}))
	for _, dst := range to {
		id := &pb.ReplicationID{User: user, FromStorage: mainKey, ToStorage: dst, FromBucket: &bucket, ToBucket: &bucket}
		_, err := e.PolicyClient.AddReplication(ctx, &pb.AddReplicationRequest{Id: id})
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			repl, err := e.PolicyClient.GetReplication(ctx, id)
			return err == nil && repl.IsInitDone
		}, e.WaitLong, e.RetryLong, "initial sync main->%s did not finish", dst)
	}
}

func replID(bucket, to string) *pb.ReplicationID {
	return &pb.ReplicationID{User: user, FromStorage: mainKey, ToStorage: to, FromBucket: &bucket, ToBucket: &bucket}
}

// presignedPut uploads content through a presigned URL and returns the raw response (no client retries).
func (e *testEnv) presignedPut(t testing.TB, bucket, key, content string) (int, http.Header, string) {
	t.Helper()
	u, err := e.proxyClient(t).PresignedPutObject(t.Context(), bucket, key, 10*time.Minute)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, u.String(), strings.NewReader(content))
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, res.Header, string(body)
}

// presignedStat issues a presigned HEAD through the proxy and returns the status code.
func (e *testEnv) presignedHead(t testing.TB, bucket, key string, timeout time.Duration) (int, time.Duration, error) {
	t.Helper()
	u, err := e.proxyClient(t).PresignedHeadObject(t.Context(), bucket, key, 10*time.Minute, nil)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, u.String(), nil)
	require.NoError(t, err)
	start := time.Now()
	res, err := (&http.Client{Timeout: timeout}).Do(req)
	took := time.Since(start)
	if err != nil {
		return 0, took, err
	}
	defer res.Body.Close()
	return res.StatusCode, took, nil
}

func readObject(t testing.TB, c *mclient.Client, bucket, key string) (string, error) {
	t.Helper()
	obj, err := c.GetObject(t.Context(), bucket, key, mclient.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	b, err := io.ReadAll(obj)
	return string(b), err
}

func existsDirect(t testing.TB, storage, bucket, key string) bool {
	t.Helper()
	_, err := direct[storage].StatObject(t.Context(), bucket, key, mclient.StatObjectOptions{})
	if err == nil {
		return true
	}
	code := mclient.ToErrorResponse(err).Code
	require.Contains(t, []string{"NoSuchKey", "NoSuchBucket"}, code, "unexpected stat error: %v", err)
	return false
}

// restJSON calls the REST gateway of the worker and decodes the JSON answer.
func (e *testEnv) restJSON(t testing.TB, method, path string, body any) map[string]any {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, e.UrlHttpApi+path, rd)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode, string(b))
	out := map[string]any{}
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

// proxyStatus returns the REST GetProxyStatus document of the first proxy, or nil.
func (e *testEnv) proxyStatus(t testing.TB) map[string]any {
	t.Helper()
	res := e.restJSON(t, http.MethodGet, "/tutti/proxies", nil)
	list, _ := res["proxies"].([]any)
	if len(list) == 0 {
		return nil
	}
	return list[0].(map[string]any)
}

// storageAvailable reads the availability of a storage from the proxy status document.
func storageAvailable(status map[string]any, name string) (available bool, found bool) {
	list, _ := status["storages"].([]any)
	for _, s := range list {
		m := s.(map[string]any)
		if m["name"] == name {
			a, _ := m["available"].(bool)
			return a, true
		}
	}
	return false, false
}
