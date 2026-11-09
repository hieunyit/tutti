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
}

type ReplicationConfig struct {
	// OnEventError defines what to do when a write succeeded on the main storage but its replication event
	// could not be recorded: "fail" (default) returns 503 so the client retries the (idempotent) request,
	// which records the event again; "log" keeps the legacy behavior (respond with the storage response and only log).
	OnEventError string `yaml:"onEventError"`
	// EventRetryBudget is the total time to retry recording the event before OnEventError applies. Default 2s.
	EventRetryBudget time.Duration `yaml:"eventRetryBudget"`
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
	return nil
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
	return nil
}

func ValidateAuth(storage Storages, auth *auth.Config) error {
	if len(storage.S3Storages()) == 0 {
		return nil
	}
	// S3 storages set: validate auth config
	if auth == nil {
		return fmt.Errorf("proxy config: empty Auth config")
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
