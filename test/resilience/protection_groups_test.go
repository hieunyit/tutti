// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package resilience

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	mclient "github.com/minio/minio-go/v7"
	mcredentials "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/test/app"
)

// groupEnv is one worker plus several proxies with known IDs (the electorate of a protection group)
// on one Redis. Every proxy reaches the main storage through the same forwarder.
type groupEnv struct {
	*testEnv
	redis   *miniredis.Miniredis
	proxies map[string]string // app id -> host:port
	// redisFwd is the Redis forwarder of proxies started with a partitionable Redis connection.
	redisFwd map[string]*forwarder
}

func freePort(t testing.TB) (int, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return l.Addr().(*net.TCPAddr).Port, addr
}

func startGroupEnv(t *testing.T, plain []string, partitionable []string) *groupEnv {
	t.Helper()
	r := require.New(t)
	mr := miniredis.RunT(t)
	// miniredis only expires keys on FastForward: let TTLs (writer leases, status documents) follow real time
	ttlCtx, ttlStop := context.WithCancel(context.Background())
	t.Cleanup(ttlStop)
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ttlCtx.Done():
				return
			case <-tick.C:
				mr.FastForward(100 * time.Millisecond)
			}
		}
	}()
	ge := &groupEnv{testEnv: &testEnv{fwd: map[string]*forwarder{}}, redis: mr, proxies: map[string]string{}, redisFwd: map[string]*forwarder{}}

	wc, err := app.DeepCopyStruct(workerConf)
	r.NoError(err)
	wc.Storage = app.WorkerS3Config(mainKey, storages)
	ge.Chorus = app.SetupChorusRedis(t, wc, nil, mr.Addr())

	mainFwd := newForwarder(t, strings.TrimPrefix(storages[mainKey].Address, "http://"))
	ge.fwd[mainKey] = mainFwd
	pStorages := map[string]s3.Storage{}
	for name, st := range storages {
		cp := st
		if name == mainKey {
			cp.Address = "http://" + mainFwd.addr()
		}
		cp.HealthCheckInterval = 500 * time.Millisecond
		cp.HttpTimeout = 2 * time.Second
		pStorages[name] = cp
	}
	ctx, cancel := context.WithCancel(context.WithoutCancel(t.Context()))
	done := make(chan struct{})
	started := 0
	start := func(id string, redisAddr string) {
		pc, err := app.DeepCopyStruct(proxyConf)
		r.NoError(err)
		pc.Storage = app.ProxyS3Config(mainKey, pStorages)
		pc.Auth.UseStorage = mainKey
		pc.Health.FailureThreshold = 2
		pc.Health.OpenDuration = 30 * time.Second
		pc.Replication.EventRetryBudget = 300 * time.Millisecond
		pc.Status.Interval = 500 * time.Millisecond
		pc.ProtectionGroups.WriterLease = 3 * time.Second
		pc.DrainDelay = 0
		pc.Redis.Address = redisAddr
		pc.Port, pc.Address = freePort(t)
		r.NoError(pc.Validate())
		ge.proxies[id] = pc.Address
		started++
		go func() {
			defer func() { done <- struct{}{} }()
			if err := proxy.Start(ctx, dom.AppInfo{Version: "test", App: "proxy", AppID: id}, pc); err != nil && !errors.Is(err, context.Canceled) {
				t.Logf("proxy %s stopped: %v", id, err)
			}
		}()
	}
	for _, id := range plain {
		start(id, mr.Addr())
	}
	for _, id := range partitionable {
		f := newForwarder(t, mr.Addr())
		ge.redisFwd[id] = f
		start(id, f.addr())
	}
	t.Cleanup(func() {
		cancel()
		for i := 0; i < started; i++ {
			select {
			case <-done:
			case <-time.After(20 * time.Second):
				t.Log("proxy did not stop in time")
				return
			}
		}
	})
	ge.ProxyAddr = ge.proxies[plain[0]]
	// all proxies report and see every storage
	require.Eventually(t, func() bool {
		res := ge.restJSON(t, http.MethodGet, "/tutti/proxies", nil)
		list, _ := res["proxies"].([]any)
		return len(list) == len(plain)+len(partitionable)
	}, 30*time.Second, 200*time.Millisecond)
	return ge
}

func (e *groupEnv) clientOf(t testing.TB, proxyID string) *mclient.Client {
	t.Helper()
	c, err := mclient.New(e.proxies[proxyID], &mclient.Options{Creds: mcredentials.NewStaticV4(mainCreds.AccessKeyID, mainCreds.SecretAccessKey, ""), Region: "us-east-1"})
	require.NoError(t, err)
	return c
}

func (e *groupEnv) group(t testing.TB, id string) map[string]any {
	t.Helper()
	res := e.restJSON(t, http.MethodGet, "/tutti/protection-groups/"+id, nil)
	return res["group"].(map[string]any)
}

func (e *groupEnv) waitGroup(t *testing.T, id string, d time.Duration, cond func(g map[string]any) bool, msg string) map[string]any {
	t.Helper()
	var g map[string]any
	require.Eventually(t, func() bool {
		g = e.group(t, id)
		return cond(g)
	}, d, 250*time.Millisecond, msg)
	return g
}

func (e *groupEnv) createGroup(t *testing.T, group map[string]any) {
	t.Helper()
	e.restJSON(t, http.MethodPost, "/tutti/protection-groups", map[string]any{"group": group})
	e.waitGroup(t, group["id"].(string), 90*time.Second, func(g map[string]any) bool {
		return g["state"] == "ready" && g["frozen"] != true
	}, "group did not become ready")
}

