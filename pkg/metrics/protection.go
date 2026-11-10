package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	pgCopiesHealthy = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "protection_group_copies_healthy",
		Help: "Members of the protection group that are fully current (available, initialized, zero event lag, verified)."}, []string{"group"})
	pgCopiesWanted = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "protection_group_copies_wanted",
		Help: "Number of copies the protection group is configured to keep."}, []string{"group"})
	pgMaxEventLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "protection_group_max_event_lag",
		Help: "Largest number of unprocessed replication events among the secondary members of the protection group."}, []string{"group"})
	pgFrozen = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "protection_group_frozen",
		Help: "1 while writes to the protection group are frozen (promotion, initial drain or audit)."}, []string{"group"})
	pgActiveWriters = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "protection_group_active_writers",
		Help: "Admitted proxy writers of the protection group that have not been released."}, []string{"group"})
	pgPrimaryChanges = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "protection_group_primary_changes_total",
		Help: "Primary changes of a protection group by reason (automatic, manual)."}, []string{"group", "reason"})
	pgReconcileErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "protection_group_reconcile_errors_total",
		Help: "Failed reconcile cycles of a protection group. While it grows, the group's other gauges may be stale."}, []string{"group"})
	pgReapedWriters = promauto.NewCounter(prometheus.CounterOpts{
		Name: "protection_group_reaped_writers_total",
		Help: "Writer records reaped because the owning process lease expired and the process stopped reporting."})
	pgAutomaticFrozen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "protection_group_automatic_failover_frozen",
		Help: "1 while the global switch that freezes automatic protection group failover is on."})
	pgSwitchBlocked = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "protection_group_switch_blocked_total",
		Help: "Reconcile cycles in which an automatic switch had quorum but was held back, by reason (global_freeze, rate_limit)."}, []string{"group", "reason"})
	noCredentials = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_no_credentials_total",
		Help: "Proxy requests answered 503 because no credentials resolve for the user on the storage."}, []string{"storage"})
	cpOutboxPending = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "controlplane_outbox_pending",
		Help: "Unpublished control-plane outbox entries waiting to be projected to Redis."})
	cpOutboxOldestSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "controlplane_outbox_oldest_seconds",
		Help: "Age in seconds of the oldest unpublished control-plane outbox entry."})
	cpOutboxFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "controlplane_outbox_failures_total",
		Help: "Failed attempts to publish a control-plane outbox entry to Redis."})
)

var (
	pgMu    sync.Mutex
	pgKnown = map[string]bool{}
)

// ProtectionGroupState is the per-group snapshot exported as gauges.
type ProtectionGroupState struct {
	Healthy, Wanted, MaxEventLag int
	Frozen                       bool
	ActiveWriters                int64
}

// SetProtectionGroup updates the gauges of one protection group.
func SetProtectionGroup(group string, s ProtectionGroupState) {
	pgMu.Lock()
	pgKnown[group] = true
	pgMu.Unlock()
	pgCopiesHealthy.WithLabelValues(group).Set(float64(s.Healthy))
	pgCopiesWanted.WithLabelValues(group).Set(float64(s.Wanted))
	pgMaxEventLag.WithLabelValues(group).Set(float64(s.MaxEventLag))
	pgActiveWriters.WithLabelValues(group).Set(float64(s.ActiveWriters))
	SetProtectionFrozen(group, s.Frozen)
}

// SetProtectionFrozen updates only the frozen gauge of a group.
func SetProtectionFrozen(group string, frozen bool) {
	pgMu.Lock()
	pgKnown[group] = true // so KeepProtectionGroups also removes this series once the group is deleted
	pgMu.Unlock()
	f := 0.0
	if frozen {
		f = 1
	}
	pgFrozen.WithLabelValues(group).Set(f)
}

// KeepProtectionGroups removes the gauge series of groups that no longer exist.
func KeepProtectionGroups(ids []string) {
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	pgMu.Lock()
	defer pgMu.Unlock()
	for g := range pgKnown {
		if keep[g] {
			continue
		}
		for _, v := range []*prometheus.GaugeVec{pgCopiesHealthy, pgCopiesWanted, pgMaxEventLag, pgFrozen, pgActiveWriters} {
			v.DeleteLabelValues(g)
		}
		pgReconcileErrors.DeleteLabelValues(g)
		delete(pgKnown, g)
	}
}

// CountProtectionReconcileError counts a failed reconcile cycle of a group.
func CountProtectionReconcileError(group string) {
	pgMu.Lock()
	pgKnown[group] = true
	pgMu.Unlock()
	pgReconcileErrors.WithLabelValues(group).Inc()
}

// CountProtectionPrimaryChange counts a primary change with reason "automatic" or "manual".
func CountProtectionPrimaryChange(group, reason string) {
	pgPrimaryChanges.WithLabelValues(group, reason).Inc()
}

// CountReapedWriters counts reaped writer records.
func CountReapedWriters(n int) {
	if n > 0 {
		pgReapedWriters.Add(float64(n))
	}
}

// SetAutomaticFailoverFrozen exports the global automatic failover freeze switch.
func SetAutomaticFailoverFrozen(frozen bool) {
	v := 0.0
	if frozen {
		v = 1
	}
	pgAutomaticFrozen.Set(v)
}

// CountProtectionSwitchBlocked counts an automatic switch with quorum that was held back.
func CountProtectionSwitchBlocked(group, reason string) {
	pgSwitchBlocked.WithLabelValues(group, reason).Inc()
}

// CountNoCredentials counts a request failed because the storage has no credentials for the user.
func CountNoCredentials(storage string) {
	if storage == "" {
		storage = "unknown"
	}
	noCredentials.WithLabelValues(storage).Inc()
}

// SetControlPlaneOutbox updates the control-plane outbox backlog gauges.
func SetControlPlaneOutbox(pending int64, oldest time.Duration) {
	cpOutboxPending.Set(float64(pending))
	cpOutboxOldestSeconds.Set(oldest.Seconds())
}

// CountControlPlaneOutboxFailure increments the control-plane outbox publish failure counter.
func CountControlPlaneOutboxFailure() {
	cpOutboxFailures.Inc()
}

var (
	unprotectedScopes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "replication_scopes_unprotected",
		Help: "Scopes (users or buckets) with a replication from the storage that are covered neither by an enabled legacy failover policy of the storage nor by a protection group."}, []string{"source_storage"})
	unprotectedMu    sync.Mutex
	unprotectedKnown = map[string]bool{}
)

// SetUnprotectedScopes replaces the replication_scopes_unprotected series: counts holds one entry (possibly 0)
// for every configured storage; series of storages no longer listed are removed.
func SetUnprotectedScopes(counts map[string]int) {
	unprotectedMu.Lock()
	defer unprotectedMu.Unlock()
	for s, n := range counts {
		unprotectedScopes.WithLabelValues(s).Set(float64(n))
		unprotectedKnown[s] = true
	}
	for s := range unprotectedKnown {
		if _, ok := counts[s]; !ok {
			unprotectedScopes.DeleteLabelValues(s)
			delete(unprotectedKnown, s)
		}
	}
}
