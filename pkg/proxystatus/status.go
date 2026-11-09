// Package proxystatus publishes the live status of each proxy instance to Redis (config DB) and reads it back.
//
// Every proxy writes a JSON Status document under tutti:proxy-status:<app_id> every Interval with a TTL of
// 3*Interval, so a stopped or crashed proxy disappears from the listing after at most 3 intervals.
package proxystatus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	// KeyPrefix is the Redis key prefix of status documents.
	KeyPrefix = "tutti:proxy-status:"
	// DefaultInterval is the publish period when none is given.
	DefaultInterval = 10 * time.Second
)

// StorageHealth is the circuit breaker state of one storage.
type StorageHealth struct {
	Name                string    `json:"name"`
	Available           bool      `json:"available"`
	ConsecutiveFailures int       `json:"consecutiveFailures"`
	LastError           string    `json:"lastError,omitempty"`
	LastChange          time.Time `json:"lastChange"`
}

// SyncSettings are the effective synchronous replication settings.
type SyncSettings struct {
	Enabled         bool          `json:"enabled"`
	WriteQuorum     int           `json:"writeQuorum"`
	Timeout         time.Duration `json:"timeout"`
	OnQuorumFailure string        `json:"onQuorumFailure"`
}

// ReplicationSettings are the effective replication settings.
type ReplicationSettings struct {
	OnEventError     string        `json:"onEventError"`
	EventRetryBudget time.Duration `json:"eventRetryBudget"`
	Sync             SyncSettings  `json:"sync"`
}

// HealthSettings are the effective circuit breaker settings.
type HealthSettings struct {
	FailureThreshold int           `json:"failureThreshold"`
	OpenDuration     time.Duration `json:"openDuration"`
}

// FailoverSettings are the effective failover controller defaults.
type FailoverSettings struct {
	Enabled       bool          `json:"enabled"`
	CheckInterval time.Duration `json:"checkInterval"`
	After         time.Duration `json:"after"`
	MaxEventLag   int           `json:"maxEventLag"`
}

// Settings are the effective proxy settings.
type Settings struct {
	Replication              ReplicationSettings `json:"replication"`
	Health                   HealthSettings      `json:"health"`
	Failover                 FailoverSettings    `json:"failover"`
	RequireSignedContentType bool                `json:"requireSignedContentType"`
}

// Account is a proxy account: a user and one of its access key ids.
type Account struct {
	User        string `json:"user"`
	AccessKeyID string `json:"accessKeyId"`
	// Source is "custom" or "storage:<name>".
	Source string `json:"source"`
}

// Status is the document published by one proxy instance.
type Status struct {
	AppID     string          `json:"appId"`
	Address   string          `json:"address"`
	StartedAt time.Time       `json:"startedAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Storages  []StorageHealth `json:"storages"`
	// Accounts are the accounts able to authenticate against this proxy (no secrets).
	Accounts []Account `json:"accounts,omitempty"`
	Settings Settings  `json:"settings"`
}

// Key returns the Redis key of the app's status document.
func Key(appID string) string { return KeyPrefix + appID }

// TTL returns the document TTL for a publish interval.
func TTL(interval time.Duration) time.Duration { return 3 * interval }

// Publisher periodically writes the status of one proxy.
type Publisher struct {
	client   redis.Cmdable
	collect  func() Status
	logger   zerolog.Logger
	appID    string
	interval time.Duration
}

// NewPublisher creates a publisher. collect returns the current document (AppID is forced to appID and UpdatedAt to now).
func NewPublisher(client redis.Cmdable, appID string, interval time.Duration, collect func() Status, logger zerolog.Logger) *Publisher {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Publisher{client: client, appID: appID, interval: interval, collect: collect, logger: logger}
}

// PublishOnce writes the document with TTL 3*interval.
func (p *Publisher) PublishOnce(ctx context.Context) error {
	st := p.collect()
	st.AppID = p.appID
	st.UpdatedAt = time.Now().UTC()
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return p.client.Set(ctx, Key(p.appID), b, TTL(p.interval)).Err()
}

// Run publishes immediately and then every interval until ctx is done.
func (p *Publisher) Run(ctx context.Context) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		if err := p.PublishOnce(ctx); err != nil && ctx.Err() == nil {
			p.logger.Warn().Err(err).Msg("unable to publish proxy status")
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// List returns all live (not expired) proxy status documents sorted by app id.
func List(ctx context.Context, client redis.Cmdable) ([]Status, error) {
	var res []Status
	iter := client.Scan(ctx, 0, KeyPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		val, err := client.Get(ctx, iter.Val()).Result()
		if errors.Is(err, redis.Nil) {
			continue // expired between SCAN and GET
		}
		if err != nil {
			return nil, err
		}
		var st Status
		if err := json.Unmarshal([]byte(val), &st); err != nil {
			return nil, fmt.Errorf("proxy status %s: %w", iter.Val(), err)
		}
		res = append(res, st)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	sort.Slice(res, func(i, j int) bool { return res[i].AppID < res[j].AppID })
	return res, nil
}
