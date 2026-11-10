package proxystatus

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func ptr[T any](v T) *T { return &v }

func TestResolve_LayeredSettingsAndPendingRestart(t *testing.T) {
	startup := Settings{
		Replication: ReplicationSettings{
			OnEventError:     "log",
			EventRetryBudget: 100 * time.Millisecond,
			Sync: SyncSettings{
				Enabled:         false,
				WriteQuorum:     1,
				Timeout:         5 * time.Second,
				OnQuorumFailure: "fail",
			},
		},
		Health: HealthSettings{
			FailureThreshold: 3,
			OpenDuration:     10 * time.Second,
		},
		Failover: FailoverSettings{
			Enabled:       false,
			CheckInterval: 5 * time.Second,
			After:         30 * time.Second,
			MaxEventLag:   100,
		},
		RequireSignedContentType: false,
		ReadHeaderTimeout:        15 * time.Second,
		WriterLease:              10 * time.Second,
		DrainDelay:               3 * time.Second,
		ReadyMaxStaleness:        30 * time.Second,
	}

	// nil FleetConfig returns startup unchanged and no pendingRestart.
	eff, pending, reasons := Resolve(startup, "", nil)
	require.Equal(t, startup, eff)
	require.False(t, pending)
	require.Empty(t, reasons)

	fc := &FleetConfig{
		Version: 2,
		Defaults: SettingsOverride{
			Replication: &ReplicationOverride{
				OnEventError: ptr("fail"),
				Sync: &SyncOverride{
					Enabled:     ptr(true),
					WriteQuorum: ptr(2),
				},
			},
			Health: &HealthOverride{
				FailureThreshold: ptr(5),
			},
			RequireSignedContentType: ptr(true),
			DrainDelay:               ptr(5 * time.Second),
		},
		Groups: map[string]SettingsOverride{
			"canary": {
				Replication: &ReplicationOverride{
					Sync: &SyncOverride{
						WriteQuorum:     ptr(3),
						Timeout:         ptr(8 * time.Second),
						OnQuorumFailure: ptr("accept"),
					},
				},
				Failover: &FailoverOverride{
					Enabled: ptr(true),
					After:   ptr(15 * time.Second),
				},
				ReadHeaderTimeout: ptr(25 * time.Second),
				WriterLease:       ptr(20 * time.Second),
			},
		},
	}

	// Default group gets startup + defaults (all hot-reloadable).
	effDefault, pendingDefault, reasonsDefault := Resolve(startup, "default", fc)
	require.False(t, pendingDefault)
	require.Empty(t, reasonsDefault)
	require.Equal(t, "fail", effDefault.Replication.OnEventError)
	require.True(t, effDefault.Replication.Sync.Enabled)
	require.Equal(t, 2, effDefault.Replication.Sync.WriteQuorum)
	require.Equal(t, 5*time.Second, effDefault.Replication.Sync.Timeout)
	require.Equal(t, 5, effDefault.Health.FailureThreshold)
	require.True(t, effDefault.RequireSignedContentType)
	require.Equal(t, 5*time.Second, effDefault.DrainDelay)

	// Canary group gets startup + defaults + canary overrides, plus pendingRestart for readHeaderTimeout & writerLease.
	effCanary, pendingCanary, reasonsCanary := Resolve(startup, "canary", fc)
	require.True(t, pendingCanary)
	require.Equal(t, []string{"readHeaderTimeout", "protectionGroups.writerLease"}, reasonsCanary)
	require.Equal(t, "fail", effCanary.Replication.OnEventError)
	require.True(t, effCanary.Replication.Sync.Enabled)
	require.Equal(t, 3, effCanary.Replication.Sync.WriteQuorum)
	require.Equal(t, 8*time.Second, effCanary.Replication.Sync.Timeout)
	require.Equal(t, "accept", effCanary.Replication.Sync.OnQuorumFailure)
	require.True(t, effCanary.Failover.Enabled)
	require.Equal(t, 15*time.Second, effCanary.Failover.After)
	require.Equal(t, 25*time.Second, effCanary.ReadHeaderTimeout)
	require.Equal(t, 20*time.Second, effCanary.WriterLease)
}

