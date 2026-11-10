package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// docker-compose derives the worker DSN from POSTGRES_USER/PASSWORD/DB with this override.
func TestControlPlaneDSNEnvOverride(t *testing.T) {
	f, err := os.Open("../../docker-compose/worker-conf.yaml")
	require.NoError(t, err)
	defer f.Close()
	m, err := readYAMLMap(f)
	require.NoError(t, err)
	t.Setenv("CFG_CONTROLPLANE_POSTGRES_DSN", "postgres://u:p@postgres:5432/d?sslmode=disable")
	require.NoError(t, applyEnvOverrides(m, "CFG"))
	cp := m["controlPlane"].(map[string]any)["postgres"].(map[string]any)
	require.Equal(t, "postgres://u:p@postgres:5432/d?sslmode=disable", cp["dsn"])
}
