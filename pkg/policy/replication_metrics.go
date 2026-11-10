package policy

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/hieunyit/tutti/pkg/entity"
)

// replicationUnprocessed is aggregated per (user, from, to). Bucket-level replications can number in the
// thousands per user, so their lag is summed into the user's series instead of getting a series per bucket:
// the label set is bounded by users x storage pairs. Drill down with chorctl repl.
var replicationUnprocessed = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "replication_unprocessed_events",
	Help: "Unprocessed replication events (queued or running) summed over the user and bucket replications of a user between two storages."},
	[]string{"user", "from", "to"})

// ReplicationLister is the part of the policy service read by the replication metrics collector.
type ReplicationLister interface {
	ListUserReplicationsInfo(ctx context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error)
	ListBucketReplicationsInfo(ctx context.Context, user string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error)
}

type replKey struct{ user, from, to string }

// CollectReplicationMetrics reads all replications once and replaces the replication_unprocessed_events series.
// users lists the users that may own bucket replications.
func CollectReplicationMetrics(ctx context.Context, l ReplicationLister, users func() []string) error {
	sums := map[replKey]float64{}
	ur, err := l.ListUserReplicationsInfo(ctx)
	if err != nil {
		return err
	}
	for p, st := range ur {
		sums[replKey{p.User, p.FromStorage, p.ToStorage}] += float64(st.EventMigration.Unprocessed)
	}
	seen := map[string]bool{}
	for _, u := range users() {
		if seen[u] {
			continue
		}
		seen[u] = true
		br, err := l.ListBucketReplicationsInfo(ctx, u)
		if err != nil {
			return err
		}
		for p, st := range br {
			sums[replKey{p.User, p.FromStorage, p.ToStorage}] += float64(st.EventMigration.Unprocessed)
		}
	}
	replicationUnprocessed.Reset()
	for k, v := range sums {
		replicationUnprocessed.WithLabelValues(k.user, k.from, k.to).Set(v)
	}
	return nil
}

// RunReplicationMetrics refreshes the replication metrics until ctx is done.
func RunReplicationMetrics(ctx context.Context, l ReplicationLister, users func() []string, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := CollectReplicationMetrics(ctx, l, users); err != nil && onError != nil && ctx.Err() == nil {
			onError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
