package resilience

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/rs/xid"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/util"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/test/app"
)

func getFreePort(t testing.TB) (int, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port, fmt.Sprintf("127.0.0.1:%d", port)
}

func startSecondProxy(t *testing.T, redisAddr, group string) string {
	t.Helper()
	pStorages := map[string]s3.Storage{}
	for name, st := range storages {
		cp := st
		cp.HealthCheckInterval = 500 * time.Millisecond
		cp.HttpTimeout = 5 * time.Second
		pStorages[name] = cp
	}
	pc, err := app.DeepCopyStruct(proxyConf)
	require.NoError(t, err)
	pc.Storage = app.ProxyS3Config(mainKey, pStorages)
	pc.Auth.UseStorage = mainKey
	pc.Health.FailureThreshold = 2
	pc.Health.OpenDuration = 3 * time.Second
	pc.Replication.EventRetryBudget = 300 * time.Millisecond
	pc.Status.Interval = 200 * time.Millisecond
	pc.Group = group
	pc.Redis.Address = redisAddr
	port, addr := getFreePort(t)
	pc.Port = port
	pc.Address = addr
	require.NoError(t, pc.Validate())

	ctx := t.Context()
	done := make(chan error, 1)
	go func() {
		done <- proxy.Start(ctx, dom.AppInfo{
			Version: "test",
			App:     "proxy",
			AppID:   "proxy-" + group + "-" + xid.New().String(),
		}, pc)
	}()
	t.Cleanup(func() {
		if err := <-done; util.IsServerError(err) {
			t.Errorf("second proxy (%s) exited with error: %v", group, err)
		}
	})
	return addr
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

func findProxyByGroup(proxies []any, group string) map[string]any {
	for _, raw := range proxies {
		m, ok := raw.(map[string]any)
		if ok && m["group"] == group {
			return m
		}
	}
	return nil
}

func TestProxyFleetConfig(t *testing.T) {
	r := newFailingRedis(t)
	e := startEnv(t, envOpts{
		redis: r,
		proxyHook: func(c *proxy.Config) {
			c.Group = "group-a"
			c.Status.Interval = 200 * time.Millisecond
		},
	})
	_ = startSecondProxy(t, r.Addr(), "group-b")

	// 1. Both proxies register their respective group and initial appliedConfigVersion == 0.
	initFC := e.restJSON(t, http.MethodGet, "/tutti/proxy-config", nil)
	require.EqualValues(t, 0, toInt64(initFC["version"]))

	require.Eventually(t, func() bool {
		res := e.restJSON(t, http.MethodGet, "/tutti/proxies", nil)
		list, _ := res["proxies"].([]any)
		pa := findProxyByGroup(list, "group-a")
		pb := findProxyByGroup(list, "group-b")
		return pa != nil && pb != nil &&
			toInt64(pa["appliedConfigVersion"]) == 0 &&
			toInt64(pb["appliedConfigVersion"]) == 0
	}, e.WaitShort, e.RetryShort, "both group-a and group-b proxies should report initial v0")

	// 2. Set cluster-wide defaults (v1): sync replication enabled (writeQuorum=2), requireSignedContentType=true, health.failureThreshold=5.
	savedV1 := e.restJSON(t, http.MethodPut, "/tutti/proxy-config", map[string]any{
		"defaults": map[string]any{
			"replication": map[string]any{
				"onEventError": "log",
				"sync": map[string]any{
					"enabled":     true,
					"writeQuorum": 2,
					"timeout":     "10s",
				},
			},
			"health": map[string]any{
				"failureThreshold": 5,
			},
			"requireSignedContentType": true,
		},
	})
	require.EqualValues(t, 1, toInt64(savedV1["version"]))

	require.Eventually(t, func() bool {
		res := e.restJSON(t, http.MethodGet, "/tutti/proxies", nil)
		if toInt64(res["configVersion"]) != 1 {
			return false
		}
		list, _ := res["proxies"].([]any)
		pa := findProxyByGroup(list, "group-a")
		pb := findProxyByGroup(list, "group-b")
		if pa == nil || pb == nil {
			return false
		}
		if toInt64(pa["appliedConfigVersion"]) != 1 || toInt64(pb["appliedConfigVersion"]) != 1 {
			return false
		}
		for _, p := range []map[string]any{pa, pb} {
			settings, _ := p["settings"].(map[string]any)
			repl, _ := settings["replication"].(map[string]any)
			syncCfg, _ := repl["sync"].(map[string]any)
			healthCfg, _ := settings["health"].(map[string]any)
			if repl["onEventError"] != "log" ||
				syncCfg["enabled"] != true ||
				toInt64(syncCfg["writeQuorum"]) != 2 ||
				toInt64(healthCfg["failureThreshold"]) != 5 ||
				settings["requireSignedContentType"] != true ||
				p["pendingRestart"] == true {
				return false
			}
		}
		return true
	}, e.WaitShort, e.RetryShort, "both proxies should hot-reload v1 defaults without pendingRestart")

	// Verify live synchronous replication on group-a (which started with sync.enabled=false!).
	bkt := bucketName("fleet-sync")
	e.replicateBucket(t, bkt, f1Key)
	code, hdr, body := e.presignedPut(t, bkt, "obj-v1", "hello-fleet-sync")
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "2/2", hdr.Get("X-Tutti-Replicas"), "sync replication enabled via fleet config should take effect live")

	// 3. Set per-group override for group-b (v2): disable sync, enable failover, set requireSignedContentType=false.
	savedV2 := e.restJSON(t, http.MethodPut, "/tutti/proxy-config", map[string]any{
		"defaults": map[string]any{
			"replication": map[string]any{
				"onEventError": "log",
				"sync": map[string]any{
					"enabled":     true,
					"writeQuorum": 2,
					"timeout":     "10s",
				},
			},
			"health": map[string]any{
				"failureThreshold": 5,
			},
			"requireSignedContentType": true,
		},
		"groups": map[string]any{
			"group-b": map[string]any{
				"replication": map[string]any{
					"sync": map[string]any{
						"enabled": false,
					},
				},
				"failover": map[string]any{
					"enabled": true,
					"after":   "15s",
				},
				"requireSignedContentType": false,
			},
		},
	})
	require.EqualValues(t, 2, toInt64(savedV2["version"]))

	require.Eventually(t, func() bool {
		res := e.restJSON(t, http.MethodGet, "/tutti/proxies", nil)
		if toInt64(res["configVersion"]) != 2 {
			return false
		}
		list, _ := res["proxies"].([]any)
		pa := findProxyByGroup(list, "group-a")
		pb := findProxyByGroup(list, "group-b")
		if pa == nil || pb == nil {
			return false
		}
		if toInt64(pa["appliedConfigVersion"]) != 2 || toInt64(pb["appliedConfigVersion"]) != 2 {
			return false
		}
		sa, _ := pa["settings"].(map[string]any)
		sb, _ := pb["settings"].(map[string]any)
		syncA, _ := sa["replication"].(map[string]any)["sync"].(map[string]any)
		syncB, _ := sb["replication"].(map[string]any)["sync"].(map[string]any)
		foB, _ := sb["failover"].(map[string]any)
		return syncA["enabled"] == true &&
			sa["requireSignedContentType"] == true &&
			syncB["enabled"] == false &&
			foB["enabled"] == true &&
			foB["after"] == "15s" &&
			sb["requireSignedContentType"] == false
	}, e.WaitShort, e.RetryShort, "group-b should apply its group override while group-a keeps cluster defaults")

	// 4. Set a non-hot-reloadable setting (readHeaderTimeout) on group-b (v3) and verify pendingRestart + pendingRestartReasons.
	savedV3 := e.restJSON(t, http.MethodPut, "/tutti/proxy-config", map[string]any{
		"defaults": map[string]any{
			"requireSignedContentType": true,
		},
		"groups": map[string]any{
			"group-b": map[string]any{
				"readHeaderTimeout": "25s",
			},
		},
	})
	require.EqualValues(t, 3, toInt64(savedV3["version"]))

	require.Eventually(t, func() bool {
		res := e.restJSON(t, http.MethodGet, "/tutti/proxies", nil)
		list, _ := res["proxies"].([]any)
		pa := findProxyByGroup(list, "group-a")
		pb := findProxyByGroup(list, "group-b")
		if pa == nil || pb == nil {
			return false
		}
		if toInt64(pa["appliedConfigVersion"]) != 3 || toInt64(pb["appliedConfigVersion"]) != 3 {
			return false
		}
		reasonsRaw, _ := pb["pendingRestartReasons"].([]any)
		return pa["pendingRestart"] != true &&
			pb["pendingRestart"] == true &&
			len(reasonsRaw) == 1 &&
			reasonsRaw[0] == "readHeaderTimeout"
	}, e.WaitShort, e.RetryShort, "group-b should report pendingRestart=true with reason readHeaderTimeout")
}
