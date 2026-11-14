package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	copyObjectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tutti_copy_objects_total",
		Help: "Objects copied by the worker, by result (success or failure)."}, []string{"result"})
	copyBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tutti_copy_bytes_total",
		Help: "Bytes copied by the worker."})
	copyDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tutti_copy_duration_seconds",
		Help:    "Duration of one object copy.",
		Buckets: []float64{.01, .05, .1, .5, 1, 5, 15, 60, 300, 900},
	})
	copyInflight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tutti_copy_inflight",
		Help: "Object copies running now."})
	copyBufferWait = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tutti_copy_buffer_wait_seconds_total",
		Help: "Total time copies spent blocked waiting for the buffer budget."})
	copyBufferUsed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tutti_copy_buffer_used_bytes",
		Help: "Buffer budget bytes reserved by running copies."})
	copyBufferTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tutti_copy_buffer_total_bytes",
		Help: "Buffer budget size in bytes (0 = unlimited)."})
)

// CopyStarted and CopyFinished track the in-flight gauge.
func CopyStarted()  { copyInflight.Inc() }
func CopyFinished() { copyInflight.Dec() }

// ObserveCopy records one finished copy.
func ObserveCopy(success bool, bytes int64, d time.Duration) {
	copyDuration.Observe(d.Seconds())
	if success {
		copyObjectsTotal.WithLabelValues("success").Inc()
		if bytes > 0 {
			copyBytesTotal.Add(float64(bytes))
		}
		return
	}
	copyObjectsTotal.WithLabelValues("failure").Inc()
}

// AddCopyBufferWait adds time spent blocked in the buffer budget.
func AddCopyBufferWait(d time.Duration) { copyBufferWait.Add(d.Seconds()) }

// SetCopyBuffer exports the buffer budget usage.
func SetCopyBuffer(used, total int64) {
	copyBufferUsed.Set(float64(used))
	copyBufferTotal.Set(float64(total))
}
