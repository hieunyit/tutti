/*
 * Copyright © 2026 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package accesspolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Key is the Redis hash (config DB) holding policies: field = user, value = JSON Policy.
const Key = "tutti:access-policies"

// Store persists policies in Redis.
type Store struct {
	rdb redis.Cmdable
}

func NewStore(rdb redis.Cmdable) *Store { return &Store{rdb: rdb} }

// List returns all policies sorted by user. Unparsable entries are skipped.
func (s *Store) List(ctx context.Context) ([]Policy, error) {
	m, err := s.rdb.HGetAll(ctx, Key).Result()
	if err != nil {
		return nil, fmt.Errorf("access policies: list: %w", err)
	}
	res := make([]Policy, 0, len(m))
	for user, raw := range m {
		var p Policy
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Str("user", user).Msg("access policies: skip malformed policy")
			// keep it as a deny-all policy: a corrupt entry must not unrestrict the account
			res = append(res, Policy{User: user})
			continue
		}
		p.User = user
		res = append(res, p)
	}
	SortPolicies(res)
	return res, nil
}

// Get returns the policy of the user or dom.ErrNotFound.
func (s *Store) Get(ctx context.Context, user string) (Policy, error) {
	raw, err := s.rdb.HGet(ctx, Key, user).Result()
	if errors.Is(err, redis.Nil) {
		return Policy{}, fmt.Errorf("%w: access policy of user %q", dom.ErrNotFound, user)
	}
	if err != nil {
		return Policy{}, fmt.Errorf("access policies: get: %w", err)
	}
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Policy{}, fmt.Errorf("access policies: decode %q: %w", user, err)
	}
	p.User = user
	return p, nil
}

// Set validates and stores the policy.
func (s *Store) Set(ctx context.Context, p Policy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := s.rdb.HSet(ctx, Key, p.User, raw).Err(); err != nil {
		return fmt.Errorf("access policies: set: %w", err)
	}
	return nil
}

// Delete removes the policy of the user. Returns dom.ErrNotFound if missing.
func (s *Store) Delete(ctx context.Context, user string) error {
	if user == "" {
		return fmt.Errorf("%w: access policy: user is required", dom.ErrInvalidArg)
	}
	n, err := s.rdb.HDel(ctx, Key, user).Result()
	if err != nil {
		return fmt.Errorf("access policies: delete: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: access policy of user %q", dom.ErrNotFound, user)
	}
	return nil
}

// DefaultTTL is how long a loaded policy set is used before reloading.
const DefaultTTL = 5 * time.Second

// Cache serves policies from memory and reloads them from the Store when older than ttl.
// If Redis fails, the last known policies are kept. If none were ever loaded, no account is restricted
// (as when no policy exists) and a warning is logged.
type Cache struct {
	store *Store
	ttl   time.Duration
	now   func() time.Time

	mu       sync.RWMutex
	policies map[string]Policy
	loaded   bool
	next     time.Time // next reload attempt
	loading  sync.Mutex
}

func NewCache(store *Store, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{store: store, ttl: ttl, now: time.Now}
}

// Lookup returns the policy of the user, if any.
func (c *Cache) Lookup(ctx context.Context, user string) (Policy, bool) {
	c.mu.RLock()
	due := !c.now().Before(c.next)
	c.mu.RUnlock()
	if due && c.loading.TryLock() {
		c.reload(ctx)
		c.loading.Unlock()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.policies[user]
	return p, ok
}

func (c *Cache) reload(ctx context.Context) {
	// detached from the request: a canceled client must not poison the cache
	lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	list, err := c.store.List(lctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next = c.now().Add(c.ttl)
	if err != nil {
		if c.loaded {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("access policies: reload failed, using last known policies")
		} else {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("access policies: never loaded, accounts are unrestricted until redis is reachable")
		}
		return
	}
	m := make(map[string]Policy, len(list))
	for _, p := range list {
		m[p.User] = p
	}
	c.policies = m
	c.loaded = true
}
