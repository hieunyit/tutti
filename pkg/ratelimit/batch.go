// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/log"
)

// EnvReserveBatch enables per-process token reservation when set to an integer > 1.
// Unset, 0 or 1 keeps the exact one-EVAL-per-request behavior.
const EnvReserveBatch = "TUTTI_RATELIMIT_RESERVE_BATCH"

// reserveLeaseTTL bounds how long locally reserved tokens may be spent after they were
// taken from the shared bucket; unused tokens expire (they stay consumed, never refunded).
// This keeps the burst error of reservation below reserveBatch tokens per process and 1s.
const reserveLeaseTTL = time.Second

// StorageReq is one storage side of a multi-storage rate limit request.
type StorageReq struct {
	Storage string
	N       int
	Opts    []Opt
}

// MultiRPM acquires limits of several storages atomically in one Redis round trip.
// All-or-nothing: if any storage is over its limit nothing is consumed on any of them.
type MultiRPM interface {
	StorReqMulti(ctx context.Context, reqs ...StorageReq) error
}

var _ MultiRPM = (*Svc)(nil)

type lease struct {
	tokens  int
	expires time.Time
}

type reservations struct {
	mu    sync.Mutex
	now   func() time.Time
	batch int
	pools map[string]*lease
}

// clock must be called with r.mu held.
func (r *reservations) clock() time.Time {
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

func reserveBatchFromEnv() int {
	n, _ := strconv.Atoi(os.Getenv(EnvReserveBatch))
	if n < 1 {
		return 0
	}
	return n
}

// SetReserveBatch sets the per-process token reservation size (<=1 disables reservation).
func (s *Svc) SetReserveBatch(n int) {
	s.res.mu.Lock()
	defer s.res.mu.Unlock()
	if n < 1 {
		n = 0
	}
	s.res.batch = n
	s.res.pools = map[string]*lease{}
}

// SetClock overrides the clock used for reservation lease expiry (tests).
func (s *Svc) SetClock(now func() time.Time) {
	s.res.mu.Lock()
	defer s.res.mu.Unlock()
	s.res.now = now
}

type need struct {
	storage string
	limit   redis_rate.Limit
	cost    int
}

func (s *Svc) resolve(ctx context.Context, reqs []StorageReq) []need {
	var out []need
	for _, r := range reqs {
		conf, ok := s.setting(r.Storage)
		if !ok || !conf.Enabled {
			continue
		}
		n := filterRateLimitMethods(conf, r.N, r.Opts...)
		if n <= 0 {
			continue
		}
		merged := false
		for i := range out {
			if out[i].storage == r.Storage {
				out[i].cost += n
				merged = true
			}
		}
		if !merged {
			out = append(out, need{storage: r.Storage, limit: redis_rate.PerMinute(conf.RPM), cost: n})
		}
	}
	return out
}

// StorReqMulti acquires the rate limits of all given storages in a single Redis EVAL
// (or none, when served from locally reserved tokens).
func (s *Svc) StorReqMulti(ctx context.Context, reqs ...StorageReq) error {
	needs := s.resolve(ctx, reqs)
	if len(needs) == 0 {
		return nil
	}
	return s.acquire(ctx, needs)
}

// acquire consumes tokens for needs: first from local reservations, then from Redis.
func (s *Svc) acquire(ctx context.Context, needs []need) error {
	r := &s.res
	r.mu.Lock()
	batch := r.batch
	now := r.clock()
	var missing []need
	var taken []need
	for _, nd := range needs {
		if l := r.pools[nd.storage]; batch > 1 && l != nil && now.Before(l.expires) && l.tokens >= nd.cost {
			l.tokens -= nd.cost
			taken = append(taken, nd)
			continue
		}
		missing = append(missing, nd)
	}
	r.mu.Unlock()
	if len(missing) == 0 {
		return nil
	}
	refund := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, nd := range taken {
			if l := r.pools[nd.storage]; l != nil {
				l.tokens += nd.cost
			}
		}
	}

	// ask for a whole batch of tokens; fall back to the exact cost near the limit
	costs := make([]int, len(missing))
	reserved := false
	for i, nd := range missing {
		costs[i] = nd.cost
		if batch > 1 {
			if c := min(batch, nd.limit.Rate); c > nd.cost {
				costs[i] = c
				reserved = true
			}
		}
	}
	allowed, retry, err := s.request(ctx, missing, costs)
	if err == nil && !allowed && reserved {
		for i, nd := range missing {
			costs[i] = nd.cost
		}
		reserved = false
		allowed, retry, err = s.request(ctx, missing, costs)
	}
	if err != nil {
		// rate limiter failure should not affect business logic.
		for _, nd := range missing {
			zerolog.Ctx(ctx).Err(err).Str(log.Storage, nd.storage).Msg("rate limit error")
		}
		return nil
	}
	if !allowed {
		refund()
		return &dom.ErrRateLimitExceeded{RetryIn: retry}
	}
	if reserved {
		r.mu.Lock()
		if r.pools == nil {
			r.pools = map[string]*lease{}
		}
		for i, nd := range missing {
			extra := costs[i] - nd.cost
			if extra <= 0 {
				continue
			}
			l := r.pools[nd.storage]
			if l == nil || !now.Before(l.expires) {
				l = &lease{}
				r.pools[nd.storage] = l
			}
			l.tokens += extra
			l.expires = r.clock().Add(reserveLeaseTTL)
		}
		r.mu.Unlock()
	}
	return nil
}

