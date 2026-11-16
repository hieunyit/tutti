package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// ActivityMaxInFlight bounds the in-flight set of one replication (the newest entries are kept).
	ActivityMaxInFlight = 500
	// ActivityMaxEvents is the approximate length of the event stream of one replication.
	ActivityMaxEvents = 1000
	// ActivityInFlightMaxAge is the safety expiry of an in-flight entry: entries left behind by a crashed
	// worker disappear after this time even if nothing else touches the replication.
	ActivityInFlightMaxAge = 2 * time.Hour
	// ActivityMaxErrorLen truncates stored error messages.
	ActivityMaxErrorLen = 300
	// ActivityMaxReadLimit is the maximum number of events returned by one read.
	ActivityMaxReadLimit = 500

	activityKeyPrefix = "p:repl_act:"
	activityDataTTL   = 24 * time.Hour
)

// Activity event levels and types.
const (
	ActivityInfo  = "info"
	ActivityWarn  = "warn"
	ActivityError = "error"

	ActivityStarted = "started"
	ActivityDone    = "done"
	ActivityFailed  = "failed"
	ActivityRetry   = "retry"
	ActivitySkipped = "skipped"

	ActivityFlowInit = "init"
	ActivityFlowLive = "live"
	// ActivityFlowRescan marks copies of the periodic source rescan.
	ActivityFlowRescan = "rescan"
)

// ActivityInFlight is one object copy that is running right now.
type ActivityInFlight struct {
	Key       string `json:"key"`
	VersionID string `json:"versionId,omitempty"`
	Size      int64  `json:"size"`
	StartedAt int64  `json:"startedAt"` // unix milliseconds
	Flow      string `json:"flow"`
	Attempt   int    `json:"attempt"`
	// BytesDone is filled at read time from the initial replication progress (never more than Size).
	BytesDone int64 `json:"-"`
}

// ActivityEvent is one entry of the replication activity log.
type ActivityEvent struct {
	Cursor         string `json:"cursor"`
	Time           int64  `json:"ts"` // unix milliseconds
	Level          string `json:"level"`
	Type           string `json:"type"`
	Key            string `json:"key"`
	VersionID      string `json:"versionId,omitempty"`
	Size           int64  `json:"size"`
	DurationMs     int64  `json:"durationMs"`
	BytesPerSecond int64  `json:"bytesPerSecond"`
	Attempt        int    `json:"attempt"`
	Flow           string `json:"flow"`
	Error          string `json:"error,omitempty"`
}

// ActivityStore keeps the live activity of a replication (running copies and a capped event log) in Redis.
// Like InitBytesStore it is runtime data that lives in the queue Redis, never in the control plane store.
//
//	p:repl_act:{<id>:<gen>}:fly   hash  field "<key>\x00<versionId>" -> in-flight JSON
//	p:repl_act:{<id>:<gen>}:flyz  zset  same field scored by start time (unix ms), used for the cap and expiry
//	p:repl_act:{<id>:<gen>}:ev    stream, MAXLEN ~ ActivityMaxEvents; the stream entry ID is the cursor
type ActivityStore struct {
	client redis.Cmdable
}

func NewActivityStore(client redis.Cmdable) *ActivityStore {
	return &ActivityStore{client: client}
}

func activityKeys(replID string, generation int64) (hash, zset, stream string) {
	base := fmt.Sprintf("%s{%s:%d}", activityKeyPrefix, replID, generation)
	return base + ":fly", base + ":flyz", base + ":ev"
}

func activityField(key, versionID string) string { return key + "\x00" + versionID }

// KEYS: hash, zset, stream. ARGV: field ("" = leave in-flight untouched), entry JSON ("" = remove field,
// otherwise add), now ms, cap, stale-before ms, ttl seconds, stream maxlen, [event field/value pairs...].
var activityScript = redis.NewScript(`
local field = ARGV[1]
if field ~= '' then
  if ARGV[2] ~= '' then
    redis.call('ZADD', KEYS[2], ARGV[3], field)
    redis.call('HSET', KEYS[1], field, ARGV[2])
    local stale = redis.call('ZRANGEBYSCORE', KEYS[2], '-inf', ARGV[5])
    for _, f in ipairs(stale) do redis.call('ZREM', KEYS[2], f); redis.call('HDEL', KEYS[1], f) end
    local over = redis.call('ZCARD', KEYS[2]) - tonumber(ARGV[4])
    if over > 0 then
      local old = redis.call('ZRANGE', KEYS[2], 0, over - 1)
      for _, f in ipairs(old) do redis.call('ZREM', KEYS[2], f); redis.call('HDEL', KEYS[1], f) end
    end
    redis.call('EXPIRE', KEYS[1], ARGV[6])
    redis.call('EXPIRE', KEYS[2], ARGV[6])
  else
    redis.call('ZREM', KEYS[2], field)
    redis.call('HDEL', KEYS[1], field)
  end
end
if #ARGV >= 8 then
  redis.call('XADD', KEYS[3], 'MAXLEN', '~', ARGV[7], '*', unpack(ARGV, 8))
  redis.call('EXPIRE', KEYS[3], ARGV[6])
end
return 1
`)

