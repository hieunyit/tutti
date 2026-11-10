// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

// publisherLockID serializes publishers: only one process applies the outbox at a time, which keeps per-object order.
const publisherLockID int64 = 0x6f757462 // "outb"

// bumpScript is the script of pkg/policy.bumpScript: INCR the snapshot version and PUBLISH it.
const bumpScript = `local v = redis.call('incr', KEYS[1]); redis.call('publish', ARGV[1], tostring(v)); return v`

// Publisher applies committed outbox entries to Redis: it writes the projected keys and bumps the version keys and
// publishes the notifications that proxies and workers already watch (tutti:cfg:version / tutti:cfg:changed,
// tutti:storages:version, tutti:s3users:version, the version field of chorus:dynamic_creds).
//
// Guarantees:
//   - after commit: entries are written in the same transaction as the change, the publisher only sees committed rows;
//   - ordered per object: entries of one object_key are applied in id order, and a failed entry holds back the later
//     entries of its object (not of other objects) until it succeeds;
//   - idempotent: data operations set absolute state, so a retry after a partial failure converges; counters are
//     bumped at-least-once, which only makes pollers reload once more;
//   - retried: failures are recorded (attempts, last_error) and retried with exponential backoff.
type Publisher struct {
	Store *Store
	Redis redis.UniversalClient
	Log   zerolog.Logger
	cfg   controlplane.OutboxConfig

	mu sync.Mutex
}

// NewPublisher creates a publisher.
func NewPublisher(store *Store, rdb redis.UniversalClient, cfg controlplane.OutboxConfig, log zerolog.Logger) *Publisher {
	return &Publisher{Store: store, Redis: rdb, Log: log, cfg: controlplane.Config{Outbox: cfg}.WithDefaults().Outbox}
}

// Result of one RunOnce cycle.
type Result struct {
	Published int
	Failed    int
	// Skipped entries were held back behind a failed entry of the same object or are waiting for their retry time.
	Skipped int
	// Busy is true if another publisher holds the lock.
	Busy bool
}

type outboxRow struct {
	id       int64
	key      string
	ops      []byte
	attempts int
	next     time.Time
}

