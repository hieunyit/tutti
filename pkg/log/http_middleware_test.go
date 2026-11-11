package log

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
)

func TestHTTPMiddleware_RedactsPresignedQuery(t *testing.T) {
	origLevel := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(origLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	var buf bytes.Buffer
	cfg := &Config{Level: "debug", Json: true}
	middleware := HttpMiddleware(cfg, "testapp", "id1", xctx.Api)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := zerolog.Ctx(r.Context()).Output(&buf)
		l.Debug().Msg("handling request")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/bucket/object?X-Amz-Signature=secret123&X-Amz-Credential=key", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logged := buf.String()
	require.Contains(t, logged, "/bucket/object")
	require.NotContains(t, logged, "secret123")
	require.NotContains(t, logged, "X-Amz-Signature")
	require.NotContains(t, logged, "http_query")
}
