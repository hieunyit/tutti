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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/config"
	"github.com/hieunyit/tutti/pkg/health"
)

func TestGetConfigDefaults(t *testing.T) {
	r := require.New(t)
	_, err := GetConfig()
	r.NoError(err)
}

// TestHelmEnvOverrides tests env vars from deploy/chorus/templates/proxy/deployment-proxy.yaml
func TestHelmEnvOverrides(t *testing.T) {
	const storagesOverride = `storage:
  main: s3
  storages:
    s3:
      type: S3
      provider: Ceph
      credentials:
        user1:
          accessKeyID: key
          secretAccessKey: secret
      address: s3.example.com
auth:
  useStorage: s3`

	t.Run("CFG_REDIS_PASSWORD", func(t *testing.T) {
		r := require.New(t)
		t.Setenv("CFG_REDIS_PASSWORD", "secret-from-helm")
		conf, err := GetConfig(config.Reader(strings.NewReader(storagesOverride), "test"))
		r.NoError(err)
		r.Equal("secret-from-helm", conf.Redis.Password)
	})

	t.Run("CFG_REDIS_SENTINEL_MASTERNAME", func(t *testing.T) {
		r := require.New(t)
		t.Setenv("CFG_REDIS_SENTINEL_MASTERNAME", "mymaster")
		conf, err := GetConfig(config.Reader(strings.NewReader(storagesOverride), "test"))
		r.NoError(err)
		r.Equal("mymaster", conf.Redis.Sentinel.MasterName)
	})

	t.Run("CFG_STORAGE_DYNAMICCREDENTIALS_MASTERPASSWORD", func(t *testing.T) {
		r := require.New(t)
		t.Setenv("CFG_STORAGE_DYNAMICCREDENTIALS_MASTERPASSWORD", "encryption-key!!")
		conf, err := GetConfig(config.Reader(strings.NewReader(storagesOverride), "test"))
		r.NoError(err)
		r.Equal("encryption-key!!", conf.Storage.DynamicCredentials.MasterPassword)
	})
}

func TestReplicationConfigValidate(t *testing.T) {
	r := require.New(t)
	conf, err := GetConfig()
	r.NoError(err)
	r.Equal("fail", conf.Replication.OnEventError)
	r.Equal(2*time.Second, conf.Replication.EventRetryBudget)

	for _, ok := range []string{"", "fail", "log"} {
		rc := ReplicationConfig{OnEventError: ok}
		r.NoError(rc.Validate(), ok)
	}
	bad := ReplicationConfig{OnEventError: "ignore"}
	r.Error(bad.Validate())
	neg := ReplicationConfig{OnEventError: "fail", EventRetryBudget: -time.Second}
	r.Error(neg.Validate())
}

func TestSyncConfigValidate(t *testing.T) {
	r := require.New(t)
	conf, err := GetConfig()
	r.NoError(err)
	r.False(conf.Replication.Sync.Enabled)
	r.Equal(30*time.Second, conf.Replication.Sync.Timeout)
	r.Equal("fail", conf.Replication.Sync.OnQuorumFailure)
	r.NoError(conf.Replication.Validate())

	r.NoError((&SyncConfig{}).Validate(), "disabled needs no timeout")
	r.NoError((&SyncConfig{Enabled: true, Timeout: time.Second, WriteQuorum: 2, OnQuorumFailure: "accept"}).Validate())
	r.Error((&SyncConfig{Enabled: true}).Validate(), "zero timeout")
	r.Error((&SyncConfig{Enabled: true, Timeout: -time.Second}).Validate())
	r.Error((&SyncConfig{WriteQuorum: -1}).Validate())
	r.Error((&SyncConfig{OnQuorumFailure: "ignore"}).Validate())
	rc := ReplicationConfig{OnEventError: "fail", Sync: SyncConfig{Enabled: true}}
	r.Error(rc.Validate())
}

func TestHealthConfigValidate(t *testing.T) {
	r := require.New(t)
	conf, err := GetConfig()
	r.NoError(err)
	r.Equal(3, conf.Health.FailureThreshold)
	r.Equal(30*time.Second, conf.Health.OpenDuration)
	r.NoError(conf.Health.Validate())
	r.Error((&health.Config{FailureThreshold: -1}).Validate())
	r.Error((&health.Config{OpenDuration: -time.Second}).Validate())
}

func TestFailoverConfigValidate(t *testing.T) {
	r := require.New(t)
	conf, err := GetConfig()
	r.NoError(err)
	r.False(conf.Failover.Enabled)
	r.Equal(60*time.Second, conf.Failover.After)
	r.Equal(0, conf.Failover.MaxEventLag)
	r.Equal(10*time.Second, conf.Failover.CheckInterval)
	r.NoError(conf.Failover.Validate())

	ok := FailoverConfig{Enabled: true, After: time.Minute, CheckInterval: time.Second, MaxEventLag: 5}
	r.NoError(ok.Validate())
	// disabled config is not validated beyond sign checks
	r.NoError((&FailoverConfig{}).Validate())

	for name, bad := range map[string]FailoverConfig{
		"negative lag":        {Enabled: true, After: time.Minute, CheckInterval: time.Second, MaxEventLag: -1},
		"negative after":      {After: -time.Second},
		"negative interval":   {CheckInterval: -time.Second},
		"enabled zero after":  {Enabled: true, CheckInterval: time.Second},
		"enabled zero ticker": {Enabled: true, After: time.Minute},
	} {
		r.Error(bad.Validate(), name)
	}

	full := Config{Failover: FailoverConfig{Enabled: true}}
	r.Error(full.Failover.Validate())
}

func TestStatusConfig_Validate(t *testing.T) {
	r := require.New(t)
	r.NoError((&StatusConfig{Interval: 2 * time.Second}).Validate())
	r.Error((&StatusConfig{}).Validate())
	r.Error((&StatusConfig{Interval: -time.Second}).Validate())
}