// putEventually retries while the group is briefly frozen (503) during its final resync.
func putEventually(t *testing.T, c *mclient.Client, bucket, key, content string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := c.PutObject(t.Context(), bucket, key, strings.NewReader(content), int64(len(content)), mclient.PutObjectOptions{})
		return err == nil
	}, 60*time.Second, 250*time.Millisecond, "put %s/%s through the proxy never succeeded", bucket, key)
}

// Three storages, primary A (main) blackholed: the three-proxy electorate votes, the group fails over
// to B (f1) and re-points replication so C (f2) keeps receiving copies of new writes.
func TestProtectionGroup_FailoverRestoresCopies(t *testing.T) {
	e := startGroupEnv(t, []string{"p1", "p2", "p3"}, nil)
	ctx := t.Context()
	bucket := bucketName("pg-failover")
	require.NoError(t, direct[mainKey].MakeBucket(ctx, bucket, mclient.MakeBucketOptions{}))
	_, err := direct[mainKey].PutObject(ctx, bucket, "before", strings.NewReader("before"), 6, mclient.PutObjectOptions{})
	require.NoError(t, err)

	e.createGroup(t, map[string]any{"id": "pg1", "user": user, "buckets": []string{bucket}, "members": []string{mainKey, f1Key, f2Key},
		"copies": 2, "mode": "auto", "after": "1s", "electorate": []string{"p1", "p2", "p3"}})
	for _, st := range []string{f1Key, f2Key} {
		require.True(t, existsDirect(t, st, bucket, "before"), "group resync must copy existing data to %s", st)
	}
	c := e.clientOf(t, "p1")
	putEventually(t, c, bucket, "while-healthy", "v1")
	require.Eventually(t, func() bool { return existsDirect(t, f2Key, bucket, "while-healthy") }, 30*time.Second, 200*time.Millisecond)

	e.fwd[mainKey].setMode(modeBlackhole)
	g := e.waitGroup(t, "pg1", 90*time.Second, func(g map[string]any) bool { return g["primary"] == f1Key }, "no automatic failover")
	require.Equal(t, "2", fmt.Sprint(g["generation"]))
	require.Equal(t, f1Key, e.routeOf(t, bucket))

	// writes are admitted again (catch-up runs unfrozen) and land on B, then C receives a copy
	putEventually(t, c, bucket, "after-failover", "written on the new primary")
	require.True(t, existsDirect(t, f1Key, bucket, "after-failover"))
	require.Eventually(t, func() bool { return existsDirect(t, f2Key, bucket, "after-failover") }, 60*time.Second, 250*time.Millisecond,
		"replication from the new primary to the remaining member must be re-pointed")
	require.False(t, existsDirect(t, mainKey, bucket, "after-failover"))
	e.waitGroup(t, "pg1", 90*time.Second, func(g map[string]any) bool { return g["state"] == "ready" && g["frozen"] != true }, "group did not settle")
}

// A proxy that dies (here: partitioned from Redis) in the middle of a protected write leaves a writer
// record behind. Promotion must still complete once its lease expired and it stopped reporting.
func TestProtectionGroup_PromotionCompletesAfterDeadWriterLease(t *testing.T) {
	e := startGroupEnv(t, []string{"p1"}, []string{"p-dead"})
	ctx := t.Context()
	bucket := bucketName("pg-lease")
	require.NoError(t, direct[mainKey].MakeBucket(ctx, bucket, mclient.MakeBucketOptions{}))
	e.createGroup(t, map[string]any{"id": "pg2", "user": user, "buckets": []string{bucket}, "members": []string{mainKey, f1Key, f2Key}, "copies": 2})

	// a slow upload through p-dead: admitted (writer record registered), body never completes
	u, err := e.clientOf(t, "p-dead").PresignedPutObject(ctx, bucket, "in-flight", 10*time.Minute)
	require.NoError(t, err)
	body, bodyW := io.Pipe()
	defer bodyW.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), body)
	require.NoError(t, err)
	req.ContentLength = 1 << 20
	go func() {
		res, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
		if err == nil {
			_ = res.Body.Close()
		}
	}()
	go func() { _, _ = bodyW.Write(make([]byte, 1024)) }()
	require.Eventually(t, func() bool {
		w := e.restJSON(t, http.MethodGet, "/tutti/protection-groups/pg2/writers", nil)
		m, _ := w["writers"].(map[string]any)
		for _, v := range m {
			if strings.HasPrefix(fmt.Sprint(v), "p-dead:") {
				return true
			}
		}
		return false
	}, 30*time.Second, 100*time.Millisecond, "slow write was not admitted")

	// the proxy "dies": it can no longer reach Redis (no release, no heartbeat, no status)
	e.redisFwd["p-dead"].setMode(modeBlackhole)
	e.restJSON(t, http.MethodPost, "/tutti/protection-groups/switchover", map[string]any{"id": "pg2", "target": f1Key})

	// while the lease is valid the group waits in draining
	time.Sleep(time.Second)
	g := e.group(t, "pg2")
	require.Equal(t, mainKey, g["primary"])

	start := time.Now()
	g = e.waitGroup(t, "pg2", 60*time.Second, func(g map[string]any) bool { return g["primary"] == f1Key }, "promotion stuck on a dead writer")
	t.Logf("promotion completed %s after the partition (lease 3s, status ttl 1.5s)", time.Since(start)+time.Second)
	w := e.restJSON(t, http.MethodGet, "/tutti/protection-groups/pg2/writers", nil)
	require.Empty(t, w["writers"])
	putEventually(t, e.clientOf(t, "p1"), bucket, "after-switch", "ok")
	require.True(t, existsDirect(t, f1Key, bucket, "after-switch"))
}
