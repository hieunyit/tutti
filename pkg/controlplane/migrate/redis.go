// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
)

// Sources are the Redis instances to read: Config is the config role (storages, users, policies, routing, replication,
// failover, groups), Meta the role that holds the dynamic credentials hash (tutti:dynamic_creds). Meta defaults to
// Config.
type Sources struct {
	Config redis.UniversalClient
	Meta   redis.UniversalClient
}

func scanKeys(ctx context.Context, rdb redis.UniversalClient, pattern string) ([]string, error) {
	var keys []string
	it := rdb.Scan(ctx, 0, pattern, 500).Iterator()
	for it.Next(ctx) {
		keys = append(keys, it.Val())
	}
	sort.Strings(keys)
	return keys, it.Err()
}

func hashAll(ctx context.Context, rdb redis.UniversalClient, key string) (map[string]string, error) {
	res := map[string]string{}
	it := rdb.HScan(ctx, key, 0, "*", 500).Iterator()
	for it.Next(ctx) {
		f := it.Val()
		if !it.Next(ctx) {
			break
		}
		res[f] = it.Val()
	}
	return res, it.Err()
}

// decode unmarshals one document; failures become warnings so that one corrupt entry does not block the migration.
func decode[T any](d *Data, what, key, raw string) (T, bool) {
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		d.Warnings = append(d.Warnings, fmt.Sprintf("%s %q: skipped, malformed document: %v", what, key, err))
		return v, false
	}
	return v, true
}

// ReadRedis reads all control-plane data. It uses only read commands.
func ReadRedis(ctx context.Context, src Sources) (*Data, error) {
	rdb := src.Config
	meta := src.Meta
	if meta == nil {
		meta = rdb
	}
	d := &Data{ConsoleLogins: map[string]time.Time{}, ReplStatuses: map[controlplane.ReplicationID]controlplane.ReplicationStatus{}, ReplGenerations: map[controlplane.ReplicationID]int64{}}
	docs := func(key string) (map[string]string, error) {
		m, err := hashAll(ctx, rdb, key)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", key, err)
		}
		return m, nil
	}
	sorted := func(m map[string]string) []string {
		ks := make([]string, 0, len(m))
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return ks
	}

	m, err := docs(pg.KeyStorages)
	if err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if v, ok := decode[controlplane.StorageRecord](d, "storage", k, m[k]); ok {
			v.Name = k
			d.Storages = append(d.Storages, v)
		}
	}
	if m, err = docs(pg.KeyS3Users); err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if v, ok := decode[controlplane.S3Identity](d, "user record", k, m[k]); ok {
			v.Name = k
			d.S3Identities = append(d.S3Identities, v)
		}
	}
	if m, err = hashAll(ctx, meta, pg.KeyDynamicCreds); err != nil {
		return nil, fmt.Errorf("read %s: %w", pg.KeyDynamicCreds, err)
	}
	for _, k := range sorted(m) {
		switch k {
		case "version":
			continue // counter, rebuilt by the outbox
		case "salt":
			d.DynamicSalt = m[k]
			continue
		}
		parts := strings.SplitN(k, ":", 3)
		if len(parts) != 3 {
			d.Warnings = append(d.Warnings, fmt.Sprintf("dynamic credential field %q: skipped, unexpected format", k))
			continue
		}
		d.DynamicCreds = append(d.DynamicCreds, controlplane.DynamicCredential{StorageType: parts[0], Storage: parts[1], User: parts[2], Sealed: m[k]})
	}
	if m, err = docs(pg.KeyProxyAccounts); err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if v, ok := decode[controlplane.ProxyAccountRecord](d, "proxy account", k, m[k]); ok {
			v.AccessKeyID = k
			d.ProxyAccounts = append(d.ProxyAccounts, v)
		}
	}
	if m, err = docs(pg.KeyAccessPolicies); err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if v, ok := decode[controlplane.AccessPolicy](d, "access policy", k, m[k]); ok {
			v.User = k
			d.AccessPolicies = append(d.AccessPolicies, v)
		}
	}
	if m, err = docs(pg.KeyConsoleUsers); err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if v, ok := decode[controlplane.ConsoleUser](d, "console user", k, m[k]); ok {
			v.Username = k
			v.LastLoginAt = time.Time{}
			d.ConsoleUsers = append(d.ConsoleUsers, v)
		}
	}
	if m, err = docs(pg.KeyConsoleLogins); err != nil {
		return nil, err
	}
	for k, v := range m {
		if _, ok := d.ConsoleLoginUser(k); !ok {
			continue // login of a deleted user
		}
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			d.Warnings = append(d.Warnings, fmt.Sprintf("console login %q: skipped, bad time: %v", k, err))
			continue
		}
		d.ConsoleLogins[k] = t.UTC()
	}
	if err := readRouting(ctx, rdb, d); err != nil {
		return nil, err
	}
	if err := readReplication(ctx, rdb, d); err != nil {
		return nil, err
	}
	if m, err = docs(pg.KeyFailoverPolicies); err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if v, ok := decode[controlplane.FailoverPolicy](d, "failover policy", k, m[k]); ok {
			v.Source = k
			d.FailoverPolicies = append(d.FailoverPolicies, v)
		}
	}
	for _, kind := range []string{"user", "bucket"} {
		keys, err := scanKeys(ctx, rdb, pg.KeyFailoverEventPrefix+kind+":*")
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			raw, err := rdb.Get(ctx, key).Result()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				d.Warnings = append(d.Warnings, fmt.Sprintf("failover event %q: skipped: %v", key, err))
				continue
			}
			if v, ok := decode[controlplane.FailoverEvent](d, "failover event", key, raw); ok {
				d.FailoverEvents = append(d.FailoverEvents, v)
			}
		}
	}
	if m, err = docs(pg.KeyProtectionGroups); err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if json.Valid([]byte(m[k])) {
			d.ProtectionGroups = append(d.ProtectionGroups, controlplane.RawDoc{ID: k, JSON: json.RawMessage(m[k])})
		} else {
			d.Warnings = append(d.Warnings, fmt.Sprintf("protection group %q: skipped, malformed document", k))
		}
	}
	if m, err = docs(pg.KeyStorageTests); err != nil {
		return nil, err
	}
	for _, k := range sorted(m) {
		if json.Valid([]byte(m[k])) {
			d.StorageTests = append(d.StorageTests, controlplane.RawDoc{ID: k, JSON: json.RawMessage(m[k])})
		} else {
			d.Warnings = append(d.Warnings, fmt.Sprintf("storage test %q: skipped, malformed document", k))
		}
	}
	return d, nil
}

