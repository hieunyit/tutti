// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

// Package migrate copies the control-plane data from the Redis stores to PostgreSQL and verifies that both agree.
// The Redis reader parses the documented key layouts of the existing stores; tests populate Redis through the real
// stores to keep the two in sync.
package migrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

// Family names the object families of a migration, in dependency order.
type Family string

const (
	FamStorages         Family = "storages"
	FamS3Identities     Family = "s3_identities"
	FamDynamicCreds     Family = "dynamic_credentials"
	FamDynamicSalt      Family = "dynamic_credentials_salt"
	FamProxyAccounts    Family = "proxy_accounts"
	FamAccessPolicies   Family = "access_policies"
	FamConsoleUsers     Family = "console_users"
	FamConsoleLogins    Family = "console_user_logins"
	FamRoutes           Family = "routes"
	FamRouteBlocks      Family = "route_blocks"
	FamReplPolicies     Family = "replication_policies"
	FamReplStatus       Family = "replication_status"
	FamReplGenerations  Family = "replication_generations"
	FamFailoverPolicies Family = "failover_policies"
	FamFailoverEvents   Family = "failover_events"
	FamProtectionGroups Family = "protection_groups"
	FamStorageTests     Family = "storage_tests"
)

// Families in the order they are imported.
var Families = []Family{
	FamStorages, FamS3Identities, FamDynamicSalt, FamDynamicCreds, FamProxyAccounts, FamAccessPolicies, FamConsoleUsers, FamConsoleLogins,
	FamRoutes, FamRouteBlocks, FamReplStatus, FamReplPolicies, FamReplGenerations, FamFailoverPolicies, FamFailoverEvents, FamProtectionGroups, FamStorageTests,
}

// Data is the whole control-plane state in typed form.
type Data struct {
	Storages         []controlplane.StorageRecord
	S3Identities     []controlplane.S3Identity
	DynamicCreds     []controlplane.DynamicCredential
	DynamicSalt      string
	ProxyAccounts    []controlplane.ProxyAccountRecord
	AccessPolicies   []controlplane.AccessPolicy
	ConsoleUsers     []controlplane.ConsoleUser // LastLoginAt empty, see ConsoleLogins
	ConsoleLogins    map[string]time.Time
	Routing          controlplane.RoutingExport
	ReplPolicies     []controlplane.ReplicationID
	ReplStatuses     map[controlplane.ReplicationID]controlplane.ReplicationStatus
	ReplGenerations  map[controlplane.ReplicationID]int64 // FromStorage is not part of a generation key
	FailoverPolicies []controlplane.FailoverPolicy
	FailoverEvents   []controlplane.FailoverEvent
	ProtectionGroups []controlplane.RawDoc
	StorageTests     []controlplane.RawDoc
	// Warnings are entries that could not be parsed and were skipped.
	Warnings []string
}

// Snapshot is the canonical form used to compare and checksum: family -> key -> canonical value.
type Snapshot map[Family]map[string]string

// canonJSON re-encodes JSON with sorted keys and no insignificant whitespace.
func canonJSON(raw []byte) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func canon(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<unmarshalable: %v>", err)
	}
	return canonJSON(raw)
}

func tkey(id controlplane.ReplicationID) string {
	return strings.Join([]string{id.User, id.FromBucket, id.ToStorage, id.ToBucket}, "|")
}

func scopeKey(user, bucket string) string {
	if bucket == "" {
		return user
	}
	return user + "/" + bucket
}

// PostgreSQL stores timestamps with microsecond precision; Redis keeps nanoseconds. Comparisons use microseconds.
func ts(t time.Time) string { return t.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano) }

func statusCanon(s controlplane.ReplicationStatus) string {
	m := map[string]any{"createdAt": ts(s.CreatedAt), "archived": s.Archived, "agentUrl": s.AgentURL, "eventSource": s.EventSource}
	if s.ArchivedAt != nil {
		m["archivedAt"] = ts(*s.ArchivedAt)
	}
	return canon(m)
}