func (s *ActivityStore) run(ctx context.Context, replID string, generation int64, field, entry string, now time.Time, ev *ActivityEvent) error {
	h, z, st := activityKeys(replID, generation)
	args := []any{
		field, entry, now.UnixMilli(), ActivityMaxInFlight,
		now.Add(-ActivityInFlightMaxAge).UnixMilli(), int64(activityDataTTL / time.Second), ActivityMaxEvents,
	}
	if ev != nil {
		args = append(args,
			"ts", now.UnixMilli(), "level", ev.Level, "type", ev.Type, "key", ev.Key, "ver", ev.VersionID,
			"size", ev.Size, "dur", ev.DurationMs, "bps", ev.BytesPerSecond, "att", ev.Attempt, "flow", ev.Flow,
			"err", TruncateActivityError(ev.Error))
	}
	return activityScript.Run(ctx, s.client, []string{h, z, st}, args...).Err()
}

// Begin registers a running copy and, if ev is not nil, appends the "started" event.
func (s *ActivityStore) Begin(ctx context.Context, replID string, generation int64, e ActivityInFlight, ev *ActivityEvent) error {
	e.StartedAt = time.Now().UnixMilli()
	b, _ := json.Marshal(e)
	return s.run(ctx, replID, generation, activityField(e.Key, e.VersionID), string(b), time.Now(), ev)
}

// End removes the in-flight entry of key/versionID (when untrack is set) and appends ev (when not nil).
func (s *ActivityStore) End(ctx context.Context, replID string, generation int64, key, versionID string, untrack bool, ev *ActivityEvent) error {
	field := ""
	if untrack {
		field = activityField(key, versionID)
	}
	return s.run(ctx, replID, generation, field, "", time.Now(), ev)
}

// TruncateActivityError shortens an error message to ActivityMaxErrorLen runes-safe bytes.
func TruncateActivityError(msg string) string {
	if len(msg) <= ActivityMaxErrorLen {
		return msg
	}
	cut := ActivityMaxErrorLen
	for cut > 0 && msg[cut]&0xC0 == 0x80 { // do not split a UTF-8 sequence
		cut--
	}
	return msg[:cut] + "..."
}

// ActivitySnapshot is the result of ActivityStore.Get.
type ActivitySnapshot struct {
	InFlight   []ActivityInFlight
	Events     []ActivityEvent
	NextCursor string
}

// nextStreamID returns the smallest stream ID that is greater than id.
func nextStreamID(id string) string {
	ms, seq, ok := strings.Cut(id, "-")
	if !ok {
		return id + "-1"
	}
	n, err := strconv.ParseUint(seq, 10, 64)
	if err != nil {
		return id
	}
	return ms + "-" + strconv.FormatUint(n+1, 10)
}

// Get returns the in-flight copies (oldest first) and up to limit events. With an empty after the newest events
// are returned, otherwise only events newer than the after cursor, oldest first. NextCursor is the cursor
// to pass next time (after itself when nothing new arrived).
func (s *ActivityStore) Get(ctx context.Context, replID string, generation int64, after string, limit int, now time.Time) (ActivitySnapshot, error) {
	if limit <= 0 || limit > ActivityMaxReadLimit {
		limit = ActivityMaxReadLimit
	}
	h, _, st := activityKeys(replID, generation)
	pipe := s.client.Pipeline()
	flyCmd := pipe.HGetAll(ctx, h)
	_, _, progKey := initBytesKeys(replID, generation)
	progCmd := pipe.HGetAll(ctx, progKey)
	var evCmd *redis.XMessageSliceCmd
	if after == "" {
		evCmd = pipe.XRevRangeN(ctx, st, "+", "-", int64(limit))
	} else {
		evCmd = pipe.XRangeN(ctx, st, nextStreamID(after), "+", int64(limit))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return ActivitySnapshot{}, err
	}
	res := ActivitySnapshot{NextCursor: after}
	stale := now.Add(-ActivityInFlightMaxAge).UnixMilli()
	progress := progCmd.Val()
	for field, raw := range flyCmd.Val() {
		var e ActivityInFlight
		if json.Unmarshal([]byte(raw), &e) != nil || e.StartedAt < stale {
			continue
		}
		if v, ok := progress[field]; ok {
			e.BytesDone, _ = strconv.ParseInt(v, 10, 64)
			if e.BytesDone < 0 || (e.Size > 0 && e.BytesDone > e.Size) {
				e.BytesDone = max(0, min(e.BytesDone, e.Size))
			}
		}
		res.InFlight = append(res.InFlight, e)
	}
	sort.Slice(res.InFlight, func(i, j int) bool { return res.InFlight[i].StartedAt < res.InFlight[j].StartedAt })
	msgs := evCmd.Val()
	if after == "" { // XREVRANGE returns newest first
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
	}
	for _, m := range msgs {
		res.Events = append(res.Events, activityEventFromMessage(m))
	}
	if n := len(res.Events); n > 0 {
		res.NextCursor = res.Events[n-1].Cursor
	}
	return res, nil
}

func activityEventFromMessage(m redis.XMessage) ActivityEvent {
	str := func(k string) string { v, _ := m.Values[k].(string); return v }
	num := func(k string) int64 { n, _ := strconv.ParseInt(str(k), 10, 64); return n }
	return ActivityEvent{
		Cursor: m.ID, Time: num("ts"), Level: str("level"), Type: str("type"), Key: str("key"),
		VersionID: str("ver"), Size: num("size"), DurationMs: num("dur"), BytesPerSecond: num("bps"),
		Attempt: int(num("att")), Flow: str("flow"), Error: str("err"),
	}
}
