// Package failover holds the failover policies and failover event records stored in Redis (config DB).
//
// A failover policy is defined per SOURCE storage and lists the only storages (in order of preference) the
// proxy failover controller is allowed to promote when the source is down. Events are written by the
// controller after each performed failover.
//
// Redis layout:
//
//	tutti:failover-policies            hash: field = source storage, value = JSON Policy
//	tutti:failover:user:<user>         JSON Event (user-level scope)
//	tutti:failover:bucket:<user>:<b>   JSON Event (bucket-level scope)
package failover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/dom"
)

const (
	policiesKey = "tutti:failover-policies"
	// EventPrefix is the Redis key prefix of failover events.
	EventPrefix = "tutti:failover:"
)

// Policy is the failover policy of one source storage.
type Policy struct {
	// Source is the storage that may fail.
	Source  string `json:"source"`
	Enabled bool   `json:"enabled"`
	// Targets are the only storages that may be promoted, in order of preference.
	Targets []string `json:"targets"`
	// After is how long the source breaker must be continuously open. 0 means the proxy default (failover.after).
	After time.Duration `json:"after"`
	// MaxEventLag is the max number of unreplicated events allowed on the replication to the target.
	// nil means the proxy default (0).
	MaxEventLag *int `json:"maxEventLag,omitempty"`
}

// Validate checks the policy. known reports whether a storage is configured; nil skips the existence check.
func (p Policy) Validate(known func(storage string) bool) error {
	if p.Source == "" {
		return fmt.Errorf("%w: failover policy: source is required", dom.ErrInvalidArg)
	}
	if known != nil && !known(p.Source) {
		return fmt.Errorf("%w: failover policy: unknown source storage %q", dom.ErrInvalidArg, p.Source)
	}
	if p.After < 0 {
		return fmt.Errorf("%w: failover policy: after must not be negative", dom.ErrInvalidArg)
	}
	if p.MaxEventLag != nil && *p.MaxEventLag < 0 {
		return fmt.Errorf("%w: failover policy: maxEventLag must not be negative", dom.ErrInvalidArg)
	}
	seen := map[string]struct{}{}
	for _, t := range p.Targets {
		switch {
		case t == "":
			return fmt.Errorf("%w: failover policy: empty target", dom.ErrInvalidArg)
		case t == p.Source:
			return fmt.Errorf("%w: failover policy: target %q equals source", dom.ErrInvalidArg, t)
		case known != nil && !known(t):
			return fmt.Errorf("%w: failover policy: unknown target storage %q", dom.ErrInvalidArg, t)
		}
		if _, dup := seen[t]; dup {
			return fmt.Errorf("%w: failover policy: duplicate target %q", dom.ErrInvalidArg, t)
		}
		seen[t] = struct{}{}
	}
	if p.Enabled && len(p.Targets) == 0 {
		return fmt.Errorf("%w: failover policy: enabled policy needs at least one target", dom.ErrInvalidArg)
	}
	return nil
}

// Event describes one performed failover.
type Event struct {
	User   string    `json:"user"`
	Bucket string    `json:"bucket,omitempty"` // empty for user-level failover
	From   string    `json:"from"`
	To     string    `json:"to"`
	Time   time.Time `json:"time"`
	// EventLag is the number of unreplicated events on the replication at failover time.
	EventLag int `json:"eventLag"`
}

// Key returns the Redis key of the event.
func (e Event) Key() string { return eventKey(e.User, e.Bucket) }

func eventKey(user, bucket string) string {
	if bucket == "" {
		return EventPrefix + "user:" + user
	}
	return EventPrefix + "bucket:" + user + ":" + bucket
}

// Store keeps failover policies and events in Redis.
type Store struct {
	client redis.Cmdable
	// known reports whether a storage is configured (used to validate policies). nil disables the check.
	known func(storage string) bool
}

// NewStore creates the store. known is used by SetFailoverPolicy to validate source and targets.
func NewStore(client redis.Cmdable, known func(storage string) bool) *Store {
	return &Store{client: client, known: known}
}

// SetFailoverPolicy validates and creates or replaces the policy of p.Source.
func (s *Store) SetFailoverPolicy(ctx context.Context, p Policy) error {
	if err := p.Validate(s.known); err != nil {
		return err
	}
	if p.Targets == nil {
		p.Targets = []string{}
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.client.HSet(ctx, policiesKey, p.Source, b).Err()
}

// GetFailoverPolicy returns the policy of the source storage or dom.ErrNotFound.
func (s *Store) GetFailoverPolicy(ctx context.Context, source string) (Policy, error) {
	val, err := s.client.HGet(ctx, policiesKey, source).Result()
	if errors.Is(err, redis.Nil) {
		return Policy{}, fmt.Errorf("%w: no failover policy for storage %q", dom.ErrNotFound, source)
	}
	if err != nil {
		return Policy{}, err
	}
	var p Policy
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return Policy{}, fmt.Errorf("failover policy %q: %w", source, err)
	}
	return p, nil
}

// ListFailoverPolicies returns all policies sorted by source.
func (s *Store) ListFailoverPolicies(ctx context.Context) ([]Policy, error) {
	m, err := s.client.HGetAll(ctx, policiesKey).Result()
	if err != nil {
		return nil, err
	}
	res := make([]Policy, 0, len(m))
	for src, val := range m {
		var p Policy
		if err := json.Unmarshal([]byte(val), &p); err != nil {
			return nil, fmt.Errorf("failover policy %q: %w", src, err)
		}
		res = append(res, p)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Source < res[j].Source })
	return res, nil
}

// DeleteFailoverPolicy removes the policy; deleting a missing policy is not an error.
func (s *Store) DeleteFailoverPolicy(ctx context.Context, source string) error {
	return s.client.HDel(ctx, policiesKey, source).Err()
}

// SaveFailoverEvent stores an event.
func (s *Store) SaveFailoverEvent(ctx context.Context, e Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, e.Key(), b, 0).Err()
}

// ListFailoverEvents returns all failover events sorted by key.
func (s *Store) ListFailoverEvents(ctx context.Context) ([]Event, error) {
	var res []Event
	iter := s.client.Scan(ctx, 0, EventPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		val, err := s.client.Get(ctx, iter.Val()).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var e Event
		if err := json.Unmarshal([]byte(val), &e); err != nil {
			return nil, fmt.Errorf("failover event %s: %w", iter.Val(), err)
		}
		res = append(res, e)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Key() < res[j].Key() })
	return res, nil
}

// DeleteFailoverEvent removes the event of a scope (bucket empty for user-level), e.g. after manual failback.
func (s *Store) DeleteFailoverEvent(ctx context.Context, user, bucket string) error {
	return s.client.Del(ctx, eventKey(user, bucket)).Err()
}