// Canonical converts the data to its comparable form.
func (d *Data) Canonical() Snapshot {
	s := Snapshot{}
	put := func(f Family, k, v string) {
		if s[f] == nil {
			s[f] = map[string]string{}
		}
		s[f][k] = v
	}
	for _, f := range Families {
		s[f] = map[string]string{}
	}
	for _, v := range d.Storages {
		put(FamStorages, v.Name, canon(v))
	}
	for _, v := range d.S3Identities {
		put(FamS3Identities, v.Name, canon(v))
	}
	for _, v := range d.DynamicCreds {
		put(FamDynamicCreds, v.RedisField(), v.Sealed)
	}
	if d.DynamicSalt != "" {
		put(FamDynamicSalt, "salt", d.DynamicSalt)
	}
	for _, v := range d.ProxyAccounts {
		put(FamProxyAccounts, v.AccessKeyID, canon(v))
	}
	for _, v := range d.AccessPolicies {
		put(FamAccessPolicies, v.User, canon(v))
	}
	for _, v := range d.ConsoleUsers {
		v.LastLoginAt = time.Time{}
		put(FamConsoleUsers, v.Username, canon(v))
	}
	for u, t := range d.ConsoleLogins {
		put(FamConsoleLogins, u, ts(t))
	}
	for _, e := range d.Routing.Routes {
		put(FamRoutes, scopeKey(e.User, e.Bucket), e.Storage)
	}
	for _, e := range d.Routing.Blocks {
		put(FamRouteBlocks, scopeKey(e.User, e.Bucket), "blocked")
	}
	for _, id := range d.ReplPolicies {
		put(FamReplPolicies, tkey(id), id.FromStorage)
	}
	for id, st := range d.ReplStatuses {
		put(FamReplStatus, id.String(), statusCanon(st))
	}
	for id, g := range d.ReplGenerations {
		put(FamReplGenerations, tkey(id), fmt.Sprint(g))
	}
	for _, v := range d.FailoverPolicies {
		put(FamFailoverPolicies, v.Source, canon(v))
	}
	for _, v := range d.FailoverEvents {
		put(FamFailoverEvents, v.Key(), canon(v))
	}
	for _, v := range d.ProtectionGroups {
		put(FamProtectionGroups, v.ID, canonJSON(v.JSON))
	}
	for _, v := range d.StorageTests {
		put(FamStorageTests, v.ID, canonJSON(v.JSON))
	}
	return s
}

// Summary is the count and checksum of one family.
type Summary struct {
	Count int    `json:"count"`
	SHA   string `json:"sha256"`
}

// Summarize computes count and checksum (SHA-256 over sorted key/value pairs) per family. The values of sealed
// secrets take part in the checksum but are never printed.
func (s Snapshot) Summarize() map[Family]Summary {
	res := make(map[Family]Summary, len(Families))
	for _, f := range Families {
		m := s[f]
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		h := sha256.New()
		for _, k := range keys {
			fmt.Fprintf(h, "%d:%s=%d:%s\n", len(k), k, len(m[k]), m[k])
		}
		res[f] = Summary{Count: len(m), SHA: hex.EncodeToString(h.Sum(nil))}
	}
	return res
}

// Diff is a difference between two snapshots; values are never included.
type Diff struct {
	Family Family `json:"family"`
	Key    string `json:"key"`
	// Kind: "missing_in_target", "missing_in_source" or "different".
	Kind string `json:"kind"`
}

// Compare lists the differences of source against target.
func Compare(source, target Snapshot) []Diff {
	var res []Diff
	for _, f := range Families {
		for k, v := range source[f] {
			tv, ok := target[f][k]
			switch {
			case !ok:
				res = append(res, Diff{f, k, "missing_in_target"})
			case tv != v:
				res = append(res, Diff{f, k, "different"})
			}
		}
		for k := range target[f] {
			if _, ok := source[f][k]; !ok {
				res = append(res, Diff{f, k, "missing_in_source"})
			}
		}
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Family != res[j].Family {
			return res[i].Family < res[j].Family
		}
		return res[i].Key < res[j].Key
	})
	return res
}
