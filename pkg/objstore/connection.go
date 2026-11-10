// Copyright 2026 hieunyit
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package objstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3"
)

/*
Connection accounts and credential resolution (docs/tutti/scale-plan.md, 2.1).

Every S3 storage may have ONE connection account (a dedicated user on the storage). The credentials Tutti signs with
for user U on storage S are resolved in this order by ResolveS3Credentials, which every consumer uses (proxy forwarding,
worker replication/diff/migration/switch through the clients registry, failover and health checks):

 1. override(U, S): per-user credentials (config `credentials.<user>` or dynamic credentials) - unchanged behaviour
 2. connection account of S
 3. error "storage S has no credentials for user U" (dom.ErrNoCredentials; the proxy answers 503)

Redis (config DB):
  - HASH "tutti:s3users": field = user name, value = JSON {"createdAt":...}: Tutti identities. Users created without any
    storage credentials exist only here (plus their access policy and proxy access keys).
  - STRING "tutti:s3users:version": incremented on every change, polled like the storages version.
  - HASH "tutti:storage-tests": field = storage name, value = JSON of the last connection test (opaque to this package).
*/
const (
	identitiesKey        = "tutti:s3users"
	identitiesVersionKey = "tutti:s3users:version"
	storageTestsKey      = "tutti:storage-tests"
)

// RotationPropagationDelay is how long after a rotation the previous connection account must stay valid on the
// storage: proxies and workers poll the storages version at most every 2s, so 60s leaves a wide margin for slow polls,
// in-flight requests and clock differences.
const RotationPropagationDelay = 60 * time.Second

// CredMode tells where the credentials of a user on a storage come from.
type CredMode string

const (
	// ModeOverride: per-user credentials (config or dynamic).
	ModeOverride CredMode = "override"
	// ModeConnection: the connection account of the storage.
	ModeConnection CredMode = "connection-account"
	// ModeMissing: nothing resolves, requests fail.
	ModeMissing CredMode = "missing"
)

// ConnectionInfo is the connection account of a storage.
type ConnectionInfo struct {
	Credentials s3.CredentialsV4
	// Source is StorageSourceConfig (read-only) or StorageSourceManaged.
	Source string
	// Previous is the account replaced by the last rotation (managed only), zero if none.
	Previous  s3.CredentialsV4
	RotatedAt time.Time
}

// SafeToRevokePreviousAfter is the time before which the previous account must not be revoked on the storage.
func (c ConnectionInfo) SafeToRevokePreviousAfter() time.Time {
	if c.RotatedAt.IsZero() {
		return time.Time{}
	}
	return c.RotatedAt.Add(RotationPropagationDelay)
}

// NoCredentialsError is returned when a user cannot be resolved on a storage.
type NoCredentialsError struct{ Storage, User string }

func (e *NoCredentialsError) Error() string {
	return fmt.Sprintf("storage %s has no credentials for user %s", e.Storage, e.User)
}

func (e *NoCredentialsError) Unwrap() error { return dom.ErrNoCredentials }

// ConnectionAccount returns the connection account of the S3 storage.
func (s *credsSvc) ConnectionAccount(storage string) (ConnectionInfo, bool) {
	c, ok := s.view.Load().conn[storage]
	return c, ok
}

// ResolveS3Credentials resolves the credentials for user on storage: override, then connection account, otherwise a
// *NoCredentialsError (dom.ErrNoCredentials). It returns dom.ErrNotFound for an unknown S3 storage.
func (s *credsSvc) ResolveS3Credentials(storage, user string) (s3.CredentialsV4, CredMode, error) {
	view := s.view.Load()
	if _, ok := view.s3[storage]; !ok {
		return s3.CredentialsV4{}, ModeMissing, dom.ErrNotFound
	}
	if c, err := s.GetS3Credentials(storage, user); err == nil {
		return c, ModeOverride, nil
	}
	if c, ok := view.conn[storage]; ok {
		return c.Credentials, ModeConnection, nil
	}
	return s3.CredentialsV4{}, ModeMissing, &NoCredentialsError{Storage: storage, User: user}
}

