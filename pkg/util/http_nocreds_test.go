package util

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

type namedNoCreds struct{}

func (namedNoCreds) Error() string       { return "no creds" }
func (namedNoCreds) Unwrap() error       { return dom.ErrNoCredentials }
func (namedNoCreds) StorageName() string { return "st-x" }

func noCredsCount(t *testing.T, storage string) float64 {
	fams, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, f := range fams {
		if f.GetName() != "proxy_no_credentials_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "storage" && l.GetValue() == storage {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func TestNoCredentialsMetric(t *testing.T) {
	before := noCredsCount(t, "st-x")
	rec := httptest.NewRecorder()
	WriteError(context.Background(), rec, fmt.Errorf("wrap: %w", namedNoCreds{}))
	require.Equal(t, 503, rec.Code)
	require.Equal(t, before+1, noCredsCount(t, "st-x"))
}

func TestRateLimitExceededResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(context.Background(), rec, &dom.ErrRateLimitExceeded{RetryIn: 3 * 1000000000}) // 3s
	require.Equal(t, 503, rec.Code)
	require.Equal(t, "3", rec.Header().Get("Retry-After"))
	require.Contains(t, rec.Body.String(), "<Code>SlowDown</Code>")
}
