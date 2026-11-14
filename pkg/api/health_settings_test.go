package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/stretchr/testify/require"
)

func TestHealthProbes_DoNotExposeInventoryOrDependOnLogin(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	healthy := true
	checks := 0
	h := healthHandler(ctx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }), func(context.Context) error {
		checks++
		if !healthy {
			return errors.New("private backend failure")
		}
		return nil
	})
	call := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		return w
	}
	require.Equal(t, 200, call("GET", "/healthz").Code)
	require.Zero(t, checks)
	require.Equal(t, 200, call("GET", "/readyz").Code)
	healthy = false
	failed := call("GET", "/readyz")
	require.Equal(t, 503, failed.Code)
	require.Empty(t, failed.Body.String())
	require.Equal(t, 200, call("GET", "/healthz").Code, "dependency outage must not cause liveness restart loops")
	require.Equal(t, 401, call("GET", "/tutti/storages").Code)
	require.Equal(t, 405, call("POST", "/healthz").Code)
	cancel()
	require.Equal(t, 503, call("GET", "/readyz").Code)
}
func TestPolicyWebhook_UsesEffectiveEnabledAndBaseURL(t *testing.T) {
	conf := WebhookConfig{Enabled: false, BaseURL: "http://old.test"}
	h := &policyHandlers{webhookConf: &WebhookConfig{Enabled: true, BaseURL: "http://static.test"}}
	WithPolicyWebhook(func() WebhookConfig { return conf })(h)
	_, err := h.webhookURLForStorage("source", dom.S3)
	require.Error(t, err)
	conf = WebhookConfig{Enabled: true, BaseURL: "http://new.test"}
	url, err := h.webhookURLForStorage("source", dom.S3)
	require.NoError(t, err)
	require.Equal(t, "http://new.test/webhook/source/s3-notifications", url)
	conf.BaseURL = ""
	_, err = h.webhookURLForStorage("source", dom.S3)
	require.Error(t, err, "clearing dynamic URL must not retain static URL")
	conf.Enabled = false
	require.Error(t, h.validateWebhookEventSource(pb.EventSource_EVENT_SOURCE_WEBHOOK))
}
func TestDisabledWebhook_RejectsBothProtocolsBeforeProcessing(t *testing.T) {
	prior := systemsettings.Current()
	settings := prior
	settings.Webhook.Enabled = false
	systemsettings.ApplySettings(settings)
	t.Cleanup(func() { systemsettings.ApplySettings(prior) })
	h := WebhookHandlers(nil, nil, nil, nil)
	_, err := h.S3Notifications(t.Context(), &pb.S3NotificationRequest{})
	require.Error(t, err)
	_, err = h.SwiftEvents(t.Context(), &pb.SwiftEventsRequest{})
	require.Error(t, err)
}
