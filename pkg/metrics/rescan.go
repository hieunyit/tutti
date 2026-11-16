package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rescanScansTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tutti_rescan_scans_total",
		Help: "Periodic source rescans finished by result (success, failed_copies or aborted)."}, []string{"result"})
	rescanObjectsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tutti_rescan_objects_copied_total",
		Help: "Objects copied by periodic source rescans."})
	rescanLastDuration = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tutti_rescan_last_duration_seconds",
		Help: "Duration of the last finished rescan seen by this worker."})
	rescanStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tutti_rescan_started_total",
		Help: "Periodic source rescans started by this process."})
)

// RescanStarted counts one scan started by this process (scheduler or Sync now).
func RescanStarted() { rescanStarted.Inc() }

// RescanFinished records a scan that listed everything. success is false when some copies failed for good.
func RescanFinished(success bool, copied int64, d time.Duration) {
	result := "success"
	if !success {
		result = "failed_copies"
	}
	rescanScansTotal.WithLabelValues(result).Inc()
	rescanLastDuration.Set(d.Seconds())
}

// RescanAborted counts a scan that ended on a listing error.
func RescanAborted() { rescanScansTotal.WithLabelValues("aborted").Inc() }

// RescanObjectCopied counts one object transferred by a rescan.
func RescanObjectCopied() { rescanObjectsTotal.Inc() }
