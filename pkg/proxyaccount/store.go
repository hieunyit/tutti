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

// Package proxyaccount implements runtime-managed ("dynamic") proxy accounts: S3 access keys that authenticate
// clients against the proxy and act as an existing storage user.
//
// Accounts live in the Redis config DB hash Key: field = access key id, value = JSON record. Secrets are sealed
// (AES-GCM, key derived from the dynamic credentials master password, see objstore.Seal) before they are stored;
// plaintext secrets are never written to Redis. Proxies read the hash through Cache (short TTL), so changes reach all
// proxies within Cache TTL (5s by default).
package proxyaccount

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Key is the Redis hash (config DB) holding accounts: field = access key id, value = JSON record.
const Key = "tutti:proxy-accounts"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"

	// DefaultGracePeriod is how long the previous secret keeps working after a rotation if not requested otherwise.
	DefaultGracePeriod = time.Hour
	// MaxGracePeriod limits the rotation grace period.
	MaxGracePeriod = 30 * 24 * time.Hour

	accessKeyPrefix = "TUT"
	accessKeyRandom = 17
	secretLength    = 40
)

// Sealer encrypts and decrypts secrets. It is implemented by the dynamic credentials service.
type Sealer interface {
	Seal(plaintext []byte) (string, error)
	Open(sealed string) ([]byte, error)
}

// Account is the public metadata of an account. It never contains secrets.
type Account struct {
	AccessKeyID string
	// User is the storage user the proxy acts as.
	User        string
	Description string
	Status      string
	CreatedAt   time.Time
	RotatedAt   time.Time
	// PreviousExpiresAt is set while a rotated-out secret may still be valid (check against now).
	PreviousExpiresAt time.Time
}

// Credential is an account with its decrypted secrets, for request authentication only.
type Credential struct {
	Account
	Secret            string
	PreviousSecret    string
	previousExpiresAt time.Time
}

// Secrets returns the secrets valid at now: the current one, and the previous one during the rotation grace period.
func (c Credential) Secrets(now time.Time) []string {
	res := []string{c.Secret}
	if c.PreviousSecret != "" && now.Before(c.previousExpiresAt) {
		res = append(res, c.PreviousSecret)
	}
	return res
}

