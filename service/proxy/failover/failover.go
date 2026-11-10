// Package failover implements the optional automatic failover controller of the proxy.
//
// When the main storage of a user (or bucket) has been unavailable (circuit breaker open) for a configured
// time, the controller routes the scope to the destination of its replication and pauses that replication.
// Failback is never automatic.
package failover

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	pf "github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/health"
)

const (
	lockPrefix = "tutti:failover-lock:"
	// lockTTL bounds how long a crashed holder blocks others. Must exceed the duration of one failover run.
	lockTTL = 30 * time.Second
)

// Policies provides the operator-defined failover policies (see pkg/failover).
type Policies interface {
	ListFailoverPolicies(ctx context.Context) ([]pf.Policy, error)
	SaveFailoverEvent(ctx context.Context, e pf.Event) error
}

// Policy is the subset of the policy service used by the controller.
type Policy interface {
	GetUserRoutings(ctx context.Context) (map[string]string, error)
	GetBucketRoutings(ctx context.Context, user string) (map[string]string, error)
	ListUserReplicationsInfo(ctx context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error)
	ListBucketReplicationsInfo(ctx context.Context, user string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error)
	FailoverReplication(ctx context.Context, id entity.UniversalReplicationID) error
}

// Health provides storage breaker states.
type Health interface {
	Snapshot() []health.Status
}

// Locker is a distributed lock. Obtain returns ok=false if somebody else holds the lock.
type Locker interface {
	Obtain(ctx context.Context, key string, ttl time.Duration) (release func(), ok bool, err error)
}

// Config of the controller.
type Config struct {
	// DefaultAfter and DefaultMaxEventLag apply when a policy omits them.
	DefaultAfter       time.Duration
	DefaultMaxEventLag int
	CheckInterval      time.Duration
	// MainStorage is the implicit routing target of users without explicit routing.
	MainStorage string
}

// Controller evaluates failover conditions and performs failovers.
type Controller struct {
	cfg      Config
	policy   Policy
	health   Health
	locker   Locker
	policies Policies
	// Users returns users to inspect for bucket-level policies (in addition to users of user-level policies/routings).
	users       func() []string
	isProtected func(ctx context.Context, user, bucket string) bool
	now         func() time.Time
	log   zerolog.Logger
	onDo  func(from, to string)

	mu       sync.Mutex
	enabled  bool
	reconfCh chan struct{}
	warned   map[string]struct{} // scope keys already reported in the current outage
}

type Option func(*Controller)

func WithClock(f func() time.Time) Option           { return func(c *Controller) { c.now = f } }
func WithUsers(f func() []string) Option            { return func(c *Controller) { c.users = f } }
func WithIsProtected(f func(ctx context.Context, user, bucket string) bool) Option {
	return func(c *Controller) { c.isProtected = f }
}
func WithOnFailover(f func(from, to string)) Option { return func(c *Controller) { c.onDo = f } }
func WithEnabled(enabled bool) Option               { return func(c *Controller) { c.enabled = enabled } }

func New(cfg Config, p Policy, h Health, l Locker, pol Policies, log zerolog.Logger, opts ...Option) *Controller {
	c := &Controller{
		cfg:      cfg,
		enabled:  true,
		reconfCh: make(chan struct{}, 1),
		policy:   p,
		health:   h,
		locker:   l,
		policies: pol,
		log:      log,
		now:      time.Now,
		warned:   map[string]struct{}{},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// SetConfig updates the failover controller configuration and enabled flag at runtime.
func (c *Controller) SetConfig(cfg Config, enabled bool) {
	c.mu.Lock()
	if cfg.MainStorage == "" {
		cfg.MainStorage = c.cfg.MainStorage
	}
	intervalChanged := c.cfg.CheckInterval != cfg.CheckInterval
	c.cfg = cfg
	c.enabled = enabled
	c.mu.Unlock()
	if intervalChanged {
		select {
		case c.reconfCh <- struct{}{}:
		default:
		}
	}
}

func (c *Controller) checkInterval() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.CheckInterval <= 0 {
		return 10 * time.Second
	}
	return c.cfg.CheckInterval
}

// Run evaluates the conditions every CheckInterval until ctx is done.
func (c *Controller) Run(ctx context.Context) {
	t := time.NewTicker(c.checkInterval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.reconfCh:
			t.Reset(c.checkInterval())
		case <-t.C:
			if err := c.RunOnce(ctx); err != nil && ctx.Err() == nil {
				c.log.Error().Err(err).Msg("failover: check failed")
			}
		}
	}
}

// scope is a user-level or bucket-level routing target that has a replication.
type scope struct {
	user, bucket string // bucket empty for user scope
	from         string
	candidates   []candidate
}

type candidate struct {
	id     entity.UniversalReplicationID
	to     string
	status entity.ReplicationStatusExtended
	toBkt  string // destination bucket (bucket scope)
	srcBkt string
}

