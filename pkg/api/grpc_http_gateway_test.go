package api_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/api"
)

func TestConcurrencyLimiter_LimitsInFlightRequests(t *testing.T) {
	const limit = 3
	started := make(chan struct{}, limit)
	unblock := make(chan struct{})

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-unblock
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	limited := api.ConcurrencyLimiter(baseHandler, limit)

	var wg sync.WaitGroup
	// Launch limit requests concurrently
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			limited.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
		}()
	}

	// Wait until all `limit` requests have acquired the semaphore
	for i := 0; i < limit; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent requests to start")
		}
	}

	// Send an additional request when all slots are taken; must be rejected with 503
	overflowRec := httptest.NewRecorder()
	overflowReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	limited.ServeHTTP(overflowRec, overflowReq)

	require.Equal(t, http.StatusServiceUnavailable, overflowRec.Code)
	require.Equal(t, "1", overflowRec.Header().Get("Retry-After"))

	// Unblock ongoing requests
	close(unblock)
	wg.Wait()

	// After ongoing requests finish, a new request should succeed
	unblockAfter := make(chan struct{})
	close(unblockAfter)
	postRec := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	limited.ServeHTTP(postRec, postReq)
	require.Equal(t, http.StatusOK, postRec.Code)
}