type record struct {
	User              string    `json:"user"`
	Description       string    `json:"description,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	RotatedAt         time.Time `json:"rotatedAt,omitempty"`
	Secret            string    `json:"secret"`
	PreviousSecret    string    `json:"previousSecret,omitempty"`
	PreviousExpiresAt time.Time `json:"previousExpiresAt,omitempty"`
}

func (r record) account(id string) Account {
	a := Account{AccessKeyID: id, User: r.User, Description: r.Description, Status: r.Status, CreatedAt: r.CreatedAt, RotatedAt: r.RotatedAt}
	if r.PreviousSecret != "" {
		a.PreviousExpiresAt = r.PreviousExpiresAt
	}
	return a
}

// Store persists accounts in Redis.
type Store struct {
	rdb    redis.Cmdable
	sealer func() (Sealer, error)
	now    func() time.Time
}

// NewStore creates a store. sealer is evaluated on every use; it must fail with dom.ErrFailedPrecondition
// if no master password is configured.
func NewStore(rdb redis.Cmdable, sealer func() (Sealer, error)) *Store {
	return &Store{rdb: rdb, sealer: sealer, now: time.Now}
}

func (s *Store) getSealer() (Sealer, error) {
	if s.sealer == nil {
		return nil, fmt.Errorf("%w: sealing secrets requires storage.dynamicCredentials enabled with a masterPassword", dom.ErrFailedPrecondition)
	}
	return s.sealer()
}

// SealingAvailable reports whether secrets can be sealed, i.e. whether Create can work.
func (s *Store) SealingAvailable() error {
	_, err := s.getSealer()
	return err
}

// ValidStatus reports whether status is a known account status.
func ValidStatus(status string) bool { return status == StatusActive || status == StatusDisabled }

// Create generates and stores a new active account for user and returns it with its secret (the only time it is
// available in plaintext).
func (s *Store) Create(ctx context.Context, user, description string) (Account, string, error) {
	if user == "" {
		return Account{}, "", fmt.Errorf("%w: proxy account: user is required", dom.ErrInvalidArg)
	}
	sealer, err := s.getSealer()
	if err != nil {
		return Account{}, "", err
	}
	secret, err := randomString(secretLength, alnum)
	if err != nil {
		return Account{}, "", err
	}
	sealed, err := sealer.Seal([]byte(secret))
	if err != nil {
		return Account{}, "", err
	}
	rec := record{User: user, Description: description, Status: StatusActive, CreatedAt: s.now().UTC().Truncate(time.Second), Secret: sealed}
	raw, err := json.Marshal(rec)
	if err != nil {
		return Account{}, "", err
	}
	for range 5 {
		suffix, err := randomString(accessKeyRandom, upperAlnum)
		if err != nil {
			return Account{}, "", err
		}
		id := accessKeyPrefix + suffix
		ok, err := s.rdb.HSetNX(ctx, Key, id, raw).Result()
		if err != nil {
			return Account{}, "", fmt.Errorf("proxy accounts: create: %w", err)
		}
		if ok {
			return rec.account(id), secret, nil
		}
	}
	return Account{}, "", fmt.Errorf("%w: proxy accounts: unable to generate a unique access key id", dom.ErrInternal)
}

// Rotate generates a new secret. With grace > 0 the previous secret stays valid for grace; with 0 it is invalidated
// immediately. Returns the new secret (the only time it is available in plaintext).
func (s *Store) Rotate(ctx context.Context, id string, grace time.Duration) (Account, string, error) {
	if grace < 0 || grace > MaxGracePeriod {
		return Account{}, "", fmt.Errorf("%w: proxy account: grace period must be between 0 and %s", dom.ErrInvalidArg, MaxGracePeriod)
	}
	sealer, err := s.getSealer()
	if err != nil {
		return Account{}, "", err
	}
	secret, err := randomString(secretLength, alnum)
	if err != nil {
		return Account{}, "", err
	}
	sealed, err := sealer.Seal([]byte(secret))
	if err != nil {
		return Account{}, "", err
	}
	var acc Account
	err = s.update(ctx, id, func(r *record) error {
		now := s.now().UTC()
		if grace > 0 {
			r.PreviousSecret = r.Secret
			r.PreviousExpiresAt = now.Add(grace).Truncate(time.Second)
		} else {
			r.PreviousSecret = ""
			r.PreviousExpiresAt = time.Time{}
		}
		r.Secret = sealed
		r.RotatedAt = now.Truncate(time.Second)
		acc = r.account(id)
		return nil
	})
	if err != nil {
		return Account{}, "", err
	}
	return acc, secret, nil
}

// SetStatus enables or disables an account.
func (s *Store) SetStatus(ctx context.Context, id, status string) error {
	if !ValidStatus(status) {
		return fmt.Errorf("%w: proxy account: status must be %q or %q", dom.ErrInvalidArg, StatusActive, StatusDisabled)
	}
	return s.update(ctx, id, func(r *record) error { r.Status = status; return nil })
}

// Delete removes an account. Returns dom.ErrNotFound if missing.
func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("%w: proxy account: access key id is required", dom.ErrInvalidArg)
	}
	n, err := s.rdb.HDel(ctx, Key, id).Result()
	if err != nil {
		return fmt.Errorf("proxy accounts: delete: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: proxy account %q", dom.ErrNotFound, id)
	}
	return nil
}

// List returns the metadata of all accounts sorted by user and access key id. Secrets are not read.
func (s *Store) List(ctx context.Context) ([]Account, error) {
	m, err := s.rdb.HGetAll(ctx, Key).Result()
	if err != nil {
		return nil, fmt.Errorf("proxy accounts: list: %w", err)
	}
	res := make([]Account, 0, len(m))
	for id, raw := range m {
		var r record
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			continue
		}
		res = append(res, r.account(id))
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].User != res[j].User {
			return res[i].User < res[j].User
		}
		return res[i].AccessKeyID < res[j].AccessKeyID
	})
	return res, nil
}

// LoadCredentials returns all accounts with decrypted secrets keyed by access key id. Accounts whose secrets
// cannot be opened are skipped (they cannot authenticate) and reported in skipped.
func (s *Store) LoadCredentials(ctx context.Context) (creds map[string]Credential, skipped []string, err error) {
	m, err := s.rdb.HGetAll(ctx, Key).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("proxy accounts: load: %w", err)
	}
	creds = make(map[string]Credential, len(m))
	if len(m) == 0 {
		return creds, nil, nil
	}
	sealer, err := s.getSealer()
	if err != nil {
		return nil, nil, err
	}
	for id, raw := range m {
		var r record
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			skipped = append(skipped, id)
			continue
		}
		secret, err := sealer.Open(r.Secret)
		if err != nil {
			skipped = append(skipped, id)
			continue
		}
		c := Credential{Account: r.account(id), Secret: string(secret), previousExpiresAt: r.PreviousExpiresAt}
		if r.PreviousSecret != "" {
			prev, err := sealer.Open(r.PreviousSecret)
			if err != nil {
				skipped = append(skipped, id)
				continue
			}
			c.PreviousSecret = string(prev)
		}
		creds[id] = c
	}
	return creds, skipped, nil
}

// luaCAS replaces the field value only if it still equals the expected one.
var luaCAS = redis.NewScript(`if redis.call('hget',KEYS[1],ARGV[1]) == ARGV[2] then redis.call('hset',KEYS[1],ARGV[1],ARGV[3]); return 1 else return 0 end`)

// update applies fn to the record with compare-and-set, retrying on concurrent changes.
func (s *Store) update(ctx context.Context, id string, fn func(*record) error) error {
	if id == "" {
		return fmt.Errorf("%w: proxy account: access key id is required", dom.ErrInvalidArg)
	}
	for range 10 {
		old, err := s.rdb.HGet(ctx, Key, id).Result()
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("%w: proxy account %q", dom.ErrNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("proxy accounts: get: %w", err)
		}
		var r record
		if err := json.Unmarshal([]byte(old), &r); err != nil {
			return fmt.Errorf("%w: proxy account %q: malformed record: %v", dom.ErrInternal, id, err)
		}
		if err := fn(&r); err != nil {
			return err
		}
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		n, err := luaCAS.Run(ctx, s.rdb, []string{Key}, id, old, string(raw)).Int()
		if err != nil {
			return fmt.Errorf("proxy accounts: update: %w", err)
		}
		if n == 1 {
			return nil
		}
	}
	return fmt.Errorf("%w: proxy account %q: too many concurrent changes", dom.ErrInternal, id)
}

const (
	alnum      = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	upperAlnum = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// randomString returns n characters of alphabet chosen uniformly with crypto/rand.
func randomString(n int, alphabet string) (string, error) {
	max := big.NewInt(int64(len(alphabet)))
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("%w: proxy accounts: random: %v", dom.ErrInternal, err)
		}
		b[i] = alphabet[v.Int64()]
	}
	return string(b), nil
}
