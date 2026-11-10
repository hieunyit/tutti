// Copyright 2025 Clyso GmbH
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

package util

import (
	"crypto/tls"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/config"
)

func NewRedis(conf *config.Redis, db int) redis.UniversalClient {
	opt := &redis.UniversalOptions{
		Addrs:            conf.GetAddresses(),
		DB:               db,
		Username:         conf.User,
		Password:         conf.Password,
		SentinelUsername: conf.Sentinel.User,
		SentinelPassword: conf.Sentinel.Password,
		MasterName:       conf.Sentinel.MasterName,
	}
	if conf.TLS.Enabled {
		opt.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: conf.TLS.Insecure,
		}
	}
	return redis.NewUniversalClient(opt)
}

// NewRedisRole creates a client for the role (config, queue, lock, meta): its own instance when
// redis.<role>.addresses is set, otherwise the top-level instance and the role's logical DB.
func NewRedisRole(conf *config.Redis, role config.RedisRole) redis.UniversalClient {
	eff, db := conf.Endpoint(role)
	return NewRedis(eff, db)
}

// NewRedisAsynqRole is NewRedisRole for the asynq queue client (standalone, Sentinel or cluster).
func NewRedisAsynqRole(conf *config.Redis, role config.RedisRole) asynq.RedisConnOpt {
	eff, db := conf.Endpoint(role)
	return NewRedisAsynq(eff, db)
}

func NewRedisAsynq(conf *config.Redis, db int) asynq.RedisConnOpt {
	addresses := conf.GetAddresses()
	if conf.Sentinel.MasterName != "" {
		// Sentinel (checked first: a single sentinel address is valid and must not fall back to standalone)
		opt := asynq.RedisFailoverClientOpt{
			MasterName:       conf.Sentinel.MasterName,
			SentinelAddrs:    addresses,
			SentinelUsername: conf.Sentinel.User,
			SentinelPassword: conf.Sentinel.Password,
			Username:         conf.User,
			Password:         conf.Password,
			DB:               db,
		}
		if conf.TLS.Enabled {
			opt.TLSConfig = &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: conf.TLS.Insecure,
			}
		}
		return &opt
	}
	if len(addresses) == 1 {
		// Standalone
		opt := asynq.RedisClientOpt{Addr: addresses[0], Username: conf.User, Password: conf.Password, DB: db}
		if conf.TLS.Enabled {
			opt.TLSConfig = &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: conf.TLS.Insecure,
			}
		}
		return &opt
	}

	// Cluster
	opt := asynq.RedisClusterClientOpt{
		Addrs:    addresses,
		Username: conf.User,
		Password: conf.Password,
	}
	if conf.TLS.Enabled {
		opt.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: conf.TLS.Insecure,
		}
	}
	return &opt
}
