package proxy

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hieunyit/tutti/pkg/policy"
)

// DefaultReadyMaxStaleness is how long the proxy stays ready on its last known snapshot while Redis is down.
const DefaultReadyMaxStaleness = 30 * time.Second

// DefaultReadHeaderTimeout bounds how long a client may take to send request headers when the config leaves it unset.
const DefaultReadHeaderTimeout = 10 * time.Second

// DefaultWriterLease is the default protection group writer lease TTL when unset.
const DefaultWriterLease = 30 * time.Second

// readiness answers GET /ready for the load balancer.
type readiness struct {
	mu           sync.RWMutex
	snapshot     func() policy.SnapshotStatus
	storages     func() int
	maxStaleness time.Duration
	draining     atomic.Bool
}

func (r *readiness) setMaxStaleness(d time.Duration) {
	if d <= 0 {
		d = DefaultReadyMaxStaleness
	}
	r.mu.Lock()
	r.maxStaleness = d
	r.mu.Unlock()
}

func (r *readiness) getMaxStaleness() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.maxStaleness <= 0 {
		return DefaultReadyMaxStaleness
	}
	return r.maxStaleness
}

type readyResponse struct {
	Ready   bool     `json:"ready"`
	Reasons []string `json:"reasons,omitempty"`
	// Mode is "ok" (Redis reachable), "read-only" (Redis unreachable, serving the last snapshot within
	// maxStaleness) or "" when not ready.
	Mode            string  `json:"mode,omitempty"`
	SnapshotLoaded  bool    `json:"snapshotLoaded"`
	SnapshotVersion string  `json:"snapshotVersion,omitempty"`
	SnapshotAgeSec  float64 `json:"snapshotAgeSeconds"`
	RedisReachable  bool    `json:"redisReachable"`
	Storages        int     `json:"storages"`
	Draining        bool    `json:"draining"`
	MaxStalenessSec float64 `json:"maxStalenessSeconds"`
}

func (r *readiness) check() readyResponse {
	st := r.snapshot()
	maxStaleness := r.getMaxStaleness()
	res := readyResponse{
		SnapshotLoaded:  st.Loaded,
		SnapshotVersion: st.Version,
		SnapshotAgeSec:  st.Age.Seconds(),
		RedisReachable:  st.RedisReachable,
		Storages:        r.storages(),
		Draining:        r.draining.Load(),
		MaxStalenessSec: maxStaleness.Seconds(),
	}
	if res.Draining {
		res.Reasons = append(res.Reasons, "draining: shutdown in progress")
	}
	if !st.Loaded {
		res.Reasons = append(res.Reasons, "config snapshot not loaded")
	}
	if res.Storages == 0 {
		res.Reasons = append(res.Reasons, "no storages known")
	}
	switch {
	case !st.Loaded:
	case st.RedisReachable:
		res.Mode = "ok"
	case st.Age <= maxStaleness:
		res.Mode = "read-only"
	default:
		res.Reasons = append(res.Reasons, "redis unreachable and snapshot older than maxStaleness")
	}
	res.Ready = len(res.Reasons) == 0
	if !res.Ready {
		res.Mode = ""
	}
	return res
}

// wrap serves GET/HEAD /ready before the S3 proxy chain: no auth, no traffic statistics, no logging.
// "/ready" is a reserved path. Requests that carry an Authorization header, a query string or presigned
// parameters are never answered here, so a bucket that is literally named "ready" stays reachable for
// authenticated clients.
func (r *readiness) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ready" && (req.Method == http.MethodGet || req.Method == http.MethodHead) &&
			req.URL.RawQuery == "" && req.Header.Get("Authorization") == "" {
			res := r.check()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			if res.Ready {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			if req.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(res)
			}
			return
		}
		next.ServeHTTP(w, req)
	})
}
