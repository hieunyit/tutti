package proxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/proxystatus"
)

func fleetStartup() proxystatus.Settings {
	return proxystatus.Settings{
		Replication: proxystatus.ReplicationSettings{
			OnEventError: "fail", EventRetryBudget: time.Second,
			Sync: proxystatus.SyncSettings{Timeout: time.Second, OnQuorumFailure: "fail"},
		},
		Health:            proxystatus.HealthSettings{FailureThreshold: 3, OpenDuration: 30 * time.Second},
		Failover:          proxystatus.FailoverSettings{CheckInterval: time.Second, After: time.Minute},
		ReadyMaxStaleness: 30 * time.Second,
	}
}

func TestResolveFleet(t *testing.T) {
	yes, no := true, false
	zeroTimeout := time.Duration(0)
	quorum := 5
	bad := "bogus"

	// valid override applies
	fc := &proxystatus.FleetConfig{Version: 2, Defaults: proxystatus.SettingsOverride{
		Replication: &proxystatus.ReplicationOverride{Sync: &proxystatus.SyncOverride{Enabled: &yes}},
	}}
	eff, _, _, err := resolveFleet(fleetStartup(), "default", fc, 3, true)
	require.NoError(t, err)
	require.True(t, eff.Replication.Sync.Enabled)

	// sync is not reported effective without a Syncer
	eff, _, _, err = resolveFleet(fleetStartup(), "default", fc, 3, false)
	require.NoError(t, err)
	require.False(t, eff.Replication.Sync.Enabled)

	cases := map[string]proxystatus.SettingsOverride{
		"sync enabled needs timeout": {Replication: &proxystatus.ReplicationOverride{Sync: &proxystatus.SyncOverride{Enabled: &yes, Timeout: &zeroTimeout}}},
		"quorum above storages":      {Replication: &proxystatus.ReplicationOverride{Sync: &proxystatus.SyncOverride{WriteQuorum: &quorum}}},
		"bad onEventError":           {Replication: &proxystatus.ReplicationOverride{OnEventError: &bad}},
		"tiny readiness":             {ReadyMaxStaleness: durPtr(time.Millisecond)},
		"failover needs after":       {Failover: &proxystatus.FailoverOverride{Enabled: &yes, After: durPtr(0)}},
	}
	for name, ov := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := resolveFleet(fleetStartup(), "default", &proxystatus.FleetConfig{Defaults: ov}, 3, true)
			require.Error(t, err)
		})
	}
	// a bad override of another group invalidates the document
	_, _, _, err = resolveFleet(fleetStartup(), "default", &proxystatus.FleetConfig{Groups: map[string]proxystatus.SettingsOverride{
		"other": {RequireSignedContentType: &no, ReadyMaxStaleness: durPtr(time.Millisecond)},
	}}, 3, true)
	require.Error(t, err)
}

func durPtr(d time.Duration) *time.Duration { return &d }

func TestFleetConfigStateKeepsLastGoodAndReportsError(t *testing.T) {
	st := &fleetConfigState{settings: fleetStartup(), version: 1}
	require.True(t, st.setError("boom"))
	require.False(t, st.setError("boom"))
	_, ver, _, _ := st.get()
	require.EqualValues(t, 1, ver)
	require.Equal(t, "boom", st.getError())
	require.True(t, st.setError(""))
}