// request takes costs[i] tokens from each need atomically. One need uses the stock limiter
// (identical to the pre-existing behavior); several needs use one multi-key EVAL.
func (s *Svc) request(ctx context.Context, needs []need, costs []int) (bool, time.Duration, error) {
	if len(needs) == 1 {
		res, err := s.limiter.AllowN(ctx, "lim:"+needs[0].storage, needs[0].limit, costs[0])
		if err != nil {
			return false, 0, err
		}
		return res.Allowed > 0, res.RetryAfter, nil
	}
	keys := make([]string, len(needs))
	args := make([]interface{}, 0, 4*len(needs))
	for i, nd := range needs {
		keys[i] = "rate:lim:" + nd.storage
		args = append(args, nd.limit.Burst, nd.limit.Rate, nd.limit.Period.Seconds(), costs[i])
	}
	v, err := allowMulti.Run(ctx, s.rc, keys, args...).Result()
	if err != nil {
		return false, 0, err
	}
	out := v.([]interface{})
	if out[0].(int64) == 1 {
		return true, 0, nil
	}
	retry, _ := strconv.ParseFloat(out[1].(string), 64)
	return false, time.Duration(retry * float64(time.Second)), nil
}

// allowMulti is the redis_rate GCRA script generalized to several keys: it only mutates
// state when every key admits its cost, so a denied side never burns the other side's tokens.
// State format is identical to redis_rate (TAT in the same keys), so it interoperates with
// StorReq/StorReqN used by other processes (proxy, older workers).
var allowMulti = redis.NewScript(`
redis.replicate_commands()
local jan_1_2017 = 1483228800
local t = redis.call("TIME")
local now = (t[1] - jan_1_2017) + (t[2] / 1000000)
local n = #KEYS
local new_tats = {}
local denied = false
local retry = 0
for i = 1, n do
  local base = (i - 1) * 4
  local burst = tonumber(ARGV[base + 1])
  local rate = tonumber(ARGV[base + 2])
  local period = tonumber(ARGV[base + 3])
  local cost = tonumber(ARGV[base + 4])
  local emission_interval = period / rate
  local tat = redis.call("GET", KEYS[i])
  if not tat then tat = now else tat = tonumber(tat) end
  tat = math.max(tat, now)
  local new_tat = tat + emission_interval * cost
  local allow_at = new_tat - emission_interval * burst
  local diff = now - allow_at
  if diff / emission_interval < 0 then
    denied = true
    retry = math.max(retry, -diff)
  end
  new_tats[i] = new_tat
end
if denied then
  return {0, tostring(retry)}
end
for i = 1, n do
  local reset_after = new_tats[i] - now
  if reset_after > 0 then
    redis.call("SET", KEYS[i], new_tats[i], "EX", math.ceil(reset_after))
  end
end
return {1, "-1"}
`)
