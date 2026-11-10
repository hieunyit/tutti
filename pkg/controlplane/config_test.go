package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestConfigDefaultsAndValidation(t *testing.T) {
	c := Config{}.WithDefaults()
	require.Equal(t, BackendRedis, c.Backend)
	require.NoError(t, c.Validate(), "redis backend needs nothing")
	require.EqualValues(t, 10, c.Postgres.MaxConns)
	require.Equal(t, 500*time.Millisecond, c.Outbox.PollInterval)

	pgc := Config{Backend: BackendPostgres}.WithDefaults()
	require.ErrorIs(t, pgc.Validate(), dom.ErrInvalidArg)
	pgc.Postgres.DSN = "postgres://u:p@h/db"
	require.NoError(t, pgc.Validate())
	pgc.Postgres.MinConns = 50
	require.ErrorIs(t, pgc.Validate(), dom.ErrInvalidArg)
	require.ErrorIs(t, Config{Backend: "mysql"}.Validate(), dom.ErrInvalidArg)
}

func TestConfigYAML(t *testing.T) {
	var wrapper struct {
		ControlPlane Config `yaml:"controlPlane"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(`
controlPlane:
  backend: postgres
  readOnly: true
  postgres:
    dsn: postgres://tutti@db:5432/tutti?sslmode=verify-full
    maxConns: 20
    statementTimeout: 5s
  outbox:
    pollInterval: 250ms
`), &wrapper))
	c := wrapper.ControlPlane.WithDefaults()
	require.Equal(t, BackendPostgres, c.Backend)
	require.True(t, c.ReadOnly)
	require.EqualValues(t, 20, c.Postgres.MaxConns)
	require.Equal(t, 5*time.Second, c.Postgres.StatementTimeout)
	require.Equal(t, 250*time.Millisecond, c.Outbox.PollInterval)
	require.NoError(t, c.Validate())
}

func TestActorContext(t *testing.T) {
	require.Equal(t, "system", ActorFrom(context.Background()).Name)
	ctx := WithActor(context.Background(), Actor{Name: "alice", RequestID: "r1"})
	require.Equal(t, Actor{Name: "alice", RequestID: "r1"}, ActorFrom(ctx))
	require.False(t, SkipOutbox(ctx))
	require.True(t, SkipOutbox(WithSkipOutbox(ctx)))
	require.True(t, errors.Is(ErrVersionConflict, dom.ErrFailedPrecondition))
	require.True(t, IsConflict(ErrVersionConflict))
}

func TestReplicationIDRoundTrip(t *testing.T) {
	for _, id := range []ReplicationID{
		{User: "u", FromStorage: "a", ToStorage: "b"},
		{User: "u", FromStorage: "a", ToStorage: "b", FromBucket: "x", ToBucket: "y"},
	} {
		got, ok := ParseReplicationID(id.String())
		require.True(t, ok)
		require.Equal(t, id, got)
	}
	_, ok := ParseReplicationID("a:b")
	require.False(t, ok)
	require.Equal(t, "bucket:u:b", FailoverEventKey("u", "b"))
}
