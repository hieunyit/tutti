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

package objstore

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3"
)

// manyStoragesSvc: nStorages S3 storages with a connection account, nUsers users with dynamic credentials on every
// storage and as identities (nStorages x nUsers cache entries), built without Redis.
func manyStoragesSvc(nStorages, nUsers int) *credsSvc {
	conf := &Config{Main: "st000", Storages: map[string]Storage{}, DynamicCredentials: DynamicCredentialsConfig{Enabled: true}}
	conn := s3.CredentialsV4{AccessKeyID: "c", SecretAccessKey: "s"}
	for i := 0; i < nStorages; i++ {
		conf.Storages[fmt.Sprintf("st%03d", i)] = Storage{CommonConfig: CommonConfig{Type: dom.S3}, S3: &s3.Storage{
			StorageAddress: s3.StorageAddress{Address: "http://localhost:9000", Provider: s3.ProviderMinIO}, ConnectionAccount: &conn}}
	}
	svc := &credsSvc{config: conf, s3Creds: map[string]s3.CredentialsV4{}, swiftCreds: nil}
	svc.view.Store(buildView(conf, nil))
	ids := map[string]struct{}{}
	for u := 0; u < nUsers; u++ {
		name := fmt.Sprintf("user%03d", u)
		ids[name] = struct{}{}
		for i := 0; i < nStorages; i++ {
			svc.s3Creds[fmt.Sprintf("st%03d:%s", i, name)] = s3.CredentialsV4{AccessKeyID: "a", SecretAccessKey: "b"}
		}
	}
	svc.identities.Store(&ids)
	return svc
}

func TestStoragesWithOverrideUserMatchesListOverrideUsers(t *testing.T) {
	svc := manyStoragesSvc(5, 4)
	svc.s3Creds["st001:solo"] = s3.CredentialsV4{}
	for _, u := range []string{"user002", "solo", "nobody"} {
		var want []string
		for st := range svc.config.Storages {
			for _, x := range svc.ListOverrideUsers(st) {
				if x == u {
					want = append(want, st)
				}
			}
		}
		got := svc.StoragesWithOverrideUser(u)
		var gotL []string
		for st := range got {
			gotL = append(gotL, st)
		}
		sort.Strings(want)
		sort.Strings(gotL)
		require.Equal(t, want, gotL, u)
	}
}

// BenchmarkDuplicateCheck: the user-create duplicate check with 100 storages and 200 existing users, the old way
// (ListUsers per storage) against StoragesWithOverrideUser plus one identity lookup.
func BenchmarkDuplicateCheck(b *testing.B) {
	svc := manyStoragesSvc(100, 200)
	names := make([]string, 0, 100)
	for st := range svc.config.Storages {
		names = append(names, st)
	}
	sort.Strings(names)
	b.Run("per-storage-ListUsers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, st := range names {
				for _, u := range svc.ListUsers(st) {
					if u == "fresh" {
						b.Fatal("unexpected")
					}
				}
			}
		}
	})
	b.Run("single-pass", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			found := svc.StoragesWithOverrideUser("fresh")
			isID := svc.HasIdentity("fresh")
			for _, st := range names {
				if _, ok := found[st]; ok || isID {
					b.Fatal("unexpected")
				}
			}
		}
	})
}

// BenchmarkListUnion: the union of users built by Manager.List, per storage (ListUsers) against one pass
// (AllOverrideUsers) plus Identities (identities are added anyway, so the unions are equal).
func BenchmarkListUnion(b *testing.B) {
	svc := manyStoragesSvc(100, 200)
	names := make([]string, 0, 100)
	for st := range svc.config.Storages {
		names = append(names, st)
	}
	sort.Strings(names)
	b.Run("per-storage-ListUsers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			set := map[string]struct{}{}
			for _, st := range names {
				for _, u := range svc.ListUsers(st) {
					set[u] = struct{}{}
				}
			}
			for _, u := range svc.Identities() {
				set[u] = struct{}{}
			}
		}
	})
	b.Run("single-pass", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			set := svc.AllOverrideUsers()
			for _, u := range svc.Identities() {
				set[u] = struct{}{}
			}
		}
	})
}

func TestAllOverrideUsersUnion(t *testing.T) {
	svc := manyStoragesSvc(4, 3)
	svc.s3Creds["st002:solo"] = s3.CredentialsV4{}
	svc.s3Creds["gone:ghost"] = s3.CredentialsV4{} // storage not in the view is ignored, like ListOverrideUsers
	want := map[string]struct{}{}
	for st := range svc.config.Storages {
		for _, u := range svc.ListOverrideUsers(st) {
			want[u] = struct{}{}
		}
	}
	require.Equal(t, want, svc.AllOverrideUsers())
	require.Contains(t, want, "solo")
	require.NotContains(t, want, "ghost")
}
