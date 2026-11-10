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
	"context"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/config"
)

func TestNewRedisAsynq_Topologies(t *testing.T) {
	r := require.New(t)

	standalone := NewRedisAsynq(&config.Redis{Addresses: []string{"redis:6379"}, Password: "p"}, 1)
	opt, ok := standalone.(*asynq.RedisClientOpt)
	r.True(ok)
	r.Equal("redis:6379", opt.Addr)
	r.Equal(1, opt.DB)

	// a single Sentinel address is valid: it must not fall back to standalone
	one := NewRedisAsynq(&config.Redis{
		Addresses: []string{"sentinel:26379"},
		Sentinel:  config.RedisSentinel{MasterName: "tutti", Password: "sp", User: "su"},
		Password:  "p",
	}, 2)
	fo, ok := one.(*asynq.RedisFailoverClientOpt)
	r.True(ok, "single sentinel address must use the failover client, got %T", one)
	r.Equal("tutti", fo.MasterName)
	r.Equal([]string{"sentinel:26379"}, fo.SentinelAddrs)
	r.Equal("sp", fo.SentinelPassword)
	r.Equal("su", fo.SentinelUsername)
	r.Equal("p", fo.Password)
	r.Equal(2, fo.DB)

	three := NewRedisAsynq(&config.Redis{Addresses: []string{"a:1", "b:1", "c:1"}, Sentinel: config.RedisSentinel{MasterName: "m"}}, 0)
	_, ok = three.(*asynq.RedisFailoverClientOpt)
	r.True(ok)
}

func TestNewRedisRole_Overrides(t *testing.T) {
	r := require.New(t)
	zero := 0
	conf := &config.Redis{
		Addresses: []string{"main-redis:6379"}, Password: "base", MetaDB: 0, QueueDB: 1, LockDB: 2, ConfigDB: 3,
		Queue: config.RedisEndpoint{Addresses: []string{"queue-redis:6379"}, DB: &zero},
		Lock:  config.RedisEndpoint{Addresses: []string{"s1:26379", "s2:26379"}, Sentinel: config.RedisSentinel{MasterName: "locks"}, Password: "lockpw"},
	}

	// no override: same instance, role DB
	c, ok := NewRedisRole(conf, config.RedisRoleConfig).(*redis.Client)
	r.True(ok)
	r.Equal("main-redis:6379", c.Options().Addr)
	r.Equal(3, c.Options().DB)
	r.Equal("base", c.Options().Password)

	// own instance with DB 0 and inherited password
	q := NewRedisAsynqRole(conf, config.RedisRoleQueue)
	qo, ok := q.(*asynq.RedisClientOpt)
	r.True(ok)
	r.Equal("queue-redis:6379", qo.Addr)
	r.Equal(0, qo.DB)
	r.Equal("base", qo.Password)

	// own Sentinel group, own password, role's top-level DB
	eff, db := conf.Endpoint(config.RedisRoleLock)
	r.Equal([]string{"s1:26379", "s2:26379"}, eff.GetAddresses())
	r.Equal("locks", eff.Sentinel.MasterName)
	r.Equal("lockpw", eff.Password)
	r.Equal(2, db)

	// the top-level sentinel settings are not inherited by a role that has its own addresses
	conf.Sentinel.MasterName = "base-master"
	eff, _ = conf.Endpoint(config.RedisRoleQueue)
	r.Empty(eff.Sentinel.MasterName)
	eff, _ = conf.Endpoint(config.RedisRoleMeta)
	r.Equal("base-master", eff.Sentinel.MasterName)
}

// TestLiveSentinel runs against a real Sentinel when TUTTI_TEST_SENTINEL_ADDR (host:port of ONE sentinel) and
// TUTTI_TEST_SENTINEL_MASTER are set, e.g. redis-server --port 26390 and a sentinel on 26391 monitoring it.
func TestLiveSentinel(t *testing.T) {
	addr, master := os.Getenv("TUTTI_TEST_SENTINEL_ADDR"), os.Getenv("TUTTI_TEST_SENTINEL_MASTER")
	if addr == "" || master == "" {
		t.Skip("TUTTI_TEST_SENTINEL_ADDR / TUTTI_TEST_SENTINEL_MASTER not set")
	}
	r := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conf := &config.Redis{Addresses: []string{addr}, Sentinel: config.RedisSentinel{MasterName: master}, ConfigDB: 3, QueueDB: 1}

	rc := NewRedisRole(conf, config.RedisRoleConfig)
	defer rc.Close()
	r.NoError(rc.Set(ctx, "tutti-test-sentinel", "ok", time.Minute).Err())

	// the queue client with a SINGLE sentinel address must talk to the master through Sentinel
	opt := NewRedisAsynqRole(conf, config.RedisRoleQueue)
	_, isFailover := opt.(*asynq.RedisFailoverClientOpt)
	r.True(isFailover)
	cl := asynq.NewClient(opt)
	defer cl.Close()
	info, err := cl.EnqueueContext(ctx, asynq.NewTask("t", nil), asynq.Queue("sentinel-test"))
	r.NoError(err)
	r.NotEmpty(info.ID)
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	err = insp.DeleteTask("sentinel-test", info.ID)
	r.NoError(err)
}
