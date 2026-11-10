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
	"maps"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/s3"
)

/*
Managed storages are S3 storages created at runtime through the management API.

Redis (config DB):
  - HASH "tutti:storages": field = storage name, value = JSON document with the connection settings (never secrets)
  - STRING "tutti:storages:version": incremented on every change; workers and proxies poll it like the dynamic
    credentials and rebuild their view of the storages when it changes.

The credentials of the storage users are regular dynamic credentials (sealed with the master password).
Storages defined in the config are never overridden by managed ones.
*/
const (
	managedStoragesKey        = "tutti:storages"
	managedStoragesVersionKey = "tutti:storages:version"
)

// Sources of a storage definition.
const (
	StorageSourceConfig  = "config"
	StorageSourceManaged = "managed"
)

var storageNameRe = regexp.MustCompile(`^[a-z0-9_-]{1,63}$`)

// ValidStorageName checks a storage name: 1-63 characters, lowercase letters, digits, '-' and '_'.
func ValidStorageName(name string) error {
	if !storageNameRe.MatchString(name) {
		return fmt.Errorf("%w: storage name must be 1-63 characters: lowercase letters, digits, '-' and '_'", dom.ErrInvalidArg)
	}
	return nil
}

// ManagedStorage is the runtime-managed definition of an S3 storage.
type ManagedStorage struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Address   s3.StorageAddress
	RateLimit ratelimit.RateLimit
	// ConnectionAccount is the account Tutti uses on the storage for users without a per-user override. Stored sealed.
	// On UpdateManaged an empty value keeps the stored account (unless ClearConnectionAccount is set).
	ConnectionAccount s3.CredentialsV4
	// PreviousAccount is the account replaced by the last rotation, kept until the next rotation so that proxies that
	// have not polled the new version yet keep working. RotatedAt is the time of the rotation.
	PreviousAccount s3.CredentialsV4
	RotatedAt       time.Time
	// LastForcedSaveBy/At record who saved the storage although its connection test had failing steps.
	LastForcedSaveBy string
	LastForcedSaveAt time.Time
	// ClearConnectionAccount (UpdateManaged input only, not stored) removes the connection account.
	ClearConnectionAccount bool
}

// Normalize validates the definition and fills defaults (address scheme, http timeout).
func (m ManagedStorage) Normalize() (ManagedStorage, error) {
	if err := ValidStorageName(m.Name); err != nil {
		return m, err
	}
	st := s3.Storage{StorageAddress: m.Address}
	if err := st.Validate(); err != nil {
		return m, fmt.Errorf("%w: %w", dom.ErrInvalidArg, err)
	}
	m.Address = st.StorageAddress
	u, err := url.Parse(m.Address.Address)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return m, fmt.Errorf("%w: storage address must be an HTTP(S) endpoint without credentials, path, query or fragment", dom.ErrInvalidArg)
	}
	m.Address.Address = strings.TrimSuffix(m.Address.Address, "/")
	if m.Address.HttpTimeout < 0 {
		return m, fmt.Errorf("%w: httpTimeout must not be negative", dom.ErrInvalidArg)
	}
	if m.Address.HealthCheckInterval < 0 || (m.Address.HealthCheckInterval > 0 && m.Address.HealthCheckInterval < time.Second) {
		return m, fmt.Errorf("%w: healthCheckInterval must be at least 1s (or 0 for the default)", dom.ErrInvalidArg)
	}
	if m.RateLimit.Enabled && m.RateLimit.RPM <= 0 {
		return m, fmt.Errorf("%w: rateLimit.rpm must be greater than zero when the rate limit is enabled", dom.ErrInvalidArg)
	}
	if m.RateLimit.RPM < 0 {
		return m, fmt.Errorf("%w: rateLimit.rpm must not be negative", dom.ErrInvalidArg)
	}
	if m.ConnectionAccount != (s3.CredentialsV4{}) {
		if err := m.ConnectionAccount.Validate(); err != nil {
			return m, fmt.Errorf("%w: connection account: %w", dom.ErrInvalidArg, err)
		}
	}
	return m, nil
}

