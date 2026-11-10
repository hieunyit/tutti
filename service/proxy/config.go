/*
 * Copyright © 2024 Clyso GmbH
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

package proxy

import (
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/hieunyit/tutti/pkg/config"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/swift"
	"github.com/hieunyit/tutti/service/proxy/auth"
	"github.com/hieunyit/tutti/service/proxy/cors"
	"github.com/hieunyit/tutti/service/proxy/router"
)

//go:embed config.yaml
var configFile embed.FS

func defaultConfig() fs.File {
	defaultFile, err := configFile.Open("config.yaml")
	if err != nil {
		panic(err)
	}
	return defaultFile
}

type Storages = objstore.StoragesConfig[*s3.Storage, *router.SwiftStorage]
type Storage = objstore.GenericStorage[*s3.Storage, *router.SwiftStorage]

type Config struct {
	config.Common `yaml:",inline,omitempty" mapstructure:",squash"`

	Storage Storages     `yaml:"storage,omitempty"`
	Auth    *auth.Config `yaml:"auth,omitempty"`
	Port    int          `yaml:"port"`
	Address string       `yaml:"address"`
	Cors    *cors.Config `yaml:"cors"`

	Replication ReplicationConfig `yaml:"replication"`
	// Health configures per-storage circuit breakers.
	Health health.Config `yaml:"health"`
	// Failover configures optional automatic failover of the routing to a replica when the main storage is down.
	Failover FailoverConfig `yaml:"failover"`
	// Status configures the live status document published to Redis.
	Status StatusConfig `yaml:"status"`
	// Ready configures GET /ready.
	Ready ReadyConfig `yaml:"ready"`
	// DrainDelay is how long the proxy reports not-ready before it stops listening on SIGTERM. Default 5s (embedded config.yaml); 0 disables the wait.
	DrainDelay time.Duration `yaml:"drainDelay"`
	// ReadHeaderTimeout bounds how long a client may take to send request headers (Slowloris protection).
	// It is not applied to the request body or response, so large uploads and downloads are not cut off. Default 10s.
	ReadHeaderTimeout time.Duration `yaml:"readHeaderTimeout"`
	// Snapshot configures the in-memory routing/replication policy snapshot.
	Snapshot SnapshotConfig `yaml:"snapshot"`
	// ProtectionGroups configures the protection group write fence.
	ProtectionGroups ProtectionGroupsConfig `yaml:"protectionGroups"`
}

// ProtectionGroupsConfig configures the protection group write fence of the proxy.
type ProtectionGroupsConfig struct {
	// WriterLease is the TTL of this proxy's writer lease, refreshed every WriterLease/6 while the proxy runs.
	// After a crash, promotion of a frozen group waits at most this long (plus the status report expiry) for
	// the dead proxy's in-flight writes. A proxy that cannot refresh it for WriterLease/2 cancels its in-flight
	// protected writes. Default 30s.
	WriterLease time.Duration `yaml:"writerLease"`
}

// ReadyConfig configures the readiness endpoint GET /ready.
type ReadyConfig struct {
	// MaxStaleness is how long the proxy stays ready, serving reads from its last known config snapshot,
	// while Redis is unreachable. Default 30s.
	MaxStaleness time.Duration `yaml:"maxStaleness"`
}

// SnapshotConfig configures the in-memory policy snapshot (routing, switches, replication policies, generations).
type SnapshotConfig struct {
	// Disabled turns the snapshot off: every request reads its policy data from Redis (old behavior).
	Disabled bool `yaml:"disabled"`
	// PollInterval is how often the config version is read when no change notification arrives. Default 1s.
	PollInterval time.Duration `yaml:"pollInterval"`
	// FullReload drops all cached entries at least this often. Default 30s.
	FullReload time.Duration `yaml:"fullReload"`
}

func (c *Config) validateReady() error {
	if c.Ready.MaxStaleness < 0 || c.DrainDelay < 0 || c.ReadHeaderTimeout < 0 {
		return fmt.Errorf("proxy config: ready.maxStaleness, drainDelay and readHeaderTimeout must not be negative")
	}
	if c.Snapshot.PollInterval < 0 || c.Snapshot.FullReload < 0 {
		return fmt.Errorf("proxy config: snapshot.pollInterval and fullReload must not be negative")
	}
	if c.ProtectionGroups.WriterLease < 0 || (c.ProtectionGroups.WriterLease > 0 && c.ProtectionGroups.WriterLease < time.Second) {
		return fmt.Errorf("proxy config: protectionGroups.writerLease must be 0 (default) or at least 1s")
	}
	return nil
}

// StatusConfig configures publishing of the proxy status document (tutti:proxy-status:<app_id>).
type StatusConfig struct {
	// Interval is how often the status (health, settings, stats, rates) is published. The Redis TTL is 3x. Default 2s.
	Interval time.Duration `yaml:"interval"`
}

func (s *StatusConfig) Validate() error {
	if s.Interval <= 0 {
		return fmt.Errorf("proxy config: status.interval must be greater than 0")
	}
	return nil
}

// FailoverConfig configures the automatic failover controller. Disabled by default.
type FailoverConfig struct {
	Enabled bool `yaml:"enabled"`
	// After is the default for policies that omit `after`: how long the source breaker must be continuously open.
	After time.Duration `yaml:"after"`
	// MaxEventLag is the default for policies that omit `maxEventLag`.
	MaxEventLag int `yaml:"maxEventLag"`
	// CheckInterval is how often the controller evaluates the conditions.
	CheckInterval time.Duration `yaml:"checkInterval"`
}

func (f *FailoverConfig) Validate() error {
	if f.MaxEventLag < 0 {
		return fmt.Errorf("proxy config: failover.maxEventLag must not be negative")
	}
	if f.After < 0 || f.CheckInterval < 0 {
		return fmt.Errorf("proxy config: failover.after and failover.checkInterval must not be negative")
	}
	if f.Enabled {
		if f.After <= 0 {
			return fmt.Errorf("proxy config: failover.after must be greater than 0 when failover is enabled")
		}
		if f.CheckInterval <= 0 {
			return fmt.Errorf("proxy config: failover.checkInterval must be greater than 0 when failover is enabled")
		}
	}
	return nil
}

type ReplicationConfig struct {
	// OnEventError defines what to do when a write succeeded on the main storage but its replication event
	// could not be recorded: "fail" (default) returns 503 so the client retries the (idempotent) request,
	// which records the event again; "log" keeps the legacy behavior (respond with the storage response and only log).
	OnEventError string `yaml:"onEventError"`
	// EventRetryBudget is the total time to retry recording the event before OnEventError applies. Default 2s.
	EventRetryBudget time.Duration `yaml:"eventRetryBudget"`
	// Sync enables synchronous replication on the write path.
	Sync SyncConfig `yaml:"sync"`
}

// SyncConfig configures synchronous replication: after the main write succeeded the proxy copies the
// object to the replication destinations before answering the client. The replication task stays
// enqueued as a safety net, so destinations that fail or time out are completed by the worker.
type SyncConfig struct {
	Enabled bool `yaml:"enabled"`
	// WriteQuorum is the number of copies (main included) that must exist before answering.
	// 0 means main + all destinations.
	WriteQuorum int `yaml:"writeQuorum"`
	// Timeout is the max time spent syncing destinations per request.
	Timeout time.Duration `yaml:"timeout"`
	// OnQuorumFailure: "fail" responds 503, "accept" forwards the main storage response.
	OnQuorumFailure string `yaml:"onQuorumFailure"`
}

func (c *SyncConfig) Validate() error {
	if c.WriteQuorum < 0 {
		return fmt.Errorf("proxy config: replication.sync.writeQuorum must not be negative")
	}
	switch c.OnQuorumFailure {
	case "", router.OnQuorumFailureFail, router.OnQuorumFailureAccept:
	default:
		return fmt.Errorf("proxy config: invalid replication.sync.onQuorumFailure %q: must be %q or %q", c.OnQuorumFailure, router.OnQuorumFailureFail, router.OnQuorumFailureAccept)
	}
	if c.Enabled && c.Timeout <= 0 {
		return fmt.Errorf("proxy config: replication.sync.timeout must be greater than 0 when sync is enabled")
	}
	return nil
}

func (r *ReplicationConfig) Validate() error {
	switch r.OnEventError {
	case "", router.OnEventErrorFail, router.OnEventErrorLog:
	default:
		return fmt.Errorf("proxy config: invalid replication.onEventError %q: must be %q or %q", r.OnEventError, router.OnEventErrorFail, router.OnEventErrorLog)
	}
	if r.EventRetryBudget < 0 {
		return fmt.Errorf("proxy config: replication.eventRetryBudget must not be negative")
	}
	return r.Sync.Validate()
}

func (c *Config) Validate() error {
	if err := c.Common.Validate(); err != nil {
		return err
	}
	if err := c.Storage.Validate(); err != nil {
		return err
	}
	if err := ValidateAuth(c.Storage, c.Auth); err != nil {
		return err
	}
	if err := c.Replication.Validate(); err != nil {
		return err
	}
	if err := c.Health.Validate(); err != nil {
		return err
	}
	if err := c.Failover.Validate(); err != nil {
		return err
	}
	if err := c.Status.Validate(); err != nil {
		return err
	}
	return c.validateReady()
}

func ValidateAuth(storage Storages, auth *auth.Config) error {
	if len(storage.S3Storages()) == 0 {
		return nil
	}
	// S3 storages set: validate auth config
	if auth == nil {
		return fmt.Errorf("proxy config: empty Auth config")
	}
	if err := auth.ValidateDefaultPolicy(); err != nil {
		return err
	}
	if auth.UseStorage != "" {
		if _, ok := storage.Storages[auth.UseStorage]; !ok {
			return fmt.Errorf("proxy config: auth UseStorage points to unknown storage")
		}
	}
	if len(auth.Custom) != 0 {
		for user := range auth.Custom {
			if err := storage.Exists(storage.Main, user); err != nil {
				return fmt.Errorf("proxy config: auth custom credentials unknown user %q", user)
			}
		}
	}
	if auth.UseStorage == "" && len(auth.Custom) == 0 {
		return fmt.Errorf("proxy config: auth credentials enabled but not set")
	}
	return nil
}

func GetConfig(src ...config.Opt) (*Config, error) {
	dc := defaultConfig()
	var conf Config
	cfgSource := make([]config.Opt, 0, 1+len(src))
	cfgSource = append(cfgSource, config.Reader(dc, "proxy_default_cfg"))
	cfgSource = append(cfgSource, src...)
	err := config.Get(&conf, cfgSource...)
	_ = dc.Close()
	if err != nil {
		return nil, err
	}
	return &conf, err
}

func ProxyToCredsConf(in Storages) (objstore.Config, error) {
	res := objstore.Config{
		Storages:           map[string]objstore.GenericStorage[*s3.Storage, *swift.Storage]{},
		Main:               in.Main,
		DynamicCredentials: in.DynamicCredentials,
	}

	for name, val := range in.Storages {
		switch val.Type {
		case dom.S3:
			c := *val.S3
			res.Storages[name] = objstore.Storage{
				S3:           &c,
				CommonConfig: val.CommonConfig,
			}
		case dom.Swift:
		// ignore - swift proxy conf does not contains credentials
		default:
			return objstore.Config{}, fmt.Errorf("unsupported storage type %q for storage %q", val.Type, name)
		}
	}
	return res, nil
}