func (s scope) key() string { return s.user + "/" + s.bucket }

// RunOnce performs one evaluation. Errors of individual scopes are logged; the first is returned.
func (c *Controller) RunOnce(ctx context.Context) error {
	c.mu.Lock()
	cfg, enabled := c.cfg, c.enabled
	c.mu.Unlock()
	if !enabled {
		return nil
	}
	status := map[string]health.Status{}
	for _, st := range c.health.Snapshot() {
		status[st.Name] = st
	}
	now := c.now()
	var downNames []string
	for name, st := range status {
		if st.Available {
			c.clearWarned(name)
			continue
		}
		downNames = append(downNames, name)
	}
	if len(downNames) == 0 {
		return nil
	}
	sort.Strings(downNames)
	pols, err := c.policies.ListFailoverPolicies(ctx)
	if err != nil {
		return fmt.Errorf("list failover policies: %w", err)
	}
	bySource := map[string]pf.Policy{}
	for _, p := range pols {
		bySource[p.Source] = p
	}
	var first error
	for _, failed := range downNames {
		pol, ok := bySource[failed]
		if !ok || !pol.Enabled || len(pol.Targets) == 0 {
			continue // no policy: never fail over
		}
		after := cfg.DefaultAfter
		if pol.After > 0 {
			after = pol.After
		}
		if now.Sub(status[failed].LastChange) < after {
			continue
		}
		maxLag := cfg.DefaultMaxEventLag
		if pol.MaxEventLag != nil {
			maxLag = *pol.MaxEventLag
		}
		if err := c.failoverStorage(ctx, failed, pol.Targets, maxLag, status); err != nil {
			c.log.Error().Err(err).Str("storage", failed).Msg("failover: failed")
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func (c *Controller) clearWarned(storage string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := storage + "|"
	for k := range c.warned {
		if strings.HasPrefix(k, prefix) {
			delete(c.warned, k)
		}
	}
}

func (c *Controller) failoverStorage(ctx context.Context, failed string, targets []string, maxLag int, status map[string]health.Status) error {
	scopes, err := c.scopesOf(ctx, failed)
	if err != nil {
		return err
	}
	if len(scopes) == 0 {
		return nil
	}
	release, ok, err := c.locker.Obtain(ctx, lockPrefix+failed, lockTTL)
	if err != nil {
		return fmt.Errorf("obtain failover lock: %w", err)
	}
	if !ok {
		return nil // another instance acts
	}
	defer release()
	// re-check under the lock: another instance may have failed over already
	scopes, err = c.scopesOf(ctx, failed)
	if err != nil {
		return err
	}
	var first error
	for _, s := range scopes {
		if err := c.failoverScope(ctx, failed, s, targets, maxLag, status); err != nil {
			c.log.Error().Err(err).Str("storage", failed).Str("user", s.user).Str("bucket", s.bucket).Msg("failover: scope failed")
			if first == nil {
				first = err
			}
		}
	}
	return first
}

// scopesOf returns scopes currently routed to the storage which have replications from it.
func (c *Controller) scopesOf(ctx context.Context, storage string) ([]scope, error) {
	userRoutes, err := c.policy.GetUserRoutings(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("get user routings: %w", err)
	}
	userRepls, err := c.policy.ListUserReplicationsInfo(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("list user replications: %w", err)
	}
	c.mu.Lock()
	mainStorage := c.cfg.MainStorage
	c.mu.Unlock()
	effective := func(user string) string {
		if to, ok := userRoutes[user]; ok {
			return to
		}
		return mainStorage
	}
	users := map[string]struct{}{}
	byUser := map[string]*scope{}
	for pol, st := range userRepls {
		users[pol.User] = struct{}{}
		if pol.FromStorage != storage || effective(pol.User) != storage {
			continue
		}
		if c.isProtected != nil && c.isProtected(ctx, pol.User, "") {
			continue
		}
		s := byUser[pol.User]
		if s == nil {
			s = &scope{user: pol.User, from: storage}
			byUser[pol.User] = s
		}
		s.candidates = append(s.candidates, candidate{id: entity.UniversalFromUserReplication(pol), to: pol.ToStorage, status: st})
	}
	for u := range userRoutes {
		users[u] = struct{}{}
	}
	if c.users != nil {
		for _, u := range c.users() {
			users[u] = struct{}{}
		}
	}
	var res []scope
	for _, s := range byUser {
		res = append(res, *s)
	}
	for user := range users {
		if _, isUserScope := byUser[user]; isUserScope {
			continue // bucket-level and user-level replications are mutually exclusive
		}
		bRepls, err := c.policy.ListBucketReplicationsInfo(ctx, user)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, fmt.Errorf("list bucket replications of %s: %w", user, err)
		}
		if len(bRepls) == 0 {
			continue
		}
		bRoutes, err := c.policy.GetBucketRoutings(ctx, user)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, fmt.Errorf("get bucket routings of %s: %w", user, err)
		}
		byBucket := map[string]*scope{}
		for pol, st := range bRepls {
			routed := effective(user)
			if to, ok := bRoutes[pol.FromBucket]; ok {
				routed = to
			}
			if pol.FromStorage != storage || routed != storage {
				continue
			}
			if c.isProtected != nil && c.isProtected(ctx, user, pol.FromBucket) {
				continue
			}
			s := byBucket[pol.FromBucket]
			if s == nil {
				s = &scope{user: user, bucket: pol.FromBucket, from: storage}
				byBucket[pol.FromBucket] = s
			}
			s.candidates = append(s.candidates, candidate{
				id: entity.UniversalFromBucketReplication(pol), to: pol.ToStorage, status: st,
				toBkt: pol.ToBucket, srcBkt: pol.FromBucket,
			})
		}
		for _, s := range byBucket {
			res = append(res, *s)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].key() < res[j].key() })
	return res, nil
}

