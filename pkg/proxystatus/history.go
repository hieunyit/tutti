package proxystatus

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/trafficstats"
)

const (
	// HistoryKeyPrefix is the Redis key prefix of the per-proxy history lists (newest sample first).
	HistoryKeyPrefix = "tutti:proxy-stats-history:"
	// HistoryInterval is the period between history samples.
	HistoryInterval = 10 * time.Second
	// HistoryMaxEntries caps each list: 6 hours of HistoryInterval samples.
	HistoryMaxEntries = int(6 * time.Hour / HistoryInterval)
	// HistoryTTL expires the list of a proxy that stopped recording.
	HistoryTTL = 6*time.Hour + time.Minute
)

const historyFirstDelay = 3 * time.Second

// HistoryKey returns the Redis key of the app's history list.
func HistoryKey(appID string) string { return HistoryKeyPrefix + appID }

// Sample is one history entry: the rates of one proxy at Time (unix seconds).
type Sample struct {
	Time  int64              `json:"t"`
	Rates trafficstats.Rates `json:"r"`
}

// AppendSample pushes a sample to the app's history list, trims it to HistoryMaxEntries and refreshes its TTL.
func AppendSample(ctx context.Context, client redis.Cmdable, appID string, s Sample) error {
	s.Rates = s.Rates.Round()
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	pipe := client.Pipeline()
	pipe.LPush(ctx, HistoryKey(appID), b)
	pipe.LTrim(ctx, HistoryKey(appID), 0, int64(HistoryMaxEntries-1))
	pipe.Expire(ctx, HistoryKey(appID), HistoryTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// RunHistory appends current() to the history every HistoryInterval until ctx is done. The first sample is taken
// after historyFirstDelay so a fresh proxy shows data quickly.
func RunHistory(ctx context.Context, client redis.Cmdable, appID string, current func() trafficstats.Rates, logger zerolog.Logger) {
	t := time.NewTimer(historyFirstDelay)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			t.Reset(HistoryInterval)
		}
		if err := AppendSample(ctx, client, appID, Sample{Time: time.Now().Unix(), Rates: current()}); err != nil && ctx.Err() == nil {
			logger.Warn().Err(err).Msg("unable to append proxy stats history")
		}
	}
}

// ReadHistory returns the samples of all proxies (keyed by app id, oldest first) with Time >= since.
func ReadHistory(ctx context.Context, client redis.Cmdable, since time.Time) (map[string][]Sample, error) {
	res := map[string][]Sample{}
	// newest first: at most (now-since)/HistoryInterval samples are relevant
	n := int64(time.Since(since)/HistoryInterval) + 2
	if n > int64(HistoryMaxEntries) {
		n = int64(HistoryMaxEntries)
	}
	iter := client.Scan(ctx, 0, HistoryKeyPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		vals, err := client.LRange(ctx, key, 0, n-1).Result()
		if err != nil {
			return nil, err
		}
		var list []Sample
		for _, v := range vals {
			var s Sample
			if err := json.Unmarshal([]byte(v), &s); err != nil {
				continue // skip a corrupt entry
			}
			if s.Time >= since.Unix() {
				list = append(list, s)
			}
		}
		if len(list) == 0 {
			continue
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Time < list[j].Time })
		res[strings.TrimPrefix(key, HistoryKeyPrefix)] = list
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("proxy stats history: %w", err)
	}
	return res, nil
}

// Point is one aggregated history bucket: the rates summed over all proxies.
type Point struct {
	Time  time.Time
	Rates trafficstats.Rates
}

// BucketForRange returns the bucket size for a history range: 10s up to 15m, 30s for 1h, 2m for 6h.
func BucketForRange(d time.Duration) time.Duration {
	switch {
	case d <= 15*time.Minute:
		return HistoryInterval
	case d <= time.Hour:
		return 30 * time.Second
	}
	return 2 * time.Minute
}

// Aggregate buckets the samples of every proxy and sums the buckets across proxies. Within a bucket the samples of one
// proxy are averaged (latency request-weighted, p95 max); across proxies see trafficstats.SumRates. Only buckets with
// at least one sample are returned, ordered by time.
func Aggregate(perProxy map[string][]Sample, bucket time.Duration) []Point {
	b := int64(bucket / time.Second)
	if b <= 0 {
		b = int64(HistoryInterval / time.Second)
	}
	byBucket := map[int64]map[string][]trafficstats.Rates{}
	for app, list := range perProxy {
		for _, s := range list {
			k := s.Time - s.Time%b
			m := byBucket[k]
			if m == nil {
				m = map[string][]trafficstats.Rates{}
				byBucket[k] = m
			}
			m[app] = append(m[app], s.Rates)
		}
	}
	keys := make([]int64, 0, len(byBucket))
	for k := range byBucket {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	res := make([]Point, 0, len(keys))
	for _, k := range keys {
		var per []trafficstats.Rates
		for _, rs := range byBucket[k] {
			per = append(per, trafficstats.MeanRates(rs))
		}
		res = append(res, Point{Time: time.Unix(k, 0).UTC(), Rates: trafficstats.SumRates(per)})
	}
	return res
}