type managedDoc struct {
	Name                string    `json:"name"`
	Address             string    `json:"address"`
	Provider            string    `json:"provider"`
	DefaultRegion       string    `json:"defaultRegion,omitempty"`
	IsSecure            bool      `json:"isSecure"`
	HealthCheckInterval int64     `json:"healthCheckIntervalNs,omitempty"`
	HttpTimeout         int64     `json:"httpTimeoutNs,omitempty"`
	RateEnabled         bool      `json:"rateLimitEnabled,omitempty"`
	RateRPM             int       `json:"rateLimitRpm,omitempty"`
	RateMetadata        bool      `json:"rateLimitIncludeMetadataApi,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
	// Sealed JSON of s3.CredentialsV4 (AES-GCM with the dynamic credentials master key).
	ConnectionAccount string    `json:"connectionAccount,omitempty"`
	PreviousAccount   string    `json:"previousAccount,omitempty"`
	RotatedAt         time.Time `json:"rotatedAt,omitzero"`
	LastForcedSaveBy  string    `json:"lastForcedSaveBy,omitempty"`
	LastForcedSaveAt  time.Time `json:"lastForcedSaveAt,omitzero"`
}

// sealer seals and opens the credentials in storage documents.
type sealer interface {
	Seal([]byte) (string, error)
	Open(string) ([]byte, error)
}

func sealCred(sl sealer, c s3.CredentialsV4) (string, error) {
	if c == (s3.CredentialsV4{}) {
		return "", nil
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return sl.Seal(raw)
}

func openCred(sl sealer, sealed string) (s3.CredentialsV4, error) {
	var c s3.CredentialsV4
	if sealed == "" {
		return c, nil
	}
	raw, err := sl.Open(sealed)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, &c)
	return c, err
}

func (m ManagedStorage) marshal(sl sealer) ([]byte, error) {
	conn, err := sealCred(sl, m.ConnectionAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to seal the connection account: %w", err)
	}
	prev, err := sealCred(sl, m.PreviousAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to seal the previous connection account: %w", err)
	}
	return json.Marshal(managedDoc{
		ConnectionAccount: conn, PreviousAccount: prev, RotatedAt: m.RotatedAt,
		LastForcedSaveBy: m.LastForcedSaveBy, LastForcedSaveAt: m.LastForcedSaveAt,
		Name: m.Name, Address: m.Address.Address, Provider: string(m.Address.Provider), DefaultRegion: m.Address.DefaultRegion,
		IsSecure: m.Address.IsSecure, HealthCheckInterval: int64(m.Address.HealthCheckInterval), HttpTimeout: int64(m.Address.HttpTimeout),
		RateEnabled: m.RateLimit.Enabled, RateRPM: m.RateLimit.RPM, RateMetadata: m.RateLimit.IncludeMetadataAPI,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	})
}

func unmarshalManaged(sl sealer, field string, raw string) (ManagedStorage, error) {
	var d managedDoc
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return ManagedStorage{}, err
	}
	if d.Name != field {
		return ManagedStorage{}, fmt.Errorf("document name %q does not match field %q", d.Name, field)
	}
	conn, err := openCred(sl, d.ConnectionAccount)
	if err != nil {
		return ManagedStorage{}, fmt.Errorf("failed to open the connection account: %w", err)
	}
	prev, err := openCred(sl, d.PreviousAccount)
	if err != nil {
		return ManagedStorage{}, fmt.Errorf("failed to open the previous connection account: %w", err)
	}
	return ManagedStorage{
		ConnectionAccount: conn, PreviousAccount: prev, RotatedAt: d.RotatedAt,
		LastForcedSaveBy: d.LastForcedSaveBy, LastForcedSaveAt: d.LastForcedSaveAt,
		Name: d.Name,
		Address: s3.StorageAddress{
			Address: d.Address, Provider: s3.Provider(d.Provider), DefaultRegion: d.DefaultRegion, IsSecure: d.IsSecure,
			HealthCheckInterval: time.Duration(d.HealthCheckInterval), HttpTimeout: time.Duration(d.HttpTimeout),
		},
		RateLimit: ratelimit.RateLimit{Enabled: d.RateEnabled, RPM: d.RateRPM, IncludeMetadataAPI: d.RateMetadata},
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}, nil
}

// storagesView is an immutable snapshot of all known storages (config + managed).
type storagesView struct {
	types   map[string]dom.StorageType
	s3      map[string]s3.StorageAddress
	rate    map[string]ratelimit.RateLimit
	managed map[string]ManagedStorage
	// conn holds the connection account of S3 storages (config or managed).
	conn map[string]ConnectionInfo
}

func buildView(conf *Config, managed map[string]ManagedStorage) *storagesView {
	v := &storagesView{
		types:   conf.Types(),
		s3:      make(map[string]s3.StorageAddress),
		rate:    conf.RateLimitConf(),
		managed: managed,
		conn:    make(map[string]ConnectionInfo),
	}
	for name, st := range conf.S3Storages() {
		v.s3[name] = st.StorageAddress
		if st.ConnectionAccount != nil {
			v.conn[name] = ConnectionInfo{Credentials: *st.ConnectionAccount, Source: StorageSourceConfig}
		}
	}
	for name, m := range managed {
		if _, ok := conf.Storages[name]; ok {
			continue // config always wins
		}
		v.types[name] = dom.S3
		v.s3[name] = m.Address
		v.rate[name] = m.RateLimit
		if m.ConnectionAccount != (s3.CredentialsV4{}) {
			v.conn[name] = ConnectionInfo{Credentials: m.ConnectionAccount, Source: StorageSourceManaged, Previous: m.PreviousAccount, RotatedAt: m.RotatedAt}
		}
	}
	return v
}

// StorageChange describes a change of the storages applied locally.
type StorageChange struct {
	Name string
	// Kind is "added", "updated" or "removed".
	Kind string
}

// ManagedStoragesOption enables runtime-managed storages stored in the given (config DB) Redis.
func WithManagedStorages(rc redis.UniversalClient) CredsSvcOption {
	return func(s *credsSvc) { s.managedClient = rc }
}

// WithControlPlane attaches a PostgreSQL control-plane store for storages, identities, dynamic credentials and tests,
// with Redis fallback on PostgreSQL read failure.
func WithControlPlane(cp controlplane.Store) CredsSvcOption {
	return func(s *credsSvc) { s.cp = cp }
}

// ControlPlane returns the attached PostgreSQL control-plane store, or nil when running in Redis mode.
func (s *credsSvc) ControlPlane() controlplane.Store { return s.cp }

// CredsSvcOption customizes the credentials service.
type CredsSvcOption func(*credsSvc)

// ManagedStoragesEnabled reports whether managed storages are active (Redis or PG given and dynamic credentials enabled).
func (s *credsSvc) ManagedStoragesEnabled() bool {
	return (s.managedClient != nil || s.cp != nil) && s.sealingAvailable() == nil
}

// Subscribe registers a callback invoked (outside any lock) after the storages changed locally, either by a poll or by
// a change made through this service.
func (s *credsSvc) Subscribe(fn func(changes []StorageChange)) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.listeners = append(s.listeners, fn)
}

func (s *credsSvc) notify(changes []StorageChange) {
	if len(changes) == 0 {
		return
	}
	s.listenersMu.Lock()
	ls := append([]func([]StorageChange){}, s.listeners...)
	s.listenersMu.Unlock()
	for _, fn := range ls {
		fn(changes)
	}
}

// RateLimits returns the rate limit settings of all storages.
func (s *credsSvc) RateLimits() map[string]ratelimit.RateLimit {
	return maps.Clone(s.view.Load().rate)
}

// RateLimit reads one immutable snapshot without copying the full map on the request path.
func (s *credsSvc) RateLimit(name string) (ratelimit.RateLimit, bool) {
	v, ok := s.view.Load().rate[name]
	return v, ok
}

// StorageSource tells where a storage is defined: StorageSourceConfig, StorageSourceManaged or "" if unknown.
func (s *credsSvc) StorageSource(name string) string {
	if _, ok := s.config.Storages[name]; ok {
		return StorageSourceConfig
	}
	if _, ok := s.view.Load().managed[name]; ok {
		return StorageSourceManaged
	}
	return ""
}

// ListManaged returns the managed storages sorted by name.
func (s *credsSvc) ListManaged() []ManagedStorage {
	v := s.view.Load()
	res := make([]ManagedStorage, 0, len(v.managed))
	for name, m := range v.managed {
		if _, ok := s.config.Storages[name]; ok {
			continue
		}
		res = append(res, m)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	return res
}

// GetManaged returns one managed storage.
func (s *credsSvc) GetManaged(name string) (ManagedStorage, bool) {
	m, ok := s.view.Load().managed[name]
	if _, inConf := s.config.Storages[name]; inConf {
		return ManagedStorage{}, false
	}
	return m, ok
}

func (s *credsSvc) managedPrecondition() error {
	if s.managedClient == nil && s.cp == nil {
		return fmt.Errorf("%w: managed storages are not available on this server", dom.ErrFailedPrecondition)
	}
	if err := s.sealingAvailable(); err != nil {
		return fmt.Errorf("%w: managed storages require storage.dynamicCredentials enabled with a masterPassword (encryption on): the credentials of their users are stored sealed", dom.ErrFailedPrecondition)
	}
	return nil
}

func docToStorageRecord(d managedDoc) controlplane.StorageRecord {
	return controlplane.StorageRecord{
		Name:                d.Name,
		Address:             d.Address,
		Provider:            d.Provider,
		DefaultRegion:       d.DefaultRegion,
		IsSecure:            d.IsSecure,
		HealthCheckInterval: d.HealthCheckInterval,
		HttpTimeout:         d.HttpTimeout,
		RateEnabled:         d.RateEnabled,
		RateRPM:             d.RateRPM,
		RateMetadata:        d.RateMetadata,
		CreatedAt:           d.CreatedAt,
		UpdatedAt:           d.UpdatedAt,
		ConnectionAccount:   d.ConnectionAccount,
		PreviousAccount:     d.PreviousAccount,
		RotatedAt:           d.RotatedAt,
		LastForcedSaveBy:    d.LastForcedSaveBy,
		LastForcedSaveAt:    d.LastForcedSaveAt,
	}
}

func docFromStorageRecord(r controlplane.StorageRecord) managedDoc {
	return managedDoc{
		Name:                r.Name,
		Address:             r.Address,
		Provider:            r.Provider,
		DefaultRegion:       r.DefaultRegion,
		IsSecure:            r.IsSecure,
		HealthCheckInterval: r.HealthCheckInterval,
		HttpTimeout:         r.HttpTimeout,
		RateEnabled:         r.RateEnabled,
		RateRPM:             r.RateRPM,
		RateMetadata:        r.RateMetadata,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
		ConnectionAccount:   r.ConnectionAccount,
		PreviousAccount:     r.PreviousAccount,
		RotatedAt:           r.RotatedAt,
		LastForcedSaveBy:    r.LastForcedSaveBy,
		LastForcedSaveAt:    r.LastForcedSaveAt,
	}
}

func (m ManagedStorage) toRecord(sl sealer) (controlplane.StorageRecord, error) {
	raw, err := m.marshal(sl)
	if err != nil {
		return controlplane.StorageRecord{}, err
	}
	var d managedDoc
	if err := json.Unmarshal(raw, &d); err != nil {
		return controlplane.StorageRecord{}, err
	}
	return docToStorageRecord(d), nil
}

func unmarshalRecord(sl sealer, r controlplane.StorageRecord) (ManagedStorage, error) {
	raw, err := json.Marshal(docFromStorageRecord(r))
	if err != nil {
		return ManagedStorage{}, err
	}
	return unmarshalManaged(sl, r.Name, string(raw))
}

// CreateManaged validates and stores a new managed storage. It fails with dom.ErrAlreadyExists if the name is taken
// by a config or managed storage.
func (s *credsSvc) CreateManaged(ctx context.Context, m ManagedStorage) (ManagedStorage, error) {
	if err := s.managedPrecondition(); err != nil {
		return m, err
	}
	m, err := m.Normalize()
	if err != nil {
		return m, err
	}
	if _, ok := s.config.Storages[m.Name]; ok {
		return m, fmt.Errorf("%w: storage %q is already defined in config", dom.ErrAlreadyExists, m.Name)
	}
	now := time.Now().UTC().Truncate(time.Second)
	m.CreatedAt, m.UpdatedAt = now, now
	if s.cp != nil {
		rec, err := m.toRecord(s)
		if err != nil {
			return m, err
		}
		if _, err := s.cp.Repos().Storages.Create(ctx, rec); err != nil {
			return m, controlplane.WrapWriteError(err)
		}
		if err := s.syncManaged(ctx); err != nil {
			return m, fmt.Errorf("failed to sync storages after create: %w", err)
		}
		return m, nil
	}
	raw, err := m.marshal(s)
	if err != nil {
		return m, err
	}
	n, err := redis.NewScript(`if redis.call('hexists',KEYS[1],ARGV[1]) == 1 then return 0 end; redis.call('hset',KEYS[1],ARGV[1],ARGV[2]); redis.call('incr',KEYS[2]); return 1`).Run(ctx, s.managedClient, []string{managedStoragesKey, managedStoragesVersionKey}, m.Name, raw).Int()
	if err != nil {
		return m, fmt.Errorf("failed to store storage: %w", err)
	}
	if n == 0 {
		return m, fmt.Errorf("%w: storage %q already exists", dom.ErrAlreadyExists, m.Name)
	}
	if err := s.syncManaged(ctx); err != nil {
		return m, fmt.Errorf("failed to sync storages after create: %w", err)
	}
	return m, nil
}

// UpdateManaged replaces the settings of an existing managed storage (CreatedAt is kept).
func (s *credsSvc) UpdateManaged(ctx context.Context, m ManagedStorage) (ManagedStorage, error) {
	if err := s.managedPrecondition(); err != nil {
		return m, err
	}
	if _, ok := s.config.Storages[m.Name]; ok {
		return m, fmt.Errorf("%w: storage %q is defined in config and cannot be changed through the API", dom.ErrFailedPrecondition, m.Name)
	}
	m, err := m.Normalize()
	if err != nil {
		return m, err
	}
	if s.cp != nil {
		var updated ManagedStorage
		_, err := s.cp.Repos().Storages.Mutate(ctx, m.Name, func(rec *controlplane.StorageRecord) error {
			old, err := unmarshalRecord(s, *rec)
			if err != nil {
				return fmt.Errorf("%w: stored storage %q is corrupted: %w", dom.ErrInternal, m.Name, err)
			}
			next := m
			next.CreatedAt, next.UpdatedAt = old.CreatedAt, time.Now().UTC().Truncate(time.Second)
			switch {
			case next.ClearConnectionAccount:
				next.ConnectionAccount, next.PreviousAccount, next.RotatedAt = s3.CredentialsV4{}, s3.CredentialsV4{}, time.Time{}
			case next.ConnectionAccount == (s3.CredentialsV4{}):
				next.ConnectionAccount, next.PreviousAccount, next.RotatedAt = old.ConnectionAccount, old.PreviousAccount, old.RotatedAt
			case next.ConnectionAccount != old.ConnectionAccount:
				next.PreviousAccount, next.RotatedAt = old.ConnectionAccount, next.UpdatedAt
			default:
				next.PreviousAccount, next.RotatedAt = old.PreviousAccount, old.RotatedAt
			}
			if next.LastForcedSaveBy == "" {
				next.LastForcedSaveBy, next.LastForcedSaveAt = old.LastForcedSaveBy, old.LastForcedSaveAt
			}
			next.ClearConnectionAccount = false
			newRec, err := next.toRecord(s)
			if err != nil {
				return err
			}
			*rec = newRec
			updated = next
			return nil
		})
		if err != nil {
			return m, controlplane.WrapWriteError(err)
		}
		if err := s.syncManaged(ctx); err != nil {
			return updated, fmt.Errorf("failed to sync storages after update: %w", err)
		}
		return updated, nil
	}
	rawOld, err := s.managedClient.HGet(ctx, managedStoragesKey, m.Name).Result()
	if errors.Is(err, redis.Nil) {
		return m, fmt.Errorf("%w: storage %q", dom.ErrNotFound, m.Name)
	}
	if err != nil {
		return m, fmt.Errorf("failed to read storage from redis: %w", err)
	}
	old, err := unmarshalManaged(s, m.Name, rawOld)
	if err != nil {
		return m, fmt.Errorf("%w: stored storage %q is corrupted: %w", dom.ErrInternal, m.Name, err)
	}
	m.CreatedAt, m.UpdatedAt = old.CreatedAt, time.Now().UTC().Truncate(time.Second)
	switch {
	case m.ClearConnectionAccount:
		m.ConnectionAccount, m.PreviousAccount, m.RotatedAt = s3.CredentialsV4{}, s3.CredentialsV4{}, time.Time{}
	case m.ConnectionAccount == (s3.CredentialsV4{}):
		m.ConnectionAccount, m.PreviousAccount, m.RotatedAt = old.ConnectionAccount, old.PreviousAccount, old.RotatedAt
	case m.ConnectionAccount != old.ConnectionAccount:
		m.PreviousAccount, m.RotatedAt = old.ConnectionAccount, m.UpdatedAt
	default:
		m.PreviousAccount, m.RotatedAt = old.PreviousAccount, old.RotatedAt
	}
	if m.LastForcedSaveBy == "" {
		m.LastForcedSaveBy, m.LastForcedSaveAt = old.LastForcedSaveBy, old.LastForcedSaveAt
	}
	m.ClearConnectionAccount = false
	raw, err := m.marshal(s)
	if err != nil {
		return m, err
	}
	if err := s.replaceManaged(ctx, m.Name, rawOld, raw); err != nil {
		return m, err
	}
	if err := s.syncManaged(ctx); err != nil {
		return m, fmt.Errorf("failed to sync storages after update: %w", err)
	}
	return m, nil
}

// replaceManaged replaces the document only if it is unchanged since it was read: concurrent updates fail instead of
// silently losing one.
func (s *credsSvc) replaceManaged(ctx context.Context, name, rawOld string, raw []byte) error {
	err := s.managedClient.Watch(ctx, func(tx *redis.Tx) error {
		cur, err := tx.HGet(ctx, managedStoragesKey, name).Result()
		if err != nil {
			return err
		}
		if cur != rawOld {
			return redis.TxFailedErr
		}
		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.HSet(ctx, managedStoragesKey, name, raw)
			p.Incr(ctx, managedStoragesVersionKey)
			return nil
		})
		return err
	}, managedStoragesKey)
	if errors.Is(err, redis.TxFailedErr) {
		return fmt.Errorf("%w: storage %q was changed concurrently, retry", dom.ErrFailedPrecondition, name)
	}
	if err != nil {
		return fmt.Errorf("failed to update storage in redis: %w", err)
	}
	return nil
}

// RotateConnectionAccount switches the connection account of a managed storage atomically (one Redis transaction that
// also bumps the storages version). The replaced account is kept in the document (sealed) as PreviousAccount until the
// next rotation. Proxies and workers pick the new account up on their next poll of the storages version (at most 2s);
// until then they keep signing with the old one, which therefore must stay valid on the storage for
// RotationPropagationDelay after RotatedAt.
func (s *credsSvc) RotateConnectionAccount(ctx context.Context, name string, next s3.CredentialsV4, forcedBy string) (ManagedStorage, error) {
	if err := s.managedPrecondition(); err != nil {
		return ManagedStorage{}, err
	}
	if _, ok := s.config.Storages[name]; ok {
		return ManagedStorage{}, fmt.Errorf("%w: storage %q is defined in config: change connectionAccount in the config file", dom.ErrFailedPrecondition, name)
	}
	if err := next.Validate(); err != nil {
		return ManagedStorage{}, fmt.Errorf("%w: %w", dom.ErrInvalidArg, err)
	}
	if s.cp != nil {
		var rotated ManagedStorage
		_, err := s.cp.Repos().Storages.Mutate(ctx, name, func(rec *controlplane.StorageRecord) error {
			m, err := unmarshalRecord(s, *rec)
			if err != nil {
				return fmt.Errorf("%w: stored storage %q is corrupted: %w", dom.ErrInternal, name, err)
			}
			if m.ConnectionAccount == next {
				return fmt.Errorf("%w: the new connection account equals the current one", dom.ErrInvalidArg)
			}
			m.PreviousAccount, m.ConnectionAccount = m.ConnectionAccount, next
			m.RotatedAt = time.Now().UTC().Truncate(time.Second)
			m.UpdatedAt = m.RotatedAt
			if forcedBy != "" {
				m.LastForcedSaveBy, m.LastForcedSaveAt = forcedBy, m.RotatedAt
			}
			newRec, err := m.toRecord(s)
			if err != nil {
				return err
			}
			*rec = newRec
			rotated = m
			return nil
		})
		if err != nil {
			return ManagedStorage{}, controlplane.WrapWriteError(err)
		}
		if err := s.syncManaged(ctx); err != nil {
			return rotated, fmt.Errorf("failed to sync storages after rotation: %w", err)
		}
		return rotated, nil
	}
	rawOld, err := s.managedClient.HGet(ctx, managedStoragesKey, name).Result()
	if errors.Is(err, redis.Nil) {
		return ManagedStorage{}, fmt.Errorf("%w: storage %q", dom.ErrNotFound, name)
	}
	if err != nil {
		return ManagedStorage{}, fmt.Errorf("failed to read storage from redis: %w", err)
	}
	m, err := unmarshalManaged(s, name, rawOld)
	if err != nil {
		return m, fmt.Errorf("%w: stored storage %q is corrupted: %w", dom.ErrInternal, name, err)
	}
	if m.ConnectionAccount == next {
		return m, fmt.Errorf("%w: the new connection account equals the current one", dom.ErrInvalidArg)
	}
	m.PreviousAccount, m.ConnectionAccount = m.ConnectionAccount, next
	m.RotatedAt = time.Now().UTC().Truncate(time.Second)
	m.UpdatedAt = m.RotatedAt
	if forcedBy != "" {
		m.LastForcedSaveBy, m.LastForcedSaveAt = forcedBy, m.RotatedAt
	}
	raw, err := m.marshal(s)
	if err != nil {
		return m, err
	}
	if err := s.replaceManaged(ctx, name, rawOld, raw); err != nil {
		return m, err
	}
	if err := s.syncManaged(ctx); err != nil {
		return m, fmt.Errorf("failed to sync storages after rotation: %w", err)
	}
	return m, nil
}

// DeleteManaged removes a managed storage together with the dynamic credentials of its users. Callers must check
// references (replications, routings, ...) before.
func (s *credsSvc) DeleteManaged(ctx context.Context, name string) error {
	if s.managedClient == nil && s.cp == nil {
		return fmt.Errorf("%w: managed storages are not available on this server", dom.ErrFailedPrecondition)
	}
	if _, ok := s.config.Storages[name]; ok {
		return fmt.Errorf("%w: storage %q is defined in config and cannot be deleted through the API", dom.ErrFailedPrecondition, name)
	}
	if s.cp != nil {
		err := s.cp.Tx(ctx, func(r controlplane.Repos) error {
			return DeleteManagedTx(ctx, r, name)
		})
		if err != nil {
			return controlplane.WrapWriteError(err)
		}
		if err := s.sync(ctx); err != nil {
			return fmt.Errorf("failed to sync credentials after storage delete: %w", err)
		}
		if err := s.syncManaged(ctx); err != nil {
			return fmt.Errorf("failed to sync storages after delete: %w", err)
		}
		return nil
	}
	exists, err := s.managedClient.HExists(ctx, managedStoragesKey, name).Result()
	if err != nil {
		return fmt.Errorf("failed to read storage from redis: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: storage %q", dom.ErrNotFound, name)
	}
	// credentials first: a failure leaves a storage without users, and the delete can be retried
	if err := s.deleteAllUserCredentials(ctx, name); err != nil {
		return err
	}
	tx := s.managedClient.TxPipeline()
	tx.HDel(ctx, managedStoragesKey, name)
	tx.HDel(ctx, storageTestsKey, name)
	tx.Incr(ctx, managedStoragesVersionKey)
	if _, err := tx.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete storage in redis: %w", err)
	}
	if err := s.syncManaged(ctx); err != nil {
		return fmt.Errorf("failed to sync storages after delete: %w", err)
	}
	return nil
}

// DeleteManagedTx deletes a managed storage, its user dynamic credentials and its connection test result inside
// an existing control-plane transaction (r).
func DeleteManagedTx(ctx context.Context, r controlplane.Repos, name string) error {
	if _, err := r.Storages.Get(ctx, name); err != nil {
		return err
	}
	allCreds, err := r.DynamicCredentials.List(ctx)
	if err != nil {
		return err
	}
	seenUsers := map[string]bool{}
	for _, c := range allCreds {
		if c.Storage == name && !seenUsers[c.User] {
			seenUsers[c.User] = true
			if err := r.DynamicCredentials.Delete(ctx, name, c.User); err != nil {
				return err
			}
		}
	}
	if err := r.StorageTests.Delete(ctx, name); err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	return r.Storages.Delete(ctx, name)
}

// SyncAfterTx refreshes the in-memory credentials and managed-storage caches after a caller commits a controlplane.Tx.
func (s *credsSvc) SyncAfterTx(ctx context.Context) error {
	if err := s.sync(ctx); err != nil {
		return err
	}
	if err := s.syncManaged(ctx); err != nil {
		return err
	}
	return s.syncIdentities(ctx)
}

func (s *credsSvc) deleteAllUserCredentials(ctx context.Context, storage string) error {
	if !s.config.DynamicCredentials.Enabled {
		return nil
	}
	// HSCAN with a MATCH on the storage prefix instead of HKEYS: the hash holds the credentials of every storage and
	// user, and HKEYS would load and transfer all of its fields in one blocking call. Fields are collected first and
	// deleted afterwards so the scan is not disturbed.
	var fields []string
	match := string(dom.S3) + ":" + globEscape(storage) + ":*"
	var cursor uint64
	for {
		page, next, err := s.client.HScan(ctx, redisHashKey, cursor, match, 500).Result()
		if err != nil {
			return err
		}
		// page holds field, value, field, value...
		for i := 0; i+1 < len(page); i += 2 {
			field := page[i]
			typ, key, err := credsFromRedisField(field)
			if err == nil && typ == dom.S3 {
				if st, _ := cacheKeyToCreds(key); st == storage {
					fields = append(fields, field)
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if len(fields) == 0 {
		return nil
	}
	tx := s.client.TxPipeline()
	tx.HDel(ctx, redisHashKey, fields...)
	tx.HIncrBy(ctx, redisHashKey, redisHFieldVersion, 1)
	if _, err := tx.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete credentials of storage %q in redis: %w", storage, err)
	}
	return s.sync(ctx)
}

// syncManaged reloads the managed storages from PostgreSQL (when configured) or Redis if their version changed.
func (s *credsSvc) syncManaged(ctx context.Context) error {
	if !s.ManagedStoragesEnabled() {
		return nil
	}
	s.managedMu.Lock()
	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() == zerolog.Disabled {
		logger = &zlog.Logger
	}
	if s.cp != nil {
		fp, ferr := s.cp.Repos().Storages.Fingerprint(ctx)
		if ferr == nil && fp != "" && s.managedLoaded && fp == s.managedFP {
			s.managedMu.Unlock()
			return nil // unchanged
		}
		recs, err := s.cp.Repos().Storages.List(ctx)
		if err == nil {
			managed := make(map[string]ManagedStorage, len(recs))
			var verSum int64
			for _, v := range recs {
				verSum += v.Version
				m, err := unmarshalRecord(s, v.Value)
				if err == nil {
					m, err = m.Normalize()
				}
				if err != nil {
					logger.Error().Err(err).Str("storage", v.Value.Name).Msg("ignoring invalid managed storage document")
					continue
				}
				managed[v.Value.Name] = m
			}
			changes := s.applyManagedSnapshot(logger, managed, verSum)
			if ferr == nil {
				s.managedFP = fp
			}
			s.managedMu.Unlock()
			s.notify(changes)
			return nil
		}
		s.managedFP = ""
		if s.managedClient == nil {
			s.managedMu.Unlock()
			return fmt.Errorf("failed to get storages from postgres: %w", err)
		}
		logger.Warn().Err(err).Msg("managed storages: PostgreSQL list failed, falling back to Redis")
	}
	remoteVer, err := s.managedClient.Get(ctx, managedStoragesVersionKey).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		s.managedMu.Unlock()
		return fmt.Errorf("failed to get storages version from redis: %w", err)
	}
	if s.managedLoaded && remoteVer == s.managedVer {
		s.managedMu.Unlock()
		return nil
	}
	docs, err := s.managedClient.HGetAll(ctx, managedStoragesKey).Result()
	if err != nil {
		s.managedMu.Unlock()
		return fmt.Errorf("failed to get storages from redis: %w", err)
	}
	managed := make(map[string]ManagedStorage, len(docs))
	for name, raw := range docs {
		m, err := unmarshalManaged(s, name, raw)
		if err == nil {
			m, err = m.Normalize()
		}
		if err != nil {
			// one broken document must not take the other storages down
			logger.Error().Err(err).Str("storage", name).Msg("ignoring invalid managed storage document")
			continue
		}
		managed[name] = m
	}
	changes := s.applyManagedSnapshot(logger, managed, remoteVer)
	s.managedMu.Unlock()
	s.notify(changes)
	return nil
}

func (s *credsSvc) applyManagedSnapshot(logger *zerolog.Logger, managed map[string]ManagedStorage, ver int64) []StorageChange {
	old := s.view.Load()
	nv := buildView(s.config, managed)
	var changes []StorageChange
	for name, m := range nv.managed {
		if _, inConf := s.config.Storages[name]; inConf {
			logger.Warn().Str("storage", name).Msg("managed storage ignored: a storage with this name is defined in config")
			continue
		}
		prev, ok := old.managed[name]
		switch {
		case !ok:
			changes = append(changes, StorageChange{Name: name, Kind: "added"})
			logger.Info().Str("storage", name).Str("address", m.Address.Address).Msg("managed storage added")
		case prev != m:
			changes = append(changes, StorageChange{Name: name, Kind: "updated"})
			logger.Info().Str("storage", name).Str("address", m.Address.Address).Msg("managed storage updated")
		}
	}
	for name := range old.managed {
		if _, ok := nv.managed[name]; !ok {
			changes = append(changes, StorageChange{Name: name, Kind: "removed"})
			logger.Info().Str("storage", name).Msg("managed storage removed")
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	s.view.Store(nv)
	s.managedVer, s.managedLoaded = ver, true
	return changes
}

// globEscape escapes Redis glob metacharacters in s.
func globEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "*", `\*`, "?", `\?`, "[", `\[`, "]", `\]`).Replace(s)
}
