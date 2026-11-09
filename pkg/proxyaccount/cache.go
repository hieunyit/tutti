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

package proxyaccount

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// DefaultTTL is how long a loaded account set is used before reloading. It bounds the time a change (new account,
// rotation, disable, delete) needs to reach a proxy.
const DefaultTTL = 5 * time.Second

// Cache serves accounts from memory and reloads them from the Store when older than ttl.
// If Redis fails, the last known accounts are kept. If none were ever loaded, no dynamic account authenticates
// (fail closed).
type Cache struct {
	store *Store
	ttl   time.Duration
	now   func() time.Time

	mu      sync.RWMutex
	creds   map[string]Credential
	loaded  bool
	next    time.Time
	loading sync.Mutex
}

func NewCache(store *Store, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{store: store, ttl: ttl, now: time.Now}
}

// Lookup returns the account with the access key id, if any.
func (c *Cache) Lookup(ctx context.Context, accessKeyID string) (Credential, bool) {
	c.mu.RLock()
	due := !c.now().Before(c.next)
	c.mu.RUnlock()
	if due && c.loading.TryLock() {
		c.reload(ctx)
		c.loading.Unlock()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	cr, ok := c.creds[accessKeyID]
	return cr, ok
}

func (c *Cache) reload(ctx context.Context) {
	lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	creds, skipped, err := c.store.LoadCredentials(lctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next = c.now().Add(c.ttl)
	if err != nil {
		if c.loaded {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("proxy accounts: reload failed, using last known accounts")
		} else {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("proxy accounts: never loaded, dynamic accounts cannot authenticate")
		}
		return
	}
	if len(skipped) > 0 {
		zerolog.Ctx(ctx).Warn().Strs("access_key_ids", skipped).Msg("proxy accounts: skipped accounts that cannot be decrypted (wrong master password?)")
	}
	c.creds = creds
	c.loaded = true
}
