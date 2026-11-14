package worker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/rs/zerolog"
)

// DebugConfig holds diagnostics that are OFF by default.
type DebugConfig struct {
	// PprofAddr is the listen address of the net/http/pprof server, e.g. "127.0.0.1:6060". Empty = disabled.
	// The profiling endpoints are unauthenticated: bind them to loopback and reach them with
	// `docker compose exec` or `kubectl port-forward`, never expose the port.
	PprofAddr string `yaml:"pprofAddr"`
}

// newPprofMux serves only the pprof handlers (a private mux, not http.DefaultServeMux).
func newPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

// startPprof starts the pprof server when addr is not empty. It returns the bound address ("" when disabled).
// The server stops when ctx is cancelled.
func startPprof(ctx context.Context, addr string, logger zerolog.Logger) (string, error) {
	if addr == "" {
		return "", nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	srv := &http.Server{
		Handler:           newPprofMux(),
		ReadHeaderTimeout: 10 * time.Second,
		// no WriteTimeout: a CPU profile or trace is streamed for the requested seconds
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("pprof server stopped")
		}
	}()
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
	logger.Warn().Str("addr", ln.Addr().String()).Msg("pprof server enabled (debug.pprofAddr): unauthenticated, keep it on loopback")
	return ln.Addr().String(), nil
}
