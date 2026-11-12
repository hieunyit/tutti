package migration

import (
	"bytes"
	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
	"github.com/hieunyit/tutti/test/app"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestSettingsSecurity_DisabledWebhookMustRejectRequestsBeforeAndAfterToggle(t *testing.T) {
	prior := systemsettings.Current()
	settings := prior
	settings.Webhook.Enabled = false
	systemsettings.ApplySettings(settings)
	t.Cleanup(func() { systemsettings.ApplySettings(prior) })
	e := app.SetupEmbedded(t, workerConf, proxyConf)
	client := &http.Client{Timeout: 3 * time.Second}
	call := func(method, path, body string) (int, string) {
		req, err := http.NewRequestWithContext(t.Context(), method, e.UrlHttpApi+path, bytes.NewBufferString(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	// The static config and process defaults have webhook disabled.
	require.False(t, workerConf.Api.Webhook.Enabled)
	code, b := call("POST", "/webhook/main/s3-notifications", `{"Records":[]}`)
	if code >= 200 && code < 300 {
		t.Errorf("disabled webhook accepted anonymous request before enable: HTTP %d %s", code, b)
	}
	code, b = call("PUT", "/tutti/system-settings", `{"webhook":{"enabled":true}}`)
	require.Equal(t, 200, code, b)
	require.Eventually(t, func() bool { return systemsettings.Current().Webhook.Enabled }, 3*time.Second, 20*time.Millisecond)
	code, b = call("POST", "/webhook/main/s3-notifications", `{"Records":[]}`)
	require.Equal(t, 200, code, b)
	code, b = call("PUT", "/tutti/system-settings", `{"webhook":{"enabled":false}}`)
	require.Equal(t, 200, code, b)
	require.Eventually(t, func() bool { return !systemsettings.Current().Webhook.Enabled }, 3*time.Second, 20*time.Millisecond)
	code, b = call("POST", "/webhook/main/s3-notifications", `{"Records":[]}`)
	if code >= 200 && code < 300 {
		t.Errorf("disabled webhook still accepted anonymous request after disable: HTTP %d %s", code, b)
	}
}

func TestSettingsSecurity_AnonymousInventoryLeaksWithManagementAuthEnabled(t *testing.T) {
	wc, err := worker.GetConfig()
	require.NoError(t, err)
	pc, err := proxy.GetConfig()
	require.NoError(t, err)
	hash, err := bcrypt.GenerateFromPassword([]byte("review-dummy-password"), bcrypt.MinCost)
	require.NoError(t, err)
	wc.Api.Auth = auth.Config{Enabled: true, SessionTTL: time.Hour, Users: []auth.User{{Username: "review-admin", PasswordHash: string(hash), Role: auth.RoleAdmin}}}
	e := app.SetupEmbedded(t, wc, pc)
	client := &http.Client{Timeout: 3 * time.Second}
	get := func(path string) (int, string) {
		req, err := http.NewRequestWithContext(t.Context(), "GET", e.UrlHttpApi+path, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	control, body := get("/storage")
	require.Equal(t, 401, control, "confirm management auth is active: %s", body)
	code, body := get("/tutti/storages")
	if code != 401 && code != 403 {
		t.Fatalf("anonymous inventory request bypassed enabled auth: HTTP %d %s", code, body)
	}
	for _, path := range []string{"/healthz", "/readyz"} {
		code, body = get(path)
		require.Equal(t, 200, code, "health endpoint %s must work without inventory access: %s", path, body)
	}
}
