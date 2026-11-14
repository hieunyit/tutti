package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// InitBytesRateWindow is the rolling window used to compute the initial replication throughput.
	InitBytesRateWindow = 5 * time.Minute

	initBytesBucket    = time.Minute
	initBytesRateTTL   = 15 * time.Minute
	initBytesStatsTTL  = 30 * 24 * time.Hour
	initBytesKeyPrefix = "p:repl_init_bytes:"
)

// InitBytes is a point-in-time view of initial replication progress measured in bytes.
type InitBytes struct {
	// Total is the sum of sizes of objects enumerated by the listing so far.
	Total int64
	// Done is the sum of sizes of objects whose copy completed.
	Done int64
	// BytesPerSecond is the completion-based throughput over the rolling window, 0 if unknown/idle.
	BytesPerSecond int64
}

// InitBytesStore tracks initial replication bytes in Redis. It is independent of the control plane backend:
// like queue statistics it is runtime data, so it lives in Redis in both Redis-only and PostgreSQL modes.
//
// Keys are scoped by replication ID and generation, so re-creating a replication starts from zero.
//
//	p:repl_init_bytes:{<id>:<gen>}       hash {total, done, first}
//	p:repl_init_bytes:{<id>:<gen>}:rate  hash {<unix minute start>: bytes completed in that minute}
type InitBytesStore struct {
	client redis.Cmdable
}

func NewInitBytesStore(client redis.Cmdable) *InitBytesStore {
	return &InitBytesStore{client: client}
}

func initBytesKeys(replID string, generation int64) (stats, rate string) {
	base := fmt.Sprintf("%s{%s:%d}", initBytesKeyPrefix, replID, generation)
	return base, base + ":rate"
}

// AddTotal registers bytes of objects enumerated by the listing (call once per enqueued object).
func (s *InitBytesStore) AddTotal(ctx context.Context, replID string, generation, n int64) error {
	if n <= 0 {
		return nil
	}
	stats, _ := initBytesKeys(replID, generation)
	pipe := s.client.Pipeline()
	pipe.HIncrBy(ctx, stats, "total", n)
	pipe.Expire(ctx, stats, initBytesStatsTTL)
	_, err := pipe.Exec(ctx)
	return err
}

var initBytesDoneScript = redis.NewScript(`
redis.call('HINCRBY', KEYS[1], 'done', ARGV[1])
redis.call('HSETNX', KEYS[1], 'first', ARGV[4])
redis.call('EXPIRE', KEYS[1], ARGV[5])
redis.call('HINCRBY', KEYS[2], ARGV[2], ARGV[1])
redis.call('EXPIRE', KEYS[2], ARGV[6])
local cutoff = tonumber(ARGV[3])
for _, f in ipairs(redis.call('HKEYS', KEYS[2])) do
  if tonumber(f) < cutoff then redis.call('HDEL', KEYS[2], f) end
end
return 1
`)

// AddDone registers bytes of an object whose copy completed. It atomically bumps the done counter and the
// per-minute throughput bucket.
func (s *InitBytesStore) AddDone(ctx context.Context, replID string, generation, n int64, now time.Time) error {
	if n < 0 {
		return nil
	}
	stats, rate := initBytesKeys(replID, generation)
	minute := now.Truncate(initBytesBucket).Unix()
	cutoff := now.Add(-InitBytesRateWindow).Truncate(initBytesBucket).Unix()
	return initBytesDoneScript.Run(ctx, s.client, []string{stats, rate},
		n, minute, cutoff, now.Unix(), int64(initBytesStatsTTL/time.Second), int64(initBytesRateTTL/time.Second)).Err()
}

// Get returns the current counters and the rolling throughput. A missing key yields zero values.
func (s *InitBytesStore) Get(ctx context.Context, replID string, generation int64, now time.Time) (InitBytes, error) {
	stats, rate := initBytesKeys(replID, generation)
	pipe := s.client.Pipeline()
	statsCmd := pipe.HGetAll(ctx, stats)
	rateCmd := pipe.HGetAll(ctx, rate)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return InitBytes{}, err
	}
	var res InitBytes
	st := statsCmd.Val()
	res.Total, _ = strconv.ParseInt(st["total"], 10, 64)
	res.Done, _ = strconv.ParseInt(st["done"], 10, 64)
	first, _ := strconv.ParseInt(st["first"], 10, 64)
	res.BytesPerSecond = rollingRate(rateCmd.Val(), first, now)
	return res, nil
}

// rollingRate divides bytes completed in the window by the elapsed time of the window, but never by more time
// than passed since the first completion (so a fresh replication is not under-reported).
func rollingRate(buckets map[string]string, first int64, now time.Time) int64 {
	windowStart := now.Add(-InitBytesRateWindow).Truncate(initBytesBucket).Unix()
	var sum int64
	for f, v := range buckets {
		ts, err := strconv.ParseInt(f, 10, 64)
		if err != nil || ts < windowStart {
			continue
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		sum += n
	}
	if sum <= 0 || first <= 0 {
		return 0
	}
	start := windowStart
	if first > start {
		start = first
	}
	elapsed := now.Unix() - start
	if elapsed < 10 {
		elapsed = 10
	}
	return sum / elapsed
}
