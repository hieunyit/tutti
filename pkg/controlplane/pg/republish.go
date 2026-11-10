// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// EnqueueSnapshot inserts outbox entries for every record currently stored in PostgreSQL without
// modifying row versions or writing audit log entries. Applying those entries via Publisher rebuilds
// the Redis projection from PostgreSQL when Redis has been lost or reset.
func (s *Store) EnqueueSnapshot(ctx context.Context) (int, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("controlplane: begin snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	// Exclusive against every writer that has an outbox entry in flight (they hold the shared lock until commit), taken
	// before anything is read. The transaction stays READ COMMITTED on purpose: a REPEATABLE READ snapshot would be
	// fixed by the lock statement itself, before the wait for in-flight writers, and could miss their commits; with
	// READ COMMITTED every statement after the lock sees all committed changes, and later writers queue behind this
	// transaction, so their (newer) entries get higher ids than the snapshot entries and are applied after them.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, snapshotLockID); err != nil {
		return 0, fmt.Errorf("controlplane: snapshot lock: %w", err)
	}

	r := newRepos(&runner{pool: s.pool, tx: tx, readOnly: &s.readOnly})
	enqueued := 0
	enqueue := func(objectKey string, ops []RedisOp) error {
		if len(ops) == 0 {
			return nil
		}
		raw, err := json.Marshal(ops)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox (object_key, ops) VALUES ($1,$2)`, objectKey, json.RawMessage(raw)); err != nil {
			return fmt.Errorf("controlplane: enqueue outbox %s: %w", objectKey, err)
		}
		enqueued++
		return nil
	}

	storages, err := r.Storages.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range storages {
		val := v.Value
		if err := enqueue("storages/"+val.Name, hsetDoc(KeyStorages, val.Name, &val, opIncr(KeyStoragesVersion))); err != nil {
			return 0, err
		}
	}

	identities, err := r.S3Identities.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range identities {
		val := v.Value
		if err := enqueue("s3_identities/"+val.Name, hsetDoc(KeyS3Users, val.Name, &val, opIncr(KeyS3UsersVersion))); err != nil {
			return 0, err
		}
	}

	if salt, ok, err := r.DynamicCredentials.Salt(ctx); err != nil {
		return 0, err
	} else if ok && salt != "" {
		if err := enqueue("dynamic_credentials_salt/salt", []RedisOp{{Op: OpHSetNX, Key: KeyDynamicCreds, Field: "salt", Value: salt}}); err != nil {
			return 0, err
		}
	}

	creds, err := r.DynamicCredentials.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, c := range creds {
		if err := enqueue("dynamic_credentials/"+c.RedisField(), []RedisOp{
			{Op: OpHSet, Key: KeyDynamicCreds, Field: c.RedisField(), Value: c.Sealed},
			{Op: OpHIncr, Key: KeyDynamicCreds, Field: "version"},
		}); err != nil {
			return 0, err
		}
	}

	accounts, err := r.ProxyAccounts.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range accounts {
		val := v.Value
		if err := enqueue("proxy_accounts/"+val.AccessKeyID, hsetDoc(KeyProxyAccounts, val.AccessKeyID, &val)); err != nil {
			return 0, err
		}
	}

	policies, err := r.AccessPolicies.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range policies {
		val := v.Value
		if err := enqueue("access_policies/"+val.User, hsetDoc(KeyAccessPolicies, val.User, &val)); err != nil {
			return 0, err
		}
	}

	users, err := r.ConsoleUsers.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range users {
		val := v.Value
		loginAt := val.LastLoginAt
		val.LastLoginAt = time.Time{}
		if err := enqueue("console_users/"+val.Username, hsetDoc(KeyConsoleUsers, val.Username, &val)); err != nil {
			return 0, err
		}
		if !loginAt.IsZero() {
			ts := loginAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
			if err := enqueue("console_user_logins/"+val.Username, []RedisOp{{Op: OpHSet, Key: KeyConsoleLogins, Field: val.Username, Value: ts}}); err != nil {
				return 0, err
			}
		}
	}

	routingExp, err := r.Routing.Export(ctx)
	if err != nil {
		return 0, err
	}
	for _, e := range routingExp.Routes {
		if err := enqueue("routes/"+scopeID(e.User, e.Bucket), routeOps(e.User, e.Bucket, e.Storage, false)); err != nil {
			return 0, err
		}
	}
	for _, e := range routingExp.Blocks {
		if err := enqueue("route_blocks/"+scopeID(e.User, e.Bucket), blockOps(e.User, e.Bucket, true)); err != nil {
			return 0, err
		}
	}

	statuses, err := r.Replications.ListStatuses(ctx, "")
	if err != nil {
		return 0, err
	}
	for id, st := range statuses {
		if err := enqueue(replScopeKey(id.User, id.FromBucket), statusOps(id, st)); err != nil {
			return 0, err
		}
	}

	type scope struct{ user, bucket string }
	scopes := map[scope]bool{}
	replPols, err := r.Replications.ListAll(ctx)
	if err != nil {
		return 0, err
	}
	for _, id := range replPols {
		scopes[scope{id.User, id.FromBucket}] = true
	}
	replGens, err := r.Replications.ListGenerations(ctx)
	if err != nil {
		return 0, err
	}
	for id := range replGens {
		scopes[scope{id.User, id.FromBucket}] = true
	}
	scopeList := make([]scope, 0, len(scopes))
	for sc := range scopes {
		scopeList = append(scopeList, sc)
	}
	sort.Slice(scopeList, func(i, j int) bool {
		if scopeList[i].user != scopeList[j].user {
			return scopeList[i].user < scopeList[j].user
		}
		return scopeList[i].bucket < scopeList[j].bucket
	})
	for _, sc := range scopeList {
		ops, err := scopeOps(ctx, tx, sc.user, sc.bucket)
		if err != nil {
			return 0, err
		}
		if err := enqueue(replScopeKey(sc.user, sc.bucket), ops); err != nil {
			return 0, err
		}
	}

	fpols, err := r.FailoverPolicies.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range fpols {
		val := v.Value
		if err := enqueue("failover_policies/"+val.Source, hsetDoc(KeyFailoverPolicies, val.Source, &val)); err != nil {
			return 0, err
		}
	}

	fevents, err := r.FailoverEvents.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range fevents {
		val := v.Value
		raw, _ := json.Marshal(val)
		if err := enqueue("failover_events/"+val.Key(), []RedisOp{{Op: OpSet, Key: KeyFailoverEventPrefix + val.Key(), Value: string(raw)}}); err != nil {
			return 0, err
		}
	}

	// Protection groups are deliberately not projected: Redis holds their runtime fields (promotion, fencing, scope
	// index) next to the desired spec and pkg/protectiongroups maintains both; overwriting the documents from
	// PostgreSQL would reset the runtime state.

	stests, err := r.StorageTests.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, v := range stests {
		val := v.Value
		if err := enqueue("storage_tests/"+val.ID, []RedisOp{opHSet(KeyStorageTests, val.ID, val.JSON)}); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("controlplane: commit snapshot: %w", err)
	}
	return enqueued, nil
}
