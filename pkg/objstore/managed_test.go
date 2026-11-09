// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package objstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func managedTestConf() *Config {
	return &Config{Main: "main", Storages: map[string]Storage{"main": {CommonConfig: CommonConfig{Type: dom.S3}, S3: &s3.Storage{StorageAddress: s3.StorageAddress{Address: "http://localhost:9000", Provider: s3.ProviderMinIO}, Credentials: map[string]s3.CredentialsV4{"u": {AccessKeyID: "key", SecretAccessKey: "secret"}}}}}, DynamicCredentials: DynamicCredentialsConfig{Enabled: true, MasterPassword: "managed-test-password", PollInterval: time.Hour}}
}
func TestManagedStoragesPropagationAndProtection(t *testing.T) {
	rc := testutil.SetupRedis(t)
	confRC := redis.NewClient(&redis.Options{Addr: rc.(*redis.Client).Options().Addr, DB: 1})
	defer confRC.Close()
	conf := managedTestConf()
	a, err := NewCredsSvc(t.Context(), conf, rc, WithManagedStorages(confRC))
	require.NoError(t, err)
	b, err := NewCredsSvc(t.Context(), conf, rc, WithManagedStorages(confRC))
	require.NoError(t, err)
	m := ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: "http://localhost:9001", Provider: s3.ProviderMinIO}}
	_, err = a.CreateManaged(t.Context(), m)
	require.NoError(t, err)
	require.NoError(t, a.SetS3Credentials(t.Context(), "archive", "u", s3.CredentialsV4{AccessKeyID: "privatekey", SecretAccessKey: "privatesecret"}))
	require.Eventually(t, func() bool {
		_, e := b.GetS3Address("archive")
		_, ce := b.GetS3Credentials("archive", "u")
		return e == nil && ce == nil
	}, 5*time.Second, 20*time.Millisecond)
	raw, err := rc.HGetAll(t.Context(), redisHashKey).Result()
	require.NoError(t, err)
	sealed, exists := raw[credsToRedisField(dom.S3, "archive", "u")]
	require.True(t, exists)
	require.NotEmpty(t, sealed)
	require.NotContains(t, sealed, "privatesecret")
	_, err = a.CreateManaged(t.Context(), m)
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
	require.ErrorIs(t, a.DeleteManaged(t.Context(), "main"), dom.ErrFailedPrecondition)
	m.Address.Address = "http://localhost:9002"
	_, err = a.UpdateManaged(t.Context(), m)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		addr, e := b.GetS3Address("archive")
		return e == nil && strings.Contains(addr.Address, "9002")
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, a.DeleteManaged(t.Context(), "archive"))
	require.Eventually(t, func() bool {
		_, e := b.GetS3Credentials("archive", "u")
		_, ae := b.GetS3Address("archive")
		return e != nil && ae != nil
	}, 5*time.Second, 20*time.Millisecond)
}
func TestManagedStorageEndpointValidation(t *testing.T) {
	for _, address := range []string{"http://user:secret@host:9000", "http:///", "http://host/bucket", "http://host?secret=x", "http://host/#secret"} {
		_, err := (ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: address, Provider: s3.ProviderMinIO}}).Normalize()
		require.Error(t, err, address)
	}
}
func TestManagedAddressSwapKeepsInflightRequest(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bucket/old" {
			close(entered)
			<-release
			_, _ = w.Write([]byte("endpointA"))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bucket/new" {
			_, _ = w.Write([]byte("endpointB"))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer b.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	rc := testutil.SetupRedis(t)
	c, err := NewCredsSvc(t.Context(), managedTestConf(), rc, WithManagedStorages(rc))
	require.NoError(t, err)
	m := ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: a.URL, Provider: s3.ProviderMinIO, DefaultRegion: "us-east-1"}}
	_, err = c.CreateManaged(t.Context(), m)
	require.NoError(t, err)
	require.NoError(t, c.SetS3Credentials(t.Context(), "archive", "u", s3.CredentialsV4{AccessKeyID: "key", SecretAccessKey: "secret"}))
	// Construct a lazy registry to avoid probing the unrelated config storage.
	reg := &clients{metricsSvc: metrics.NewS3Service(false), credsSvc: c, s3Clients: map[string]s3CacheVal{}}
	old, err := reg.AsS3(t.Context(), "archive", "u")
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://proxy/bucket/old", nil)
		resp, _, e := old.Do(r)
		if resp != nil {
			resp.Body.Close()
		}
		result <- e
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("old request never reached endpoint A")
	}
	m.Address.Address = b.URL
	_, err = c.UpdateManaged(t.Context(), m)
	require.NoError(t, err)
	newer, err := reg.AsS3(t.Context(), "archive", "u")
	require.NoError(t, err)
	require.NotSame(t, old, newer)
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://proxy/bucket/new", nil)
	resp, _, err := newer.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	close(release)
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("old request failed to finish")
	}
}

func TestManagedStorageCapabilityRequiresEncryption(t *testing.T) {
	rc := testutil.SetupRedis(t)
	conf := managedTestConf()
	conf.DynamicCredentials.DisableEncryption = true
	svc, err := NewCredsSvc(t.Context(), conf, rc, WithManagedStorages(rc))
	require.NoError(t, err)
	require.False(t, svc.ManagedStoragesEnabled())
	_, err = svc.CreateManaged(t.Context(), ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: "http://localhost:9001", Provider: s3.ProviderMinIO}})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
}