func lagOf(st entity.ReplicationStatusExtended) int { return st.EventMigration.Unprocessed }

// evaluate returns the first target (in the operator's order) that passes all safety checks, or the reasons.
// Replications to storages that are not in targets are never considered.
func (c *Controller) evaluate(s scope, targets []string, maxLag int, status map[string]health.Status) (*candidate, string) {
	var reasons []string
	for _, target := range targets {
		var cands []candidate
		for _, cand := range s.candidates {
			if cand.to == target {
				cands = append(cands, cand)
			}
		}
		if len(cands) == 0 {
			reasons = append(reasons, target+": no replication to this target")
			continue
		}
		for _, cand := range cands {
			why := ""
			switch {
			case cand.to == s.from:
				why = "destination equals source"
			case cand.toBkt != "" && cand.toBkt != cand.srcBkt:
				why = "custom destination bucket"
			case !isAvailable(status, cand.to):
				why = "destination unavailable"
			case cand.status.ReplicationStatus != nil && cand.status.IsArchived:
				why = "replication archived"
			case cand.status.IsPaused && (cand.status.ReplicationStatus == nil || !cand.status.FailoverPending):
				why = "replication paused"
			case cand.status.Switch != nil:
				why = "replication switch in progress"
			case !cand.status.InitDone():
				why = "initial migration not done"
			case lagOf(cand.status) > maxLag:
				why = fmt.Sprintf("event lag %d exceeds maxEventLag %d", lagOf(cand.status), maxLag)
			}
			if why == "" {
				c := cand
				return &c, ""
			}
			reasons = append(reasons, cand.to+": "+why)
		}
	}
	return nil, strings.Join(reasons, "; ")
}

func isAvailable(status map[string]health.Status, storage string) bool {
	st, ok := status[storage]
	return ok && st.Available
}

func (c *Controller) failoverScope(ctx context.Context, failed string, s scope, targets []string, maxLag int, status map[string]health.Status) error {
	cand, why := c.evaluate(s, targets, maxLag, status)
	if cand == nil {
		c.mu.Lock()
		wk := failed + "|" + s.key()
		_, seen := c.warned[wk]
		c.warned[wk] = struct{}{}
		c.mu.Unlock()
		if !seen {
			c.log.Warn().Str("from", failed).Str("user", s.user).Str("bucket", s.bucket).
				Str("reason", why).Msg("failover: storage is down but no replication destination qualifies; routing is not changed")
		}
		return nil
	}
	if err := c.policy.FailoverReplication(ctx, cand.id); err != nil {
		return err
	}
	rec := pf.Event{User: s.user, Bucket: s.bucket, From: failed, To: cand.to, Time: c.now().UTC(), EventLag: lagOf(cand.status)}
	c.log.Warn().Str("user", rec.User).Str("bucket", rec.Bucket).Str("from", rec.From).Str("to", rec.To).
		Int("event_lag", rec.EventLag).Time("down_since", status[failed].LastChange).
		Msg("failover: routing switched to replica and replication from the failed storage paused; failback is manual")
	if c.onDo != nil {
		c.onDo(rec.From, rec.To)
	}
	if err := c.policies.SaveFailoverEvent(ctx, rec); err != nil {
		return fmt.Errorf("failover performed but event not saved: %w", err)
	}
	return nil
}

// RedisLocker is a Locker based on bsm/redislock.
type RedisLocker struct{ Client redis.Cmdable }

func (l RedisLocker) Obtain(ctx context.Context, key string, ttl time.Duration) (func(), bool, error) {
	lock, err := redislock.New(l.Client).Obtain(ctx, key, ttl, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return func() {
		rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = lock.Release(rctx)
	}, true, nil
}
