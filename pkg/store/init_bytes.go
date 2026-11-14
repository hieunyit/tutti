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
	InitBytesRateWindow = 3 * time.Minute
	// InitBytesMaxProgress caps the number of objects with tracked in-progress bytes per replication.
	InitBytesMaxProgress = 1000

	initBytesBucket    = time.Minute
	initBytesRateTTL   = 15 * time.Minute
	initBytesStatsTTL  = 30 * 24 * time.Hour
	initBytesProgTTL   = 2 * time.Hour // same safety expiry as the in-flight activity entries
	initBytesKeyPrefix = "p:repl_init_bytes:"
	// initBytesProgSum is the reserved field of the progress hash holding the sum of all progress fields.
	// Object fields are "<key>\x00<versionId>" with a non empty key, so they never equal it.
	initBytesProgSum = "\x00sum"
)

// InitBytes is a point-in-time view of initial replication progress measured in bytes.
type InitBytes struct {
	// Total is the sum of sizes of objects enumerated by the listing so far.
	Total int64
	// Done is the sum of sizes of objects whose copy completed plus the bytes already transferred of copies
	// that are still running (never more than Total when Total is known).
	Done int64
	// BytesPerSecond is the throughput of bytes actually transferred over the rolling window, 0 if idle.
	BytesPerSecond int64
}

// InitBytesStore tracks initial replication bytes in Redis. It is independent of the control plane backend:
// like queue statistics it is runtime data, so it lives in Redis in both Redis-only and PostgreSQL modes.
//
// Keys are scoped by replication ID and generation, so re-creating a replication starts from zero.
//
//	p:repl_init_bytes:{<id>:<gen>}       hash {total, done, first}
//	p:repl_init_bytes:{<id>:<gen>}:rate  hash {<unix minute start>: bytes transferred in that minute}
//	p:repl_init_bytes:{<id>:<gen>}:prog  hash {"<key>\x00<versionId>": bytes transferred so far (absolute),
//	                                          "\x00sum": sum of all fields}
type InitBytesStore struct {
	client redis.Cmdable
}

func NewInitBytesStore(client redis.Cmdable) *InitBytesStore {
	return &InitBytesStore{client: client}
}

func initBytesKeys(replID string, generation int64) (stats, rate, prog string) {
	base := fmt.Sprintf("%s{%s:%d}", initBytesKeyPrefix, replID, generation)
	return base, base + ":rate", base + ":prog"
}

