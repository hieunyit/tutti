package metrics

import "github.com/prometheus/client_golang/prometheus/promauto"
import "github.com/prometheus/client_golang/prometheus"

var cpPostgresReady = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "controlplane_postgres_ready",
	Help: "1 when the control-plane PostgreSQL store is connected (and migrated), 0 while the service runs degraded and read-only because the database was unreachable at start."})

// SetControlPlanePostgresReady exports the readiness of the control-plane PostgreSQL store.
func SetControlPlanePostgresReady(ready bool) {
	if ready {
		cpPostgresReady.Set(1)
		return
	}
	cpPostgresReady.Set(0)
}
