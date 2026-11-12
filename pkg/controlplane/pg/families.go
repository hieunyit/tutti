// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

func hsetDoc[T any](key, field string, v *T, bump ...RedisOp) []RedisOp {
	raw, _ := json.Marshal(v)
	return append([]RedisOp{opHSet(key, field, raw)}, bump...)
}

func newStorages(r *runner) controlplane.Storages {
	return &kv[controlplane.StorageRecord]{rt: r, spec: kvSpec[controlplane.StorageRecord]{
		table: "storages", label: "storage",
		key:    func(v *controlplane.StorageRecord) string { return v.Name },
		setKey: func(v *controlplane.StorageRecord, k string) { v.Name = k },
		project: func(k string, v *controlplane.StorageRecord) []RedisOp {
			if v == nil {
				return []RedisOp{opHDel(KeyStorages, k), opIncr(KeyStoragesVersion)}
			}
			return hsetDoc(KeyStorages, k, v, opIncr(KeyStoragesVersion))
		},
	}}
}

func newS3Identities(r *runner) controlplane.S3Identities {
	return &kv[controlplane.S3Identity]{rt: r, spec: kvSpec[controlplane.S3Identity]{
		table: "s3_identities", label: "user record",
		key:    func(v *controlplane.S3Identity) string { return v.Name },
		setKey: func(v *controlplane.S3Identity, k string) { v.Name = k },
		project: func(k string, v *controlplane.S3Identity) []RedisOp {
			if v == nil {
				return []RedisOp{opHDel(KeyS3Users, k), opIncr(KeyS3UsersVersion)}
			}
			return hsetDoc(KeyS3Users, k, v, opIncr(KeyS3UsersVersion))
		},
	}}
}

func newProxyAccounts(r *runner) controlplane.ProxyAccounts {
	return &kv[controlplane.ProxyAccountRecord]{rt: r, spec: kvSpec[controlplane.ProxyAccountRecord]{
		table: "proxy_accounts", label: "proxy account",
		key:    func(v *controlplane.ProxyAccountRecord) string { return v.AccessKeyID },
		setKey: func(v *controlplane.ProxyAccountRecord, k string) { v.AccessKeyID = k },
		normalize: func(v *controlplane.ProxyAccountRecord) error {
			if v.User == "" {
				return fmt.Errorf("%w: proxy account: user is required", dom.ErrInvalidArg)
			}
			return nil
		},
		project: func(k string, v *controlplane.ProxyAccountRecord) []RedisOp {
			if v == nil {
				return []RedisOp{opHDel(KeyProxyAccounts, k)}
			}
			return hsetDoc(KeyProxyAccounts, k, v)
		},
	}}
}

func newAccessPolicies(r *runner) controlplane.AccessPolicies {
	return &kv[controlplane.AccessPolicy]{rt: r, spec: kvSpec[controlplane.AccessPolicy]{
		table: "access_policies", label: "access policy",
		key:    func(v *controlplane.AccessPolicy) string { return v.User },
		setKey: func(v *controlplane.AccessPolicy, k string) { v.User = k },
		project: func(k string, v *controlplane.AccessPolicy) []RedisOp {
			if v == nil {
				return []RedisOp{opHDel(KeyAccessPolicies, k)}
			}
			return hsetDoc(KeyAccessPolicies, k, v)
		},
	}}
}

func newFailoverPolicies(r *runner) controlplane.FailoverPolicies {
	return &kv[controlplane.FailoverPolicy]{rt: r, spec: kvSpec[controlplane.FailoverPolicy]{
		table: "failover_policies", label: "failover policy",
		key:    func(v *controlplane.FailoverPolicy) string { return v.Source },
		setKey: func(v *controlplane.FailoverPolicy, k string) { v.Source = k },
		project: func(k string, v *controlplane.FailoverPolicy) []RedisOp {
			if v == nil {
				return []RedisOp{opHDel(KeyFailoverPolicies, k)}
			}
			return hsetDoc(KeyFailoverPolicies, k, v)
		},
	}}
}

func newFailoverEvents(r *runner) controlplane.FailoverEvents {
	return &kv[controlplane.FailoverEvent]{rt: r, spec: kvSpec[controlplane.FailoverEvent]{
		table: "failover_events", label: "failover event",
		key:    func(v *controlplane.FailoverEvent) string { return v.Key() },
		setKey: func(*controlplane.FailoverEvent, string) {},
		project: func(k string, v *controlplane.FailoverEvent) []RedisOp {
			if v == nil {
				return []RedisOp{{Op: OpDel, Key: KeyFailoverEventPrefix + k}}
			}
			raw, _ := json.Marshal(v)
			return []RedisOp{{Op: OpSet, Key: KeyFailoverEventPrefix + k, Value: string(raw)}}
		},
	}}
}

// Protection groups are copied to PostgreSQL but not projected: the admission and fencing Lua scripts read the Redis
// documents and scope index, which pkg/protectiongroups maintains itself (see docs/tutti/control-plane-db.md).
func newProtectionGroups(r *runner) controlplane.ProtectionGroups {
	return &kv[controlplane.RawDoc]{rt: r, spec: rawSpec("protection_groups", "protection group", "")}
}

func newStorageTests(r *runner) controlplane.StorageTests {
	// every connection test overwrites the previous result: auditing them would flood the audit log
	spec := rawSpec("storage_tests", "storage test", KeyStorageTests)
	spec.noAudit = true
	return &kv[controlplane.RawDoc]{rt: r, spec: spec}
}

