package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestHandlerDoesNotServePprof(t *testing.T) {
	h := Handler(context.Background(), dom.AppInfo{})
	for path, want := range map[string]int{"/metrics": http.StatusOK, "/health": http.StatusOK, "/debug/pprof/": http.StatusNotFound, "/debug/pprof/heap": http.StatusNotFound, "/debug/pprof/profile": http.StatusNotFound} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, want, rec.Code, path)
	}
}