// RunOnce applies up to BatchSize pending entries.
func (p *Publisher) RunOnce(ctx context.Context) (Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var res Result
	tx, err := p.Store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var got bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, publisherLockID).Scan(&got); err != nil {
		return res, err
	}
	if !got {
		res.Busy = true
		return res, nil
	}
	rows, err := tx.Query(ctx, `SELECT id, object_key, ops, attempts, next_attempt_at FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT $1`, p.cfg.BatchSize)
	if err != nil {
		return res, err
	}
	var pending []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.id, &r.key, &r.ops, &r.attempts, &r.next); err != nil {
			rows.Close()
			return res, err
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	now := time.Now()
	blocked := map[string]bool{}
	for _, r := range pending {
		if blocked[r.key] || r.next.After(now) {
			blocked[r.key] = true
			res.Skipped++
			continue
		}
		if err := p.apply(ctx, r.ops); err != nil {
			blocked[r.key] = true
			res.Failed++
			p.Log.Warn().Err(err).Int64("outbox_id", r.id).Str("object", r.key).Int("attempts", r.attempts+1).Msg("control plane outbox: publish failed, will retry")
			delay := p.cfg.RetryBackoff << min(r.attempts, 20)
			if delay <= 0 || delay > p.cfg.MaxBackoff {
				delay = p.cfg.MaxBackoff
			}
			if _, uerr := tx.Exec(ctx, `UPDATE outbox SET attempts=attempts+1, last_error=$2, next_attempt_at=now()+$3::interval WHERE id=$1`,
				r.id, truncate(err.Error(), 500), fmt.Sprintf("%d milliseconds", delay.Milliseconds())); uerr != nil {
				return res, uerr
			}
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at=now(), last_error='' WHERE id=$1`, r.id); err != nil {
			return res, err
		}
		res.Published++
	}
	return res, tx.Commit(ctx)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// apply runs the ops of one entry in a MULTI/EXEC.
func (p *Publisher) apply(ctx context.Context, raw []byte) error {
	var ops []RedisOp
	if err := json.Unmarshal(raw, &ops); err != nil {
		return fmt.Errorf("decode ops: %w", err)
	}
	_, err := p.Redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, op := range ops {
			if err := queue(ctx, pipe, op); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func queue(ctx context.Context, pipe redis.Pipeliner, op RedisOp) error {
	switch op.Op {
	case OpHSet:
		pipe.HSet(ctx, op.Key, op.Field, op.Value)
	case OpHSetNX:
		pipe.HSetNX(ctx, op.Key, op.Field, op.Value)
	case OpHDel:
		pipe.HDel(ctx, op.Key, op.Field)
	case OpHMSet:
		if len(op.Fields) > 0 {
			pipe.HSet(ctx, op.Key, toAny(op.Fields))
		}
		if len(op.Del) > 0 {
			pipe.HDel(ctx, op.Key, op.Del...)
		}
	case OpHReplace:
		pipe.Del(ctx, op.Key)
		if len(op.Fields) > 0 {
			pipe.HSet(ctx, op.Key, toAny(op.Fields))
		}
	case OpSet:
		pipe.Set(ctx, op.Key, op.Value, 0)
	case OpDel:
		pipe.Del(ctx, op.Key)
	case OpSAdd:
		pipe.SAdd(ctx, op.Key, toAnySlice(op.Members)...)
	case OpSRem:
		pipe.SRem(ctx, op.Key, toAnySlice(op.Members)...)
	case OpSReplace:
		pipe.Del(ctx, op.Key)
		if len(op.Members) > 0 {
			pipe.SAdd(ctx, op.Key, toAnySlice(op.Members)...)
		}
	case OpIncr:
		pipe.Incr(ctx, op.Key)
	case OpHIncr:
		pipe.HIncrBy(ctx, op.Key, op.Field, 1)
	case OpSnapshot:
		pipe.Eval(ctx, bumpScript, []string{SnapshotVersionKey}, SnapshotChannel)
	default:
		return fmt.Errorf("unknown outbox op %q", op.Op)
	}
	return nil
}

func toAny(m map[string]string) map[string]any {
	r := make(map[string]any, len(m))
	for k, v := range m {
		r[k] = v
	}
	return r
}

func toAnySlice(s []string) []any {
	r := make([]any, len(s))
	for i, v := range s {
		r[i] = v
	}
	return r
}

// Purge deletes published entries older than the retention. It returns the number of rows removed.
func (p *Publisher) Purge(ctx context.Context) (int64, error) {
	tag, err := p.Store.pool.Exec(ctx, `DELETE FROM outbox WHERE published_at IS NOT NULL AND published_at < now() - $1::interval`,
		fmt.Sprintf("%d seconds", int64(p.cfg.Retention.Seconds())))
	return tag.RowsAffected(), err
}

// Pending returns the number of unpublished entries and the age of the oldest one (for metrics and alerts).
func (p *Publisher) Pending(ctx context.Context) (n int64, oldest time.Duration, err error) {
	var age *float64
	err = p.Store.pool.QueryRow(ctx, `SELECT count(*), EXTRACT(EPOCH FROM now()-min(created_at)) FROM outbox WHERE published_at IS NULL`).Scan(&n, &age)
	if age != nil {
		oldest = time.Duration(*age * float64(time.Second))
	}
	return
}

// Run publishes until ctx is done.
func (p *Publisher) Run(ctx context.Context) {
	t := time.NewTicker(p.cfg.PollInterval)
	defer t.Stop()
	purge := time.NewTicker(time.Hour)
	defer purge.Stop()
	for {
		if res, err := p.RunOnce(ctx); err != nil && ctx.Err() == nil {
			p.Log.Error().Err(err).Msg("control plane outbox: cycle failed")
		} else if res.Published > 0 && !res.Busy {
			continue // more may be waiting, do not sleep
		}
		select {
		case <-ctx.Done():
			return
		case <-purge.C:
			if _, err := p.Purge(ctx); err != nil && ctx.Err() == nil {
				p.Log.Warn().Err(err).Msg("control plane outbox: purge failed")
			}
		case <-t.C:
		}
	}
}
