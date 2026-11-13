// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/testutil"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/stretchr/testify/require"
)

// scaleHandlers builds nStorages S3 storages, each with nStatic config users and nDynamic dynamic users.
func scaleHandlers(tb testing.TB, nStorages, nStatic, nDynamic int) (*resilienceHandlers, []string) {
	tb.Helper()
	rc := testutil.SetupRedis(tb)
	storages := map[string]objstore.Storage{}
	var names []string
	for i := 0; i < nStorages; i++ {
		name := fmt.Sprintf("st%03d", i)
		names = append(names, name)
		users := map[string]s3.CredentialsV4{}
		for u := 0; u < nStatic; u++ {
			users[fmt.Sprintf("cfg%d", u)] = s3.CredentialsV4{AccessKeyID: "ak", SecretAccessKey: "sk"}
		}
		storages[name] = objstore.Storage{CommonConfig: objstore.CommonConfig{Type: dom.S3}, S3: &s3.Storage{Credentials: users, StorageAddress: s3.StorageAddress{Address: "http://127.0.0.1:1", Provider: s3.ProviderMinIO}}}
	}
	cfg := &objstore.Config{Main: names[0], Storages: storages, DynamicCredentials: objstore.DynamicCredentialsConfig{Enabled: true, MasterPassword: "runtime-test-password", PollInterval: time.Hour}}
	creds, err := objstore.NewCredsSvc(tb.Context(), cfg, rc, objstore.WithManagedStorages(rc))
	require.NoError(tb, err)
	for _, n := range names {
		for u := 0; u < nDynamic; u++ {
			require.NoError(tb, creds.SetS3Credentials(tb.Context(), n, fmt.Sprintf("dyn%d", u), s3.CredentialsV4{AccessKeyID: "ak", SecretAccessKey: "sk"}))
		}
	}
	return ResilienceHandlers(rc, nil, WithRuntimeStorages(creds, nil)).(*resilienceHandlers), names
}

func TestListRuntimeStoragesUserCount(t *testing.T) {
	h, names := scaleHandlers(t, 4, 3, 5)
	ptr := func(b bool) *bool { return &b }
	for name, req := range map[string]*pb.ListRuntimeStoragesRequest{"nil": nil, "unset": {}, "true": {IncludeUsers: ptr(true)}} {
		l, err := h.ListRuntimeStorages(t.Context(), req)
		require.NoError(t, err, name)
		require.Len(t, l.Storages, len(names), name)
		for _, s := range l.Storages {
			require.Len(t, s.Users, 8, name)
			require.EqualValues(t, 8, s.UserCount, name)
			ids := make([]string, 0, len(s.Users))
			for _, u := range s.Users {
				ids = append(ids, u.User)
			}
			require.True(t, sort.StringsAreSorted(ids), name)
		}
	}
	l, err := h.ListRuntimeStorages(t.Context(), &pb.ListRuntimeStoragesRequest{IncludeUsers: ptr(false)})
	require.NoError(t, err)
	require.Len(t, l.Storages, len(names))
	for _, s := range l.Storages {
		require.Empty(t, s.Users)
		require.EqualValues(t, 8, s.UserCount)
	}
	// single get keeps today's shape and also fills the count
	one, err := h.runtimeStoragePb(context.Background(), names[1])
	require.NoError(t, err)
	require.Len(t, one.Users, 8)
	require.EqualValues(t, 8, one.UserCount)
}

// BenchmarkListRuntimeStorages100x1000: "before" is the per-storage path the list used (ListOverrideUsers scans the
// whole credential cache once per storage); "after" is the single-pass list.
func BenchmarkListRuntimeStorages100x1000(b *testing.B) {
	h, names := scaleHandlers(b, 100, 0, 1000)
	ctx := b.Context()
	tests, err := h.runtimeStorages.StorageTests(ctx)
	require.NoError(b, err)
	ptr := func(v bool) *bool { return &v }
	b.Run("before_per_storage_users", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, n := range names {
				if _, err := h.runtimeStoragePbWithTests(ctx, n, tests); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("after_includeUsers_true", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := h.ListRuntimeStorages(ctx, &pb.ListRuntimeStoragesRequest{}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("after_includeUsers_false", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := h.ListRuntimeStorages(ctx, &pb.ListRuntimeStoragesRequest{IncludeUsers: ptr(false)}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
