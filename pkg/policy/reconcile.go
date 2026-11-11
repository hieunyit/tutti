package policy

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/entity"
)

// ReconcileResult summarizes the replication reconciliation run.
type ReconcileResult struct {
	Checked int
	Resumed int
	Errors  int
}

// ReconcileReplications iterates over all user and bucket replications, checking their status
// and resuming active queues or clearing pending failover locks that need reconciliation.
func ReconcileReplications(ctx context.Context, p Service, users func() []string) (ReconcileResult, error) {
	var res ReconcileResult
	logger := zerolog.Ctx(ctx)

	// 1. User replications
	userRepls, err := p.ListUserReplicationsInfo(ctx)
	if err != nil {
		return res, err
	}
	for pol, st := range userRepls {
		res.Checked++
		uid := entity.UniversalFromUserReplication(pol)
		if st.ReplicationStatus != nil && !st.IsArchived && st.FailoverPending {
			logger.Info().Str("replication", uid.String()).Msg("reconciling failover pending user replication")
			if err := p.ResumeReplication(ctx, uid); err != nil {
				res.Errors++
				logger.Warn().Err(err).Str("replication", uid.String()).Msg("failed to resume failover pending replication")
			} else {
				res.Resumed++
			}
		}
	}

	// 2. Bucket replications
	seen := map[string]bool{}
	for _, u := range users() {
		if seen[u] {
			continue
		}
		seen[u] = true
		bucketRepls, err := p.ListBucketReplicationsInfo(ctx, u)
		if err != nil {
			res.Errors++
			continue
		}
		for pol, st := range bucketRepls {
			res.Checked++
			uid := entity.UniversalFromBucketReplication(pol)
			if st.ReplicationStatus != nil && !st.IsArchived && st.FailoverPending {
				logger.Info().Str("replication", uid.String()).Msg("reconciling failover pending bucket replication")
				if err := p.ResumeReplication(ctx, uid); err != nil {
					res.Errors++
					logger.Warn().Err(err).Str("replication", uid.String()).Msg("failed to resume bucket replication")
				} else {
					res.Resumed++
				}
			}
		}
	}

	return res, nil
}

// RunReplicationReconciler periodically reconciles all configured replications until ctx is canceled.
func RunReplicationReconciler(ctx context.Context, p Service, users func() []string, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := ReconcileReplications(ctx, p, users); err != nil && onError != nil && ctx.Err() == nil {
				onError(err)
			}
		}
	}
}