// AddTotal registers bytes of objects enumerated by the listing (call once per enqueued object).
func (s *InitBytesStore) AddTotal(ctx context.Context, replID string, generation, n int64) error {
	if n <= 0 {
		return nil
	}
	stats, _, _ := initBytesKeys(replID, generation)
	pipe := s.client.Pipeline()
	pipe.HIncrBy(ctx, stats, "total", n)
	pipe.Expire(ctx, stats, initBytesStatsTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// rateLua adds ARGV delta bytes to the per-minute bucket. The caller sets: delta, minute, cutoff, now, rateTTL, statsTTL.
// "first" is (re)set when the bucket hash is new so that an idle period does not dilute the speed.
const initBytesRateLua = `
local function addRate(delta)
  if delta <= 0 then return end
  if redis.call('EXISTS', KEYS[2]) == 0 then redis.call('HSET', KEYS[1], 'first', ARGV[%[1]d]) end
  redis.call('HSETNX', KEYS[1], 'first', ARGV[%[1]d])
  redis.call('EXPIRE', KEYS[1], ARGV[%[2]d])
  redis.call('HINCRBY', KEYS[2], ARGV[%[3]d], delta)
  redis.call('EXPIRE', KEYS[2], ARGV[%[4]d])
  local cutoff = tonumber(ARGV[%[5]d])
  for _, f in ipairs(redis.call('HKEYS', KEYS[2])) do
    if tonumber(f) < cutoff then redis.call('HDEL', KEYS[2], f) end
  end
end
`

// KEYS: stats, rate, prog. ARGV: field, bytes, size, now, statsTTL, minute, cutoff, rateTTL, progTTL, cap, sumField.
var initBytesProgressScript = redis.NewScript(fmt.Sprintf(initBytesRateLua, 4, 5, 6, 8, 7) + `
local cur = redis.call('HGET', KEYS[3], ARGV[1])
local b = tonumber(ARGV[2])
local size = tonumber(ARGV[3])
if b < 0 then b = 0 end
if size > 0 and b > size then b = size end
if not cur and redis.call('HLEN', KEYS[3]) > tonumber(ARGV[10]) then return 0 end
local old = tonumber(cur or '0')
redis.call('HSET', KEYS[3], ARGV[1], b)
redis.call('HINCRBY', KEYS[3], ARGV[11], b - old)
redis.call('EXPIRE', KEYS[3], ARGV[9])
addRate(b - old)
return 1
`)

// KEYS: stats, rate, prog. ARGV: field, size, now, statsTTL, minute, cutoff, rateTTL, sumField.
var initBytesDoneScript = redis.NewScript(fmt.Sprintf(initBytesRateLua, 3, 4, 5, 7, 6) + `
local size = tonumber(ARGV[2])
local old = 0
local cur = redis.call('HGET', KEYS[3], ARGV[1])
if cur then
  old = tonumber(cur)
  redis.call('HDEL', KEYS[3], ARGV[1])
  redis.call('HINCRBY', KEYS[3], ARGV[8], -old)
  if redis.call('HLEN', KEYS[3]) <= 1 then redis.call('DEL', KEYS[3]) end
end
redis.call('HINCRBY', KEYS[1], 'done', size)
redis.call('EXPIRE', KEYS[1], ARGV[4])
addRate(size - old)
return 1
`)

// KEYS: prog. ARGV: field, sumField.
var initBytesClearScript = redis.NewScript(`
local cur = redis.call('HGET', KEYS[1], ARGV[1])
if cur then
  redis.call('HDEL', KEYS[1], ARGV[1])
  redis.call('HINCRBY', KEYS[1], ARGV[2], -tonumber(cur))
  if redis.call('HLEN', KEYS[1]) <= 1 then redis.call('DEL', KEYS[1]) end
end
return 1
`)

// ProgressField is the field of an object in the progress hash.
func ProgressField(key, versionID string) string { return activityField(key, versionID) }

// SetProgress records that bytes of the object (absolute, since the copy attempt started) were transferred so
// far. It is idempotent: a retry that restarts from 0 simply lowers the value. Only positive growth counts
// for the throughput. size caps the stored value; new objects are ignored once InitBytesMaxProgress objects
// are tracked.
func (s *InitBytesStore) SetProgress(ctx context.Context, replID string, generation int64, field string, bytes, size int64, now time.Time) error {
	stats, rate, prog := initBytesKeys(replID, generation)
	minute := now.Truncate(initBytesBucket).Unix()
	cutoff := now.Add(-InitBytesRateWindow).Truncate(initBytesBucket).Unix()
	return initBytesProgressScript.Run(ctx, s.client, []string{stats, rate, prog},
		field, bytes, size, now.Unix(), int64(initBytesStatsTTL/time.Second), minute, cutoff,
		int64(initBytesRateTTL/time.Second), int64(initBytesProgTTL/time.Second), InitBytesMaxProgress, initBytesProgSum).Err()
}

// ClearProgress drops the in-progress bytes of an object whose attempt ended without completing it.
func (s *InitBytesStore) ClearProgress(ctx context.Context, replID string, generation int64, field string) error {
	_, _, prog := initBytesKeys(replID, generation)
	return initBytesClearScript.Run(ctx, s.client, []string{prog}, field, initBytesProgSum).Err()
}

// AddDone registers an object whose copy completed. In one atomic step it removes the object's progress,
// adds its size to the done counter and adds the not yet reported remainder to the throughput bucket.
func (s *InitBytesStore) AddDone(ctx context.Context, replID string, generation, n int64, field string, now time.Time) error {
	if n < 0 {
		return nil
	}
	stats, rate, prog := initBytesKeys(replID, generation)
	minute := now.Truncate(initBytesBucket).Unix()
	cutoff := now.Add(-InitBytesRateWindow).Truncate(initBytesBucket).Unix()
	return initBytesDoneScript.Run(ctx, s.client, []string{stats, rate, prog},
		field, n, now.Unix(), int64(initBytesStatsTTL/time.Second), minute, cutoff,
		int64(initBytesRateTTL/time.Second), initBytesProgSum).Err()
}

// Get returns the current counters and the rolling throughput. A missing key yields zero values.
func (s *InitBytesStore) Get(ctx context.Context, replID string, generation int64, now time.Time) (InitBytes, error) {
	stats, rate, prog := initBytesKeys(replID, generation)
	pipe := s.client.Pipeline()
	statsCmd := pipe.HGetAll(ctx, stats)
	rateCmd := pipe.HGetAll(ctx, rate)
	progCmd := pipe.HGet(ctx, prog, initBytesProgSum)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return InitBytes{}, err
	}
	var res InitBytes
	st := statsCmd.Val()
	res.Total, _ = strconv.ParseInt(st["total"], 10, 64)
	res.Done, _ = strconv.ParseInt(st["done"], 10, 64)
	if inProgress, _ := strconv.ParseInt(progCmd.Val(), 10, 64); inProgress > 0 {
		res.Done += inProgress
	}
	if res.Total > 0 && res.Done > res.Total {
		res.Done = res.Total
	}
	first, _ := strconv.ParseInt(st["first"], 10, 64)
	res.BytesPerSecond = rollingRate(rateCmd.Val(), first, now)
	return res, nil
}

// rollingRate divides bytes transferred in the window by the elapsed time of the window, but never by more time
// than passed since the first transfer (so a fresh replication is not under-reported).
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