// ConsoleLoginUser reports whether the user exists in the data.
func (d *Data) ConsoleLoginUser(name string) (controlplane.ConsoleUser, bool) {
	for _, u := range d.ConsoleUsers {
		if u.Username == name {
			return u, true
		}
	}
	return controlplane.ConsoleUser{}, false
}

func splitUserRest(key, prefix string) (user, rest string) {
	s := strings.TrimPrefix(key, prefix+":")
	user, rest, _ = strings.Cut(s, ":")
	return
}

func readRouting(ctx context.Context, rdb redis.UniversalClient, d *Data) error {
	users, err := hashAll(ctx, rdb, pg.KeyRouteUser)
	if err != nil {
		return err
	}
	for u, st := range users {
		d.Routing.Routes = append(d.Routing.Routes, controlplane.RouteEntry{User: u, Storage: st})
	}
	blocked, err := rdb.SMembers(ctx, pg.KeyRouteBlockUser).Result()
	if err != nil {
		return err
	}
	for _, u := range blocked {
		d.Routing.Blocks = append(d.Routing.Blocks, controlplane.RouteEntry{User: u})
	}
	keys, err := scanKeys(ctx, rdb, pg.KeyRouteBucket+":*")
	if err != nil {
		return err
	}
	for _, key := range keys {
		user := strings.TrimPrefix(key, pg.KeyRouteBucket+":")
		m, err := hashAll(ctx, rdb, key)
		if err != nil {
			return err
		}
		for b, st := range m {
			d.Routing.Routes = append(d.Routing.Routes, controlplane.RouteEntry{User: user, Bucket: b, Storage: st})
		}
	}
	if keys, err = scanKeys(ctx, rdb, pg.KeyRouteBlockBuckt+":*"); err != nil {
		return err
	}
	for _, key := range keys {
		user := strings.TrimPrefix(key, pg.KeyRouteBlockBuckt+":")
		bs, err := rdb.SMembers(ctx, key).Result()
		if err != nil {
			return err
		}
		for _, b := range bs {
			d.Routing.Blocks = append(d.Routing.Blocks, controlplane.RouteEntry{User: user, Bucket: b})
		}
	}
	return nil
}

func splitDest(s string) (storage, bucket string) {
	storage, bucket, _ = strings.Cut(s, ":")
	return
}

