// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package failover

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

// With controlPlane.backend=postgres PostgreSQL is the source of truth of routing and failover events. Proxies have no
// PostgreSQL connection, so an automatic failover decided by a proxy must not write routing to Redis itself (the next
// PostgreSQL-based write or republish would silently revert it to the failed storage). Instead the proxy hands the
// decision to a worker over a Redis list and waits for the answer; the worker applies it through the control plane
// (policy.FailoverReplicationWithEvent: routing and event in one PostgreSQL transaction, then the Redis side).
//
// The worker publishes its backend under BackendKey on start, which is how proxies learn (without configuration) that
// they must delegate.
const (
	// BackendKey holds "postgres" or "redis", written by the workers.
	BackendKey = "tutti:fo-exec:backend"
	// RequestsKey is the list of pending failover requests (RPUSH by proxies, BLPOP by workers).
	RequestsKey = "tutti:fo-exec:requests"
	replyPrefix = "tutti:fo-exec:reply:"

	backendPostgres = "postgres"
	// RequestTTL is how long a request is valid: a stale request (nobody served it in time) is dropped by the worker,
	// the proxy controller re-evaluates and asks again if the storage is still down.
	RequestTTL = 60 * time.Second
	// DefaultWait is how long a proxy waits for the worker's answer.
	DefaultWait = 20 * time.Second
)

// ErrNoExecutor is returned by Remote.Execute when no worker answered in time.
var ErrNoExecutor = fmt.Errorf("%w: no worker confirmed the failover in time (is a worker running and connected to the control plane?)", dom.ErrUnavailable)

// request is one queued failover decision.
type request struct {
	ID          string    `json:"id"`
	Replication string    `json:"replication"`
	Event       Event     `json:"event"`
	Expires     time.Time `json:"expires"`
}

type reply struct {
	Error string `json:"error,omitempty"`
	// Kind is "invalid" (dom.ErrInvalidArg), "precondition" (dom.ErrFailedPrecondition), "unavailable" or "".
	Kind string `json:"kind,omitempty"`
}

// PublishBackend records the control-plane backend of the cluster ("postgres" or "redis"). Workers call it on start.
func PublishBackend(ctx context.Context, rdb redis.Cmdable, backend string) error {
	return rdb.Set(ctx, BackendKey, backend, 0).Err()
}

// Remote lets a proxy delegate failovers to a worker.
type Remote struct {
	Client redis.Cmdable
	// Wait overrides DefaultWait.
	Wait time.Duration
}

// Required reports whether failovers must be delegated, i.e. the cluster uses the PostgreSQL control plane.
func (r Remote) Required(ctx context.Context) (bool, error) {
	v, err := r.Client.Get(ctx, BackendKey).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v == backendPostgres, nil
}

// Execute queues the failover of the replication (entity.UniversalReplicationID.AsString) with its event and waits for
// the worker's result. A nil error means the failover is applied (routing and event are committed).
func (r Remote) Execute(ctx context.Context, replication string, ev Event) error {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	req := request{ID: hex.EncodeToString(b), Replication: replication, Event: ev, Expires: time.Now().Add(RequestTTL)}
	raw, err := json.Marshal(req)
	if err != nil {
		return err
	}
	wait := r.Wait
	if wait <= 0 {
		wait = DefaultWait
	}
	if err := r.Client.RPush(ctx, RequestsKey, raw).Err(); err != nil {
		return fmt.Errorf("failover: unable to queue request: %w", err)
	}
	res, err := r.Client.BLPop(ctx, wait, replyPrefix+req.ID).Result()
	if errors.Is(err, redis.Nil) {
		return ErrNoExecutor
	}
	if err != nil {
		return fmt.Errorf("failover: waiting for the worker: %w", err)
	}
	var rep reply
	if len(res) != 2 || json.Unmarshal([]byte(res[1]), &rep) != nil {
		return fmt.Errorf("failover: malformed worker reply")
	}
	if rep.Error == "" {
		return nil
	}
	switch rep.Kind {
	case "invalid":
		return fmt.Errorf("%w: %s", dom.ErrInvalidArg, rep.Error)
	case "precondition":
		return fmt.Errorf("%w: %s", dom.ErrFailedPrecondition, rep.Error)
	case "unavailable":
		return fmt.Errorf("%w: %s", dom.ErrUnavailable, rep.Error)
	}
	return errors.New(rep.Error)
}

// Applier is implemented by the policy service.
type Applier interface {
	FailoverReplicationWithEvent(ctx context.Context, id entity.UniversalReplicationID, ev *controlplane.FailoverEvent) error
}

// RunExecutor serves failover requests until ctx is done (workers, PostgreSQL backend). Every worker may run it:
// BLPOP hands each request to exactly one.
func RunExecutor(ctx context.Context, rdb redis.UniversalClient, a Applier, log zerolog.Logger) {
	for ctx.Err() == nil {
		res, err := rdb.BLPop(ctx, 2*time.Second, RequestsKey).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Warn().Err(err).Msg("failover executor: unable to read requests")
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		if len(res) != 2 {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(res[1]), &req); err != nil {
			log.Warn().Err(err).Msg("failover executor: dropping malformed request")
			continue
		}
		if time.Now().After(req.Expires) {
			log.Warn().Str("replication", req.Replication).Msg("failover executor: dropping an expired request")
			continue
		}
		rep := execute(ctx, a, req, log)
		raw, _ := json.Marshal(rep)
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		pipe := rdb.TxPipeline()
		pipe.RPush(rctx, replyPrefix+req.ID, raw)
		pipe.Expire(rctx, replyPrefix+req.ID, RequestTTL)
		if _, err := pipe.Exec(rctx); err != nil {
			log.Warn().Err(err).Str("replication", req.Replication).Msg("failover executor: unable to store the reply")
		}
		cancel()
	}
}

func execute(ctx context.Context, a Applier, req request, log zerolog.Logger) reply {
	id, err := entity.UniversalIDFromString(req.Replication)
	if err != nil {
		return reply{Error: err.Error(), Kind: "invalid"}
	}
	ev := toCPEvent(req.Event)
	ctx = controlplane.WithActor(ctx, controlplane.Actor{Name: "failover-controller"})
	if err := a.FailoverReplicationWithEvent(ctx, id, &ev); err != nil {
		log.Warn().Err(err).Str("replication", req.Replication).Msg("failover executor: failover failed")
		rep := reply{Error: err.Error()}
		switch {
		case errors.Is(err, dom.ErrInvalidArg):
			rep.Kind = "invalid"
		case errors.Is(err, dom.ErrFailedPrecondition), errors.Is(err, controlplane.ErrReadOnly):
			rep.Kind = "precondition"
		case errors.Is(err, dom.ErrUnavailable):
			rep.Kind = "unavailable"
		}
		return rep
	}
	log.Warn().Str("replication", req.Replication).Str("from", req.Event.From).Str("to", req.Event.To).Msg("failover executor: failover applied through the control plane")
	return reply{}
}
