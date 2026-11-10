package util

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/trafficstats"
)

func TestWriteErrorNoCredentialsIs503AndCounted(t *testing.T) {
	before := trafficstats.Default.Snapshot().NoCredentials
	w := httptest.NewRecorder()
	WriteError(t.Context(), w, fmt.Errorf("route: %w", &objstore.NoCredentialsError{Storage: "archive", User: "alice"}))
	require.Equal(t, 503, w.Code)
	require.Contains(t, w.Body.String(), "<Code>ServiceUnavailable</Code>")
	require.Contains(t, w.Body.String(), "storage archive has no credentials for user alice")
	require.Equal(t, before+1, trafficstats.Default.Snapshot().NoCredentials)
}