func readReplication(ctx context.Context, rdb redis.UniversalClient, d *Data) error {
	// user-level policies
	from, err := hashAll(ctx, rdb, pg.KeyReplUserFrom)
	if err != nil {
		return err
	}
	for user, fromStorage := range from {
		tos, err := rdb.SMembers(ctx, pg.KeyReplUserTo+":"+user).Result()
		if err != nil {
			return err
		}
		for _, to := range tos {
			d.ReplPolicies = append(d.ReplPolicies, controlplane.ReplicationID{User: user, FromStorage: fromStorage, ToStorage: to})
		}
	}
	// bucket-level policies
	keys, err := scanKeys(ctx, rdb, pg.KeyReplBucketFrom+":*")
	if err != nil {
		return err
	}
	for _, key := range keys {
		user := strings.TrimPrefix(key, pg.KeyReplBucketFrom+":")
		m, err := hashAll(ctx, rdb, key)
		if err != nil {
			return err
		}
		for fromBucket, fromStorage := range m {
			tos, err := rdb.SMembers(ctx, pg.KeyReplBucketTo+":"+user+":"+fromBucket).Result()
			if err != nil {
				return err
			}
			for _, to := range tos {
				ts, tb := splitDest(to)
				d.ReplPolicies = append(d.ReplPolicies, controlplane.ReplicationID{User: user, FromStorage: fromStorage, FromBucket: fromBucket, ToStorage: ts, ToBucket: tb})
			}
		}
	}
	// generations
	if keys, err = scanKeys(ctx, rdb, pg.KeyReplUserGen+":*"); err != nil {
		return err
	}
	for _, key := range keys {
		user := strings.TrimPrefix(key, pg.KeyReplUserGen+":")
		m, err := hashAll(ctx, rdb, key)
		if err != nil {
			return err
		}
		for to, g := range m {
			addGen(d, controlplane.ReplicationID{User: user, ToStorage: to}, g)
		}
	}
	if keys, err = scanKeys(ctx, rdb, pg.KeyReplBucketGen+":*"); err != nil {
		return err
	}
	for _, key := range keys {
		user, fb := splitUserRest(key, pg.KeyReplBucketGen)
		m, err := hashAll(ctx, rdb, key)
		if err != nil {
			return err
		}
		for dest, g := range m {
			ts, tb := splitDest(dest)
			addGen(d, controlplane.ReplicationID{User: user, FromBucket: fb, ToStorage: ts, ToBucket: tb}, g)
		}
	}
	// statuses, listed by their indexes (they outlive the policies)
	ids, err := rdb.SMembers(ctx, pg.KeyReplUserStatusIndex).Result()
	if err != nil {
		return err
	}
	if keys, err = scanKeys(ctx, rdb, pg.KeyReplBucketStatusIndex+":*"); err != nil {
		return err
	}
	for _, key := range keys {
		more, err := rdb.SMembers(ctx, key).Result()
		if err != nil {
			return err
		}
		ids = append(ids, more...)
	}
	sort.Strings(ids)
	for _, s := range ids {
		if strings.HasPrefix(s, "{") { // the user-level index stores JSON (entity.UserReplicationPolicy)
			var up struct{ User, FromStorage, ToStorage string }
			if json.Unmarshal([]byte(s), &up) == nil && up.User != "" {
				s = controlplane.ReplicationID{User: up.User, FromStorage: up.FromStorage, ToStorage: up.ToStorage}.String()
			}
		}
		id, ok := controlplane.ParseReplicationID(s)
		if !ok {
			d.Warnings = append(d.Warnings, fmt.Sprintf("replication status index entry %q: skipped, bad id", s))
			continue
		}
		key := pg.KeyReplUserStatus + ":" + s
		if !id.IsUserLevel() {
			key = pg.KeyReplBucketStatus + ":" + s
		}
		h, err := rdb.HGetAll(ctx, key).Result()
		if err != nil {
			return err
		}
		if len(h) == 0 {
			d.Warnings = append(d.Warnings, fmt.Sprintf("replication status %q: indexed but the hash is missing, skipped", s))
			continue
		}
		st, err := parseStatus(h)
		if err != nil {
			d.Warnings = append(d.Warnings, fmt.Sprintf("replication status %q: skipped: %v", s, err))
			continue
		}
		d.ReplStatuses[id] = st
	}
	return nil
}

func addGen(d *Data, id controlplane.ReplicationID, v string) {
	var g int64
	if _, err := fmt.Sscan(v, &g); err != nil {
		d.Warnings = append(d.Warnings, fmt.Sprintf("replication generation %q: skipped, not a number", id.String()))
		return
	}
	d.ReplGenerations[id] = g
}

// parseStatus decodes the config fields of the status hash as written by go-redis (RFC 3339 times, "1"/"0" bools).
// failover_pending is runtime state and is deliberately ignored.
func parseStatus(h map[string]string) (controlplane.ReplicationStatus, error) {
	var st controlplane.ReplicationStatus
	if v := h["created_at"]; v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return st, fmt.Errorf("created_at: %w", err)
		}
		st.CreatedAt = t.UTC()
	}
	if v := h["archived_at"]; v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return st, fmt.Errorf("archived_at: %w", err)
		}
		t = t.UTC()
		st.ArchivedAt = &t
	}
	st.Archived = h["archived"] == "1" || h["archived"] == "true"
	st.AgentURL = h["agent_url"]
	st.EventSource = h["event_source"]
	return st, nil
}