func rawSpec(table, label, redisKey string) kvSpec[controlplane.RawDoc] {
	return kvSpec[controlplane.RawDoc]{
		table: table, label: label,
		key:    func(v *controlplane.RawDoc) string { return v.ID },
		setKey: func(v *controlplane.RawDoc, k string) { v.ID = k },
		normalize: func(v *controlplane.RawDoc) error {
			if len(v.JSON) == 0 || !json.Valid(v.JSON) {
				return fmt.Errorf("%w: %s: document must be valid JSON", dom.ErrInvalidArg, label)
			}
			return nil
		},
		project: func(k string, v *controlplane.RawDoc) []RedisOp {
			if redisKey == "" {
				return nil
			}
			if v == nil {
				return []RedisOp{opHDel(redisKey, k)}
			}
			return []RedisOp{opHSet(redisKey, k, v.JSON)}
		},
	}
}

// ---- console users

type consoleUsers struct {
	*kv[controlplane.ConsoleUser]
}

func newConsoleUsers(r *runner) controlplane.ConsoleUsers {
	return &consoleUsers{&kv[controlplane.ConsoleUser]{rt: r, spec: kvSpec[controlplane.ConsoleUser]{
		table: "console_users", label: "console user",
		key:    func(v *controlplane.ConsoleUser) string { return v.Username },
		setKey: func(v *controlplane.ConsoleUser, k string) { v.Username = k },
		// the login time lives in its own column so that a login can never overwrite a concurrent change
		normalize: func(v *controlplane.ConsoleUser) error { v.LastLoginAt = time.Time{}; return nil },
		project: func(k string, v *controlplane.ConsoleUser) []RedisOp {
			if v == nil {
				return []RedisOp{opHDel(KeyConsoleUsers, k), opHDel(KeyConsoleLogins, k)}
			}
			return hsetDoc(KeyConsoleUsers, k, v)
		},
		extraCol: "last_login_at",
		applyExtra: func(v *controlplane.ConsoleUser, t *time.Time) {
			if t != nil {
				v.LastLoginAt = t.UTC()
			}
		},
	}}}
}

const consoleAdminLock int64 = 0x636f6e73 // "cons"

func (c *consoleUsers) MutateGuarded(ctx context.Context, username string, guard controlplane.ConsoleUserGuard,
	fn func(u *controlplane.ConsoleUser) (remove, checkOtherAdmin bool, err error)) error {
	return c.rt.write(ctx, func(tx pgx.Tx) error {
		// serializes guarded changes, like the hash-wide check of the Redis script: two concurrent demotions cannot
		// both remove the last administrator
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, consoleAdminLock); err != nil {
			return err
		}
		before, _, found, err := c.lock(ctx, tx, username)
		if err != nil {
			return err
		}
		if !found {
			return c.notFound(username)
		}
		var u controlplane.ConsoleUser
		if err := json.Unmarshal(before, &u); err != nil {
			return fmt.Errorf("%w: stored console user %q is corrupted: %v", dom.ErrInternal, username, err)
		}
		u.Username = username
		remove, check, err := fn(&u)
		if err != nil {
			return err
		}
		if check {
			var others int
			names := guard.ConfigNames
			if names == nil {
				names = []string{}
			}
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM console_users
				WHERE k <> $1 AND role='admin' AND NOT COALESCE((doc->>'disabled')::boolean,false) AND NOT (k = ANY($2))`,
				username, names).Scan(&others); err != nil {
				return err
			}
			if guard.ConfigAdmins+others == 0 {
				return fmt.Errorf("%w: cannot remove the last enabled admin", dom.ErrFailedPrecondition)
			}
		}
		if remove {
			return c.deleteTx(ctx, tx, username)
		}
		u.Username = username
		_, doc, err := c.prepare(&u)
		if err != nil {
			return err
		}
		_, err = c.store(ctx, tx, username, &u, doc, before)
		return err
	})
}

func (c *consoleUsers) TouchLogin(ctx context.Context, username string, at time.Time) error {
	if c.rt.readOnly != nil && c.rt.readOnly.Load() {
		return controlplane.ErrReadOnly
	}
	at = at.UTC().Truncate(time.Microsecond)
	tag, err := c.rt.q().Exec(ctx, `UPDATE console_users SET last_login_at=$2 WHERE k=$1`, username, at)
	if err != nil || tag.RowsAffected() == 0 || controlplane.SkipOutbox(ctx) {
		return err
	}
	// keep the Redis copy warm for a rollback; no audit row and no version bump, a login is not a configuration change
	ops, _ := json.Marshal([]RedisOp{{Op: OpHSet, Key: KeyConsoleLogins, Field: username, Value: at.Format(time.RFC3339Nano)}})
	_, err = c.rt.q().Exec(ctx, `INSERT INTO outbox (object_key, ops) VALUES ($1,$2)`, "console_user_logins/"+username, json.RawMessage(ops))
	return err
}

func newSystemSettings(r *runner) controlplane.SystemSettingsStore {
	return &kv[controlplane.SystemSettings]{rt: r, spec: kvSpec[controlplane.SystemSettings]{
		table: "system_settings", label: "system settings",
		key: func(v *controlplane.SystemSettings) string {
			if v.ID == "" {
				return "system"
			}
			return v.ID
		},
		setKey: func(v *controlplane.SystemSettings, k string) { v.ID = k },
		project: func(k string, v *controlplane.SystemSettings) []RedisOp {
			if v == nil {
				return []RedisOp{
					{Op: OpDel, Key: KeySystemSettings},
					{Op: OpIncr, Key: KeySystemSettingsVersion},
					{Op: OpPublish, Key: SystemSettingsChannel, Value: "deleted"},
				}
			}
			raw, _ := json.Marshal(v)
			return []RedisOp{
				{Op: OpSet, Key: KeySystemSettings, Value: string(raw)},
				{Op: OpIncr, Key: KeySystemSettingsVersion},
				{Op: OpPublish, Key: SystemSettingsChannel, Value: "updated"},
			}
		},
	}}
}