func TestFleetConfig_Validate(t *testing.T) {
	valid := FleetConfig{
		Defaults: SettingsOverride{
			Replication: &ReplicationOverride{
				OnEventError:     ptr("log"),
				EventRetryBudget: ptr(50 * time.Millisecond),
				Sync: &SyncOverride{
					Enabled:         ptr(true),
					WriteQuorum:     ptr(2),
					Timeout:         ptr(5 * time.Second),
					OnQuorumFailure: ptr("fail"),
				},
			},
			Health: &HealthOverride{
				FailureThreshold: ptr(3),
				OpenDuration:     ptr(10 * time.Second),
			},
			Failover: &FailoverOverride{
				Enabled:       ptr(true),
				CheckInterval: ptr(2 * time.Second),
				After:         ptr(10 * time.Second),
				MaxEventLag:   ptr(0),
			},
			ReadHeaderTimeout: ptr(15 * time.Second),
			WriterLease:       ptr(10 * time.Second),
			DrainDelay:        ptr(time.Duration(0)),
			ReadyMaxStaleness: ptr(10 * time.Second),
		},
		Groups: map[string]SettingsOverride{
			"eu-west_1.a": {},
		},
	}
	require.NoError(t, valid.Validate())

	cases := []struct {
		name string
		cfg  FleetConfig
	}{
		{
			name: "invalid onEventError",
			cfg:  FleetConfig{Defaults: SettingsOverride{Replication: &ReplicationOverride{OnEventError: ptr("panic")}}},
		},
		{
			name: "negative eventRetryBudget",
			cfg:  FleetConfig{Defaults: SettingsOverride{Replication: &ReplicationOverride{EventRetryBudget: ptr(-time.Second)}}},
		},
		{
			name: "negative writeQuorum",
			cfg:  FleetConfig{Defaults: SettingsOverride{Replication: &ReplicationOverride{Sync: &SyncOverride{WriteQuorum: ptr(-1)}}}},
		},
		{
			name: "zero sync timeout",
			cfg:  FleetConfig{Defaults: SettingsOverride{Replication: &ReplicationOverride{Sync: &SyncOverride{Timeout: ptr(time.Duration(0))}}}},
		},
		{
			name: "invalid onQuorumFailure",
			cfg:  FleetConfig{Defaults: SettingsOverride{Replication: &ReplicationOverride{Sync: &SyncOverride{OnQuorumFailure: ptr("ignore")}}}},
		},
		{
			name: "zero failureThreshold",
			cfg:  FleetConfig{Defaults: SettingsOverride{Health: &HealthOverride{FailureThreshold: ptr(0)}}},
		},
		{
			name: "zero openDuration",
			cfg:  FleetConfig{Defaults: SettingsOverride{Health: &HealthOverride{OpenDuration: ptr(time.Duration(0))}}},
		},
		{
			name: "zero failover checkInterval",
			cfg:  FleetConfig{Defaults: SettingsOverride{Failover: &FailoverOverride{CheckInterval: ptr(time.Duration(0))}}},
		},
		{
			name: "negative maxEventLag",
			cfg:  FleetConfig{Defaults: SettingsOverride{Failover: &FailoverOverride{MaxEventLag: ptr(-1)}}},
		},
		{
			name: "invalid group name",
			cfg:  FleetConfig{Groups: map[string]SettingsOverride{"-bad": {}}},
		},
		{
			name: "zero readHeaderTimeout",
			cfg:  FleetConfig{Defaults: SettingsOverride{ReadHeaderTimeout: ptr(time.Duration(0))}},
		},
		{
			name: "negative drainDelay",
			cfg:  FleetConfig{Defaults: SettingsOverride{DrainDelay: ptr(-time.Second)}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			require.ErrorIs(t, err, dom.ErrInvalidArg)
		})
	}
}

func TestFleetConfigStore_GetAndSet(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewFleetConfigStore(c)
	ctx := context.Background()

	// Initial Get returns version 0 and empty groups map.
	initial, err := store.Get(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 0, initial.Version)
	require.Empty(t, initial.Groups)

	// Subscribe to tutti:cfg:changed to verify notifications.
	sub := c.Subscribe(ctx, ConfigChangedChannel)
	defer sub.Close()
	_, err = sub.Receive(ctx)
	require.NoError(t, err)

	// First Set creates version 1.
	saved1, err := store.Set(ctx, FleetConfig{
		Defaults: SettingsOverride{
			RequireSignedContentType: ptr(true),
		},
		Groups: map[string]SettingsOverride{
			"group-a": {
				Health: &HealthOverride{FailureThreshold: ptr(7)},
			},
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, saved1.Version)
	require.False(t, saved1.UpdatedAt.IsZero())
	require.NotNil(t, saved1.Defaults.RequireSignedContentType)
	require.True(t, *saved1.Defaults.RequireSignedContentType)

	// Verify pub/sub message and tutti:cfg:version increment.
	msg, err := sub.ReceiveMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, "1", msg.Payload)

	verStr, err := c.Get(ctx, ConfigVersionKey).Result()
	require.NoError(t, err)
	require.Equal(t, "1", verStr)

	// Second Set increments to version 2.
	saved2, err := store.Set(ctx, FleetConfig{
		Defaults: SettingsOverride{
			RequireSignedContentType: ptr(false),
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, saved2.Version)

	got, err := store.Get(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, got.Version)
	require.NotNil(t, got.Defaults.RequireSignedContentType)
	require.False(t, *got.Defaults.RequireSignedContentType)
}
