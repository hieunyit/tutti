package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/util"
)

const (
	// OnQuorumFailureFail responds with 503 if the replication quorum was not reached.
	OnQuorumFailureFail = "fail"
	// OnQuorumFailureAccept forwards the storage response even if the quorum was not reached.
	OnQuorumFailureAccept = "accept"

	// ReplicasHeader reports how many copies (main included) exist after a synchronous write.
	ReplicasHeader = "X-Tutti-Replicas"
)

// ObjectSyncer synchronizes one object to one replication destination (see service/worker/syncer).
type ObjectSyncer interface {
	Sync(ctx context.Context, p tasks.ObjectSyncPayload) error
}

// StorageHealth lets synchronous replication skip destinations known to be down and report outcomes.
// It is implemented by pkg/health.Registry.
type StorageHealth interface {
	// Available reports whether a request would be admitted, without taking a half-open trial.
	Available(storage string) bool
	// Allow admits a request (taking the half-open trial if needed).
	Allow(storage string) bool
	Observe(storage string, err error)
}

var (
	errDestUnavailable   = errors.New("destination storage is unavailable (circuit breaker open)")
	errQuorumUnreachable = errors.New("not attempted: write quorum is unreachable with the available destinations")
)

// SyncOptions configures synchronous replication on the write path.
type SyncOptions struct {
	Enabled bool
	Syncer  ObjectSyncer
	// WriteQuorum is the number of copies (main included) required before answering. 0 means all.
	WriteQuorum int
	// Timeout bounds the time spent syncing destinations per request.
	Timeout time.Duration
	// OnQuorumFailure is "fail" (default) or "accept".
	OnQuorumFailure string
	// Health is optional: destinations with an open breaker are not attempted.
	Health StorageHealth
}

// syncResult is the outcome of the synchronous replication of one request.
type syncResult struct {
	copies int
	total  int
	quorum int
	failed map[entity.UniversalReplicationID]error
}

func (o SyncOptions) active() bool { return o.Enabled && o.Syncer != nil }

// syncDestinations synchronizes every ObjectSyncPayload task to every destination of the request.
// ok is false when there was nothing to sync.
func (o SyncOptions) syncDestinations(ctx context.Context, taskList []tasks.ReplicationTask) (res syncResult, ok bool) {
	var payloads []tasks.ObjectSyncPayload
	for _, t := range taskList {
		if p, isObj := t.(*tasks.ObjectSyncPayload); isObj && p != nil {
			payloads = append(payloads, *p)
		}
	}
	dests := xctx.GetReplications(ctx)
	if len(payloads) == 0 || len(dests) == 0 {
		return res, false
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	res.total = 1 + len(dests)
	res.failed = map[entity.UniversalReplicationID]error{}
	res.quorum = o.WriteQuorum
	if res.quorum <= 0 || res.quorum > res.total {
		res.quorum = res.total
	}

	run := make([]entity.UniversalReplicationID, 0, len(dests))
	for _, id := range dests {
		if o.Health != nil && !o.Health.Available(id.ToStorage()) {
			res.failed[id] = errDestUnavailable
			continue
		}
		run = append(run, id)
	}
	res.copies = 1
	if o.Health != nil && 1+len(run) < res.quorum {
		// the quorum cannot be reached even if every available destination succeeds: do not wait for anything
		for _, id := range run {
			res.failed[id] = errQuorumUnreachable
		}
		return res, true
	}

	syncCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	synced := 0
	for _, id := range run {
		wg.Add(1)
		go func() {
			defer wg.Done()
			to := id.ToStorage()
			var err error
			if o.Health != nil && !o.Health.Allow(to) {
				err = errDestUnavailable
			} else {
				gen := xctx.GetReplicationGeneration(ctx, id)
				for _, p := range payloads {
					p.SetReplicationID(id, gen)
					err = o.Syncer.Sync(syncCtx, p)
					if o.Health != nil {
						o.Health.Observe(to, err)
					}
					if err != nil {
						break
					}
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.failed[id] = err
			} else {
				synced++
			}
		}()
	}
	wg.Wait()

	res.copies = 1 + synced
	return res, true
}

func (r syncResult) header() string { return fmt.Sprintf("%d/%d", r.copies, r.total) }

func (r syncResult) logFailures(logger *zerolog.Logger, onFailure string) {
	ids := make([]string, 0, len(r.failed))
	reasons := make([]string, 0, len(r.failed))
	for id, err := range r.failed {
		ids = append(ids, fmt.Sprintf("%s->%s", id.FromStorage(), id.ToStorage()))
		reasons = append(reasons, err.Error())
	}
	sort.Strings(ids)
	sort.Strings(reasons)
	logger.Warn().
		Int("copies", r.copies).Int("quorum", r.quorum).Int("destinations", r.total-1).
		Str("on_quorum_failure", onFailure).
		Str("failed_destinations", strings.Join(ids, ",")).
		Str("errors", strings.Join(reasons, "; ")).
		Msg("sync replication: not all destinations are up to date, remaining replication is asynchronous")
}

func writeQuorumError(ctx context.Context, w http.ResponseWriter) {
	util.WriteError(ctx, w, minio.ErrorResponse{
		Code:       "ServiceUnavailable",
		Message:    "The write was stored on the primary storage but the replication quorum was not reached; the change will be replicated asynchronously. Retry the request to obtain the guarantee.",
		BucketName: xctx.GetBucket(ctx),
		Key:        xctx.GetObject(ctx),
		StatusCode: http.StatusServiceUnavailable,
	})
}