// ListOverrideUsers lists the users with their own credentials on the storage (config and dynamic).
func (s *credsSvc) ListOverrideUsers(storage string) []string {
	return s.listOverrideUsers(storage)
}

// ---- identities

type identityDoc struct {
	CreatedAt time.Time `json:"createdAt"`
}

func (s *credsSvc) identitiesEnabled() bool { return s.managedClient != nil }

// Identities returns the sorted names of the users recorded as Tutti identities.
func (s *credsSvc) Identities() []string {
	m := s.identities.Load()
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(*m))
	for u := range *m {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// HasIdentity reports whether the user is recorded as a Tutti identity.
func (s *credsSvc) HasIdentity(user string) bool {
	m := s.identities.Load()
	if m == nil {
		return false
	}
	_, ok := (*m)[user]
	return ok
}

// AddIdentity records a user as a Tutti identity (a user without storage credentials has no other record).
// dom.ErrAlreadyExists if it exists.
func (s *credsSvc) AddIdentity(ctx context.Context, user string) error {
	if !s.identitiesEnabled() {
		return fmt.Errorf("%w: user records are not available on this server (config DB Redis is not configured)", dom.ErrFailedPrecondition)
	}
	raw, err := json.Marshal(identityDoc{CreatedAt: time.Now().UTC().Truncate(time.Second)})
	if err != nil {
		return err
	}
	n, err := redis.NewScript(`if redis.call('hexists',KEYS[1],ARGV[1]) == 1 then return 0 end; redis.call('hset',KEYS[1],ARGV[1],ARGV[2]); redis.call('incr',KEYS[2]); return 1`).
		Run(ctx, s.managedClient, []string{identitiesKey, identitiesVersionKey}, user, raw).Int()
	if err != nil {
		return fmt.Errorf("failed to store user record: %w", err)
	}
	if err := s.syncIdentities(ctx); err != nil {
		return fmt.Errorf("failed to sync user records: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: user %q", dom.ErrAlreadyExists, user)
	}
	return nil
}

// RemoveIdentity removes the Tutti identity record of a user (no error if there is none).
func (s *credsSvc) RemoveIdentity(ctx context.Context, user string) error {
	if !s.identitiesEnabled() {
		return nil
	}
	tx := s.managedClient.TxPipeline()
	tx.HDel(ctx, identitiesKey, user)
	tx.Incr(ctx, identitiesVersionKey)
	if _, err := tx.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete user record: %w", err)
	}
	return s.syncIdentities(ctx)
}

func (s *credsSvc) syncIdentities(ctx context.Context) error {
	if !s.identitiesEnabled() {
		return nil
	}
	s.identMu.Lock()
	defer s.identMu.Unlock()
	remote, err := s.managedClient.Get(ctx, identitiesVersionKey).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get user records version from redis: %w", err)
	}
	if s.identLoaded && remote == s.identVer {
		return nil
	}
	fields, err := s.managedClient.HKeys(ctx, identitiesKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user records from redis: %w", err)
	}
	m := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		m[f] = struct{}{}
	}
	s.identities.Store(&m)
	s.identVer, s.identLoaded = remote, true
	return nil
}

// ---- last connection test

// SaveStorageTest stores the last connection test result (JSON) of a storage. Without a config DB it is a no-op.
func (s *credsSvc) SaveStorageTest(ctx context.Context, storage string, resultJSON []byte) error {
	if s.managedClient == nil {
		return nil
	}
	return s.managedClient.HSet(ctx, storageTestsKey, storage, resultJSON).Err()
}

// StorageTests returns the stored connection test results (JSON) by storage name.
func (s *credsSvc) StorageTests(ctx context.Context) (map[string]string, error) {
	if s.managedClient == nil {
		return nil, nil
	}
	res, err := s.managedClient.HGetAll(ctx, storageTestsKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	return res, nil
}
