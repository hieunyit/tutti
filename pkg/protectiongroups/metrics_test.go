package protectiongroups

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// gauge reads a metric from the default registry; ok is false when the series does not exist.
func metricValue(t *testing.T, name string, labels map[string]string) (float64, bool) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
	next:
		for _, m := range f.GetMetric() {
			for k, v := range labels {
				found := false
				for _, l := range m.GetLabel() {
					if l.GetName() == k && l.GetValue() == v {
						found = true
					}
				}
				if !found {
					continue next
				}
			}
			if m.Gauge != nil {
				return m.GetGauge().GetValue(), true
			}
			return m.GetCounter().GetValue(), true
		}
	}
	return 0, false
}

func requireMetric(t *testing.T, name string, labels map[string]string, want float64) {
	t.Helper()
	v, ok := metricValue(t, name, labels)
	require.True(t, ok, "series %s%v missing", name, labels)
	require.Equal(t, want, v, "%s%v", name, labels)
}

func TestProtectionGroupMetricsMove(t *testing.T) {
	svc, backend, _, now, reports := newService(t)
	ctx := t.Context()
	_, err := svc.Create(ctx, Group{ID: "mg", User: "tenant", Members: []string{"a", "b", "c"}, Copies: 2, Mode: "auto",
		After: time.Second, Electorate: []string{"p1", "p2", "p3"}, MaxSwitchesPerHour: 1}, false)
	require.NoError(t, err)
	grp := map[string]string{"group": "mg"}

	backend.mu.Lock()
	backend.lag["b"] = 7
	backend.mu.Unlock()
	reconcileN(t, svc, "mg", 1)
	requireMetric(t, "protection_group_copies_wanted", grp, 2)
	requireMetric(t, "protection_group_max_event_lag", grp, 7)
	requireMetric(t, "protection_group_copies_healthy", grp, 1)

	backend.mu.Lock()
	backend.lag["b"] = 0
	backend.mu.Unlock()
	g := reconcileN(t, svc, "mg", 3)
	require.Equal(t, "ready", g.State)
	requireMetric(t, "protection_group_max_event_lag", grp, 0)
	requireMetric(t, "protection_group_copies_healthy", grp, 3)
	requireMetric(t, "protection_group_frozen", grp, 0)

	// A crashed writer shows in active_writers and is counted when reaped.
	crashed := svc.Store.NewLease("crashed", false, time.Second)
	_, err = crashed.Admit(ctx, "tenant", "x", "a")
	require.NoError(t, err)
	reconcileN(t, svc, "mg", 1)
	requireMetric(t, "protection_group_active_writers", grp, 1)
	before, _ := metricValue(t, "protection_group_reaped_writers_total", nil)

	// Automatic failover: quorum says "a" is down. The global freeze holds the switch back first.
	down := time.Now().Add(-time.Minute)
	*reports = nil
	for _, p := range []string{"p1", "p2", "p3"} {
		*reports = append(*reports, proxystatus.Status{AppID: p, StartedAt: *now, UpdatedAt: *now, Storages: []proxystatus.StorageHealth{
			{Name: "a", Available: false, LastChange: down}, {Name: "b", Available: true}, {Name: "c", Available: true}}})
	}
	require.NoError(t, svc.Store.SetAutomaticFrozen(ctx, true))
	reconcileN(t, svc, "mg", 1)
	requireMetric(t, "protection_group_automatic_failover_frozen", nil, 1)
	requireMetric(t, "protection_group_switch_blocked_total", map[string]string{"group": "mg", "reason": "global_freeze"}, 1)
	g, _ = svc.Store.Get(ctx, "mg")
	require.Equal(t, "a", g.Primary)

	require.NoError(t, svc.Store.SetAutomaticFrozen(ctx, false))
	backend.mr.FastForward(2 * time.Second) // crashed lease expires
	g = reconcileN(t, svc, "mg", 3)
	require.Equal(t, "b", g.Primary)
	fz := 0.0
	if g.Frozen {
		fz = 1
	}
	requireMetric(t, "protection_group_frozen", grp, fz)
	requireMetric(t, "protection_group_primary_changes_total", map[string]string{"group": "mg", "reason": "automatic"}, 1)
	after, _ := metricValue(t, "protection_group_reaped_writers_total", nil)
	require.Equal(t, before+1, after)

	// Rate limit: one switch per hour is used up; a second outage of b is held back.
	for i := range *reports {
		(*reports)[i].Storages = []proxystatus.StorageHealth{{Name: "a", Available: true}, {Name: "b", Available: false, LastChange: down}, {Name: "c", Available: true}}
	}
	reconcileN(t, svc, "mg", 4)
	g, _ = svc.Store.Get(ctx, "mg")
	require.Equal(t, "b", g.Primary, "rate limit must hold the second automatic switch")
	v, _ := metricValue(t, "protection_group_switch_blocked_total", map[string]string{"group": "mg", "reason": "rate_limit"})
	require.Greater(t, v, 0.0)

	// Deleted groups disappear from the exposition.
	metrics.KeepProtectionGroups(nil)
	_, ok := metricValue(t, "protection_group_copies_wanted", grp)
	require.False(t, ok)
}

// A failing reconcile still exports the gauges (never left stale) and counts the error; deleted groups lose
// every series, including frozen series set only through SetProtectionFrozen.
func TestProtectionGroupMetricsOnReconcileError(t *testing.T) {
	svc, backend, _, _, _ := newService(t)
	ctx := t.Context()
	_, err := svc.Create(ctx, Group{ID: "me", User: "tenant", Members: []string{"a", "b"}, Copies: 2}, false)
	require.NoError(t, err)
	grp := map[string]string{"group": "me"}
	backend.mu.Lock()
	backend.lag["b"] = 5
	backend.mu.Unlock()
	svc.Reports = func(context.Context) ([]proxystatus.Status, error) { return nil, errors.New("status unavailable") }
	require.Error(t, svc.Reconcile(ctx, "me"))
	requireMetric(t, "protection_group_reconcile_errors_total", grp, 1)
	requireMetric(t, "protection_group_max_event_lag", grp, 5)
	requireMetric(t, "protection_group_copies_healthy", grp, 0)
	requireMetric(t, "protection_group_frozen", grp, 1)

	metrics.SetProtectionFrozen("frozen-only", true)
	metrics.KeepProtectionGroups(nil)
	for _, name := range []string{"protection_group_reconcile_errors_total", "protection_group_max_event_lag", "protection_group_frozen"} {
		_, ok := metricValue(t, name, grp)
		require.False(t, ok, name)
	}
	_, ok := metricValue(t, "protection_group_frozen", map[string]string{"group": "frozen-only"})
	require.False(t, ok)
}
