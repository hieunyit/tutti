// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package lifecycle_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/lifecycle"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/stretchr/testify/require"
)

type safetyClients struct {
	objstore.Clients
	client s3client.Client
}

func (c safetyClients) AsS3(context.Context, string, string) (s3client.Client, error) {
	return c.client, nil
}

type leaseStore struct {
	*mockLifecycleStore
	muLease     sync.Mutex
	token       string
	expires     time.Time
	rejectRenew bool
}

func (s *leaseStore) AcquireLifecycleLease(_ context.Context, _ int64, token string, d time.Duration, _ bool) (bool, error) {
	s.muLease.Lock()
	defer s.muLease.Unlock()
	if time.Now().Before(s.expires) {
		return false, nil
	}
	s.token = token
	s.expires = time.Now().Add(d)
	return true, nil
}
func (s *leaseStore) RenewLifecycleLease(_ context.Context, _ int64, token string, d time.Duration) (bool, error) {
	s.muLease.Lock()
	defer s.muLease.Unlock()
	if s.rejectRenew || s.token != token || !time.Now().Before(s.expires) {
		return false, nil
	}
	s.expires = time.Now().Add(d)
	return true, nil
}
func (s *leaseStore) CompleteLifecycleLease(ctx context.Context, id int64, token string, next, last time.Time, stats controlplane.LifecycleRunStats) (bool, error) {
	s.muLease.Lock()
	defer s.muLease.Unlock()
	if s.token != token || !time.Now().Before(s.expires) {
		return false, nil
	}
	s.token = ""
	s.expires = time.Time{}
	return true, s.UpdateRuleRun(ctx, id, next, last, stats)
}
func safetySetup(t *testing.T, mode string) (lifecycle.Service, *leaseStore, *[]string) {
	t.Helper()
	var deletes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if _, ok := r.URL.Query()["versioning"]; ok {
			if mode == "unversioned" {
				fmt.Fprint(w, `<VersioningConfiguration/>`)
			} else {
				fmt.Fprint(w, `<VersioningConfiguration><Status>Enabled</Status></VersioningConfiguration>`)
			}
			return
		}
		if r.Method == http.MethodGet {
			versions := `<Version><Key>a</Key><VersionId>old-a</VersionId><IsLatest>true</IsLatest><LastModified>2020-01-01T00:00:00Z</LastModified><Size>42</Size></Version>`
			if mode == "older" {
				versions += `<Version><Key>a</Key><VersionId>older</VersionId><IsLatest>false</IsLatest><LastModified>2019-01-01T00:00:00Z</LastModified><Size>10</Size></Version>`
			}
			if mode == "partial" {
				versions += `<Version><Key>b</Key><VersionId>old-b</VersionId><IsLatest>true</IsLatest><LastModified>2020-01-01T00:00:00Z</LastModified><Size>50</Size></Version>`
			}
			fmt.Fprint(w, `<ListVersionsResult><Name>test-bucket</Name><IsTruncated>false</IsTruncated>`+versions+`</ListVersionsResult>`)
			return
		}
		if r.Method == http.MethodDelete {
			deletes = append(deletes, r.URL.Query().Get("versionId"))
			if mode == "partial" && r.URL.Query().Get("versionId") == "old-b" {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request %s", r.Method)
	}))
	t.Cleanup(server.Close)
	client, err := s3client.NewClient(context.Background(), metrics.NewS3Service(false), s3.StorageAddress{Address: server.URL, DefaultRegion: "us-east-1", HttpTimeout: time.Second}, s3.CredentialsV4{AccessKeyID: "test", SecretAccessKey: "test"}, "storage", "user")
	require.NoError(t, err)
	store := &leaseStore{mockLifecycleStore: newMockLifecycleStore()}
	store.rules[1] = controlplane.LifecycleRule{ID: 1, Enabled: true, StorageName: "storage", Bucket: "test-bucket", ExpirationDays: 1, Schedule: "0 2 * * *"}
	return lifecycle.NewService(store, safetyClients{client: client}, nil), store, &deletes
}
func TestExecutionDeletesObservedVersionAndRecordsPartialFailure(t *testing.T) {
	svc, store, deletes := safetySetup(t, "partial")
	stats, err := svc.ExecuteRule(context.Background(), 1)
	require.Error(t, err)
	require.Equal(t, []string{"old-a", "old-b"}, *deletes)
	require.EqualValues(t, 1, stats.DeletedCount)
	require.Len(t, store.audits, 1)
	require.Equal(t, "a", store.audits[0].ObjectKey)
	require.WithinDuration(t, time.Now().Add(10*time.Minute), *store.rules[1].NextRunAt, time.Second)
}
func TestExecutionFailsClosed(t *testing.T) {
	for _, mode := range []string{"unversioned", "older"} {
		t.Run(mode, func(t *testing.T) {
			svc, _, deletes := safetySetup(t, mode)
			_, err := svc.ExecuteRule(context.Background(), 1)
			require.Error(t, err)
			require.Empty(t, *deletes)
		})
	}
}
func TestLeaseOwnershipAndFencing(t *testing.T) {
	s := &leaseStore{mockLifecycleStore: newMockLifecycleStore()}
	ctx := context.Background()
	ok, err := s.AcquireLifecycleLease(ctx, 1, "first", time.Minute, false)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = s.RenewLifecycleLease(ctx, 1, "first", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = s.AcquireLifecycleLease(ctx, 1, "second", time.Minute, false)
	require.NoError(t, err)
	require.False(t, ok)
	s.expires = time.Now().Add(-time.Second)
	ok, err = s.AcquireLifecycleLease(ctx, 1, "second", time.Minute, false)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = s.CompleteLifecycleLease(ctx, 1, "first", time.Now(), time.Now(), controlplane.LifecycleRunStats{})
	require.NoError(t, err)
	require.False(t, ok)
}
