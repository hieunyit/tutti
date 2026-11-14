package worker

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestStartPprofDisabledByDefault(t *testing.T) {
	addr, err := startPprof(context.Background(), "", zerolog.Nop())
	require.NoError(t, err)
	require.Empty(t, addr)
	var c Config
	require.Empty(t, c.Debug.PprofAddr)
}

func TestStartPprofServes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, err := startPprof(ctx, "127.0.0.1:0", zerolog.Nop())
	require.NoError(t, err)
	resp, err := http.Get("http://" + addr + "/debug/pprof/heap?debug=1")
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	require.Contains(t, string(b), "heap profile")
	cancel()
	require.Eventually(t, func() bool {
		_, err := http.Get("http://" + addr + "/debug/pprof/")
		return err != nil
	}, 3*time.Second, 50*time.Millisecond)
}
