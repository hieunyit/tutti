/*
 * Copyright © 2023 Clyso GmbH
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

package config

import (
	"embed"
	"fmt"
	"io"
	"os"
	"strings"

	stdlog "github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/trace"
)

//go:embed config.yaml
var configFile embed.FS

type Common struct {
	Log      *log.Config      `yaml:"log,omitempty"`
	Trace    *trace.Config    `yaml:"trace,omitempty"`
	Metrics  *metrics.Config  `yaml:"metrics,omitempty"`
	Redis    *Redis           `yaml:"redis,omitempty"`
	Features *features.Config `yaml:"features,omitempty"`
}

type Redis struct {
	Sentinel RedisSentinel `yaml:"sentinel"`
	// Deprecated: Address is deprecated: use Addresses
	// If Addresses set, Address will be ignored
	Address   string   `yaml:"address"`
	User      string   `yaml:"user"`
	Password  string   `yaml:"password"`
	Addresses []string `yaml:"addresses"`
	MetaDB    int      `yaml:"metaDB"`
	QueueDB   int      `yaml:"queueDB"`
	LockDB    int      `yaml:"lockDB"`
	ConfigDB  int      `yaml:"configDB"`
	TLS       TLS      `yaml:"tls"`

	// Optional per-role overrides: a role with its own address(es) lives on a separate Redis instance
	// (or Sentinel group). Unset fields inherit from the top-level settings. See RedisEndpoint.
	Config RedisEndpoint `yaml:"config"`
	Queue  RedisEndpoint `yaml:"queue"`
	Lock   RedisEndpoint `yaml:"lock"`
	Meta   RedisEndpoint `yaml:"meta"`
}

// RedisRole identifies a group of Redis data that can be placed on its own Redis instance.
type RedisRole string

const (
	// RedisRoleConfig is the control-plane config (policies, routing, storages, keys), logical DB redis.configDB.
	RedisRoleConfig RedisRole = "config"
	// RedisRoleQueue holds the asynq replication queues, logical DB redis.queueDB.
	RedisRoleQueue RedisRole = "queue"
	// RedisRoleLock holds distributed locks, logical DB redis.lockDB.
	RedisRoleLock RedisRole = "lock"
	// RedisRoleMeta holds version vectors, uploads, sessions and migration progress, logical DB redis.metaDB.
	RedisRoleMeta RedisRole = "meta"
)

// RedisEndpoint overrides the Redis connection of one role. The zero value (and the defaults in config.yaml)
// inherit everything from the top-level redis settings, so existing configs behave exactly as before.
type RedisEndpoint struct {
	// Addresses of this role's Redis (or its Sentinel nodes when sentinel.masterName is set).
	// When set, the role uses its own instance and does not inherit the top-level sentinel settings.
	Addresses []string `yaml:"addresses"`
	// User and Password of the role's instance. Empty inherits the top-level values.
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	// Sentinel settings, used only together with Addresses.
	Sentinel RedisSentinel `yaml:"sentinel"`
	// TLS: "" inherits the top-level tls.enabled, "true" or "false" overrides it (tls.insecure is inherited).
	TLS string `yaml:"tls"`
	// DB is the logical DB on this role's instance. Unset (or negative) means: use the role's top-level
	// *DB key (metaDB, queueDB, lockDB, configDB). Set 0 for a dedicated instance. YAML only (no env override).
	DB *int `yaml:"db"`
}

type RedisSentinel struct {
	MasterName string `yaml:"masterName"`
	Password   string `yaml:"password"`
	User       string `yaml:"user"`
}

type TLS struct {
	Enabled  bool `yaml:"enabled"`
	Insecure bool `yaml:"insecure"`
}

func (r *Redis) validate() error {
	if len(r.GetAddresses()) == 0 {
		return fmt.Errorf("invalid redis config: address is not set")
	}
	for role, ep := range map[RedisRole]RedisEndpoint{RedisRoleConfig: r.Config, RedisRoleQueue: r.Queue, RedisRoleLock: r.Lock, RedisRoleMeta: r.Meta} {
		switch strings.ToLower(strings.TrimSpace(ep.TLS)) {
		case "", "true", "false":
		default:
			return fmt.Errorf("invalid redis config: redis.%s.tls must be empty, true or false", role)
		}
		if len(ep.Sentinel.MasterName) != 0 && len(ep.Addresses) == 0 {
			return fmt.Errorf("invalid redis config: redis.%s.sentinel needs redis.%s.addresses", role, role)
		}
	}
	return nil
}

// Endpoint returns the effective connection settings for the role and its logical DB.
// Without overrides it returns the top-level settings and the role's top-level DB number.
func (r *Redis) Endpoint(role RedisRole) (*Redis, int) {
	var ep RedisEndpoint
	var db int
	switch role {
	case RedisRoleConfig:
		ep, db = r.Config, r.ConfigDB
	case RedisRoleQueue:
		ep, db = r.Queue, r.QueueDB
	case RedisRoleLock:
		ep, db = r.Lock, r.LockDB
	case RedisRoleMeta:
		ep, db = r.Meta, r.MetaDB
	}
	eff := *r
	eff.Config, eff.Queue, eff.Lock, eff.Meta = RedisEndpoint{}, RedisEndpoint{}, RedisEndpoint{}, RedisEndpoint{}
	if len(ep.Addresses) != 0 {
		eff.Addresses = ep.Addresses
		eff.Address = ""
		eff.Sentinel = ep.Sentinel
	}
	if ep.User != "" {
		eff.User = ep.User
	}
	if ep.Password != "" {
		eff.Password = ep.Password
	}
	switch strings.ToLower(strings.TrimSpace(ep.TLS)) {
	case "true":
		eff.TLS.Enabled = true
	case "false":
		eff.TLS.Enabled = false
	}
	if ep.DB != nil && *ep.DB >= 0 {
		db = *ep.DB
	}
	return &eff, db
}

func (r *Redis) GetAddresses() []string {
	if len(r.Addresses) != 0 {
		return r.Addresses
	}
	if r.Address != "" {
		return []string{r.Address}
	}
	return nil
}

func Get(conf any, sources ...Opt) error {
	data, err := configFile.Open("config.yaml")
	if err != nil {
		return fmt.Errorf("%w: unable to read config.yaml", err)
	}
	defer data.Close()

	base, err := readYAMLMap(data)
	if err != nil {
		return fmt.Errorf("%w: unable to parse default config.yaml", err)
	}

	stdlog.Info().Msg("app config: reading default common config")

	for _, source := range sources {
		switch src := source.(type) {
		case pathOpt:
			_, err = os.Stat(string(src))
			if err != nil {
				stdlog.Warn().Msgf("app config: no config file %q", string(src))
				continue
			}
			f, err := os.Open(string(src))
			if err != nil {
				return fmt.Errorf("%w: unable to open config file %q", err, string(src))
			}
			overlay, err := readYAMLMap(f)
			f.Close()
			if err != nil {
				return fmt.Errorf("%w: unable to parse config file %q", err, string(src))
			}
			base = deepMerge(base, overlay)
			stdlog.Info().Msgf("app config: override with: %s", string(src))
		case readerOpt:
			overlay, err := readYAMLMap(src.Reader)
			if err != nil {
				return fmt.Errorf("%w: unable to parse config reader %q", err, src.Name)
			}
			base = deepMerge(base, overlay)
			stdlog.Info().Msgf("app config: override with: %s", src.Name)
		}
	}

	if err = applyEnvOverrides(base, "CFG"); err != nil {
		return err
	}

	yamlBytes, err := yaml.Marshal(base)
	if err != nil {
		return fmt.Errorf("%w: unable to marshal merged config", err)
	}
	if err = yaml.Unmarshal(yamlBytes, conf); err != nil {
		return fmt.Errorf("%w: unable to unmarshal config", err)
	}

	return nil
}

func (c *Common) Validate() error {
	if c.Features == nil {
		return fmt.Errorf("app config: empty Features config")
	}
	if c.Log == nil {
		return fmt.Errorf("app config: empty Log config")
	}
	if c.Redis == nil {
		return fmt.Errorf("app config: empty Redis config")
	}
	if err := c.Redis.validate(); err != nil {
		return err
	}
	if c.Metrics == nil {
		return fmt.Errorf("app config: empty Metrics config")
	}
	if c.Trace == nil {
		return fmt.Errorf("app config: empty Trace config")
	}

	return nil
}

type Opt interface {
	configOpt()
}

type pathOpt string

func (pathOpt) configOpt() {}

func Path(path string) Opt {
	return pathOpt(path)
}

type readerOpt struct {
	io.Reader
	Name string
}

func (readerOpt) configOpt() {}

func Reader(reader io.Reader, name string) Opt {
	return readerOpt{reader, name}
}
