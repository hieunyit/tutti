// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

// PruneItem is a Redis projection entry that PostgreSQL does not know: Family and Key are the canonical family name
// and key of pkg/controlplane/migrate (the family names are the PostgreSQL table names, except where noted).
type PruneItem struct {
	Family string
	Key    string
}

// OutboxHighWater returns the highest outbox id (0 if empty). Entries up to it were enqueued before the call.
func (s *Store) OutboxHighWater(ctx context.Context) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT coalesce(max(id),0) FROM outbox`).Scan(&id)
	return id, err
}

// PendingUpTo counts the unpublished entries with an id up to maxID: zero means everything enqueued before maxID
// was taken is projected to Redis (entries of concurrent writers with higher ids do not matter).
func (s *Store) PendingUpTo(ctx context.Context, maxID int64) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL AND id<=$1`, maxID).Scan(&n)
	return n, err
}

// EnqueuePrune enqueues the removal of Redis entries that have no PostgreSQL row. Each item is re-checked against the
// current PostgreSQL state under the snapshot lock, so a row created after the caller compared the two is never
// removed. Replication policies and generations are fixed by re-projecting the whole scope from PostgreSQL (absolute).
// Protection groups, the dynamic-credential salt and console login times are never pruned. It returns the number of
// entries enqueued.
func (s *Store) EnqueuePrune(ctx context.Context, items []PruneItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("controlplane: begin prune: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, snapshotLockID); err != nil {
		return 0, fmt.Errorf("controlplane: prune lock: %w", err)
	}
	n := 0
	// the entries use the object keys of the normal writes, so they are ordered with them
	enqueue := func(key string, ops []RedisOp) error {
		raw, err := json.Marshal(ops)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox (object_key, ops) VALUES ($1,$2)`, key, json.RawMessage(raw)); err != nil {
			return fmt.Errorf("controlplane: enqueue prune %s: %w", key, err)
		}
		n++
		return nil
	}
	exists := func(q string, args ...any) (bool, error) {
		var ok bool
		err := tx.QueryRow(ctx, q, args...).Scan(&ok)
		return ok, err
	}
	scopes := map[string]bool{}
	for _, it := range items {
		var (
			ops    []RedisOp
			object = it.Family + "/" + it.Key
			ok     bool
			err    error
		)
		kvOps := func(table string, del []RedisOp) {
			ok, err = exists(fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE k=$1)`, table), it.Key)
			if err == nil && !ok {
				ops = del
			}
		}
		switch it.Family {
		case "storages":
			kvOps("storages", []RedisOp{opHDel(KeyStorages, it.Key), opIncr(KeyStoragesVersion)})
		case "s3_identities":
			kvOps("s3_identities", []RedisOp{opHDel(KeyS3Users, it.Key), opIncr(KeyS3UsersVersion)})
		case "proxy_accounts":
			kvOps("proxy_accounts", []RedisOp{opHDel(KeyProxyAccounts, it.Key)})
		case "access_policies":
			kvOps("access_policies", []RedisOp{opHDel(KeyAccessPolicies, it.Key)})
		case "console_users":
			kvOps("console_users", []RedisOp{opHDel(KeyConsoleUsers, it.Key), opHDel(KeyConsoleLogins, it.Key)})
		case "failover_policies":
			kvOps("failover_policies", []RedisOp{opHDel(KeyFailoverPolicies, it.Key)})
		case "failover_events":
			kvOps("failover_events", []RedisOp{{Op: OpDel, Key: KeyFailoverEventPrefix + it.Key}})
		case "storage_tests":
			kvOps("storage_tests", []RedisOp{opHDel(KeyStorageTests, it.Key)})
		case "dynamic_credentials":
			parts := strings.SplitN(it.Key, ":", 3)
			if len(parts) != 3 {
				continue
			}
			ok, err = exists(`SELECT EXISTS (SELECT 1 FROM dynamic_credentials WHERE storage_type=$1 AND storage=$2 AND user_name=$3)`, parts[0], parts[1], parts[2])
			if err == nil && !ok {
				ops = []RedisOp{opHDel(KeyDynamicCreds, it.Key), {Op: OpHIncr, Key: KeyDynamicCreds, Field: "version"}}
			}
		case "routes", "route_blocks":
			user, bucket, _ := strings.Cut(it.Key, "/")
			table := "routes"
			if it.Family == "route_blocks" {
				table = "route_blocks"
			}
			ok, err = exists(fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE user_name=$1 AND bucket=$2)`, table), user, bucket)
			if err == nil && !ok {
				if it.Family == "routes" {
					ops = routeOps(user, bucket, "", true)
				} else {
					ops = blockOps(user, bucket, false)
				}
			}
		case "replication_policies", "replication_generations":
			// key: user|fromBucket|toStorage|toBucket
			parts := strings.Split(it.Key, "|")
			if len(parts) != 4 {
				continue
			}
			sk := replScopeKey(parts[0], parts[1])
			if scopes[sk] {
				continue
			}
			scopes[sk] = true
			sops, serr := scopeOps(ctx, tx, parts[0], parts[1])
			if serr != nil {
				return 0, serr
			}
			if err := enqueue(sk, sops); err != nil {
				return 0, err
			}
			continue
		case "replication_status":
			id, valid := controlplane.ParseReplicationID(it.Key)
			if !valid {
				continue
			}
			ok, err = exists(`SELECT EXISTS (SELECT 1 FROM replication_status WHERE user_name=$1 AND from_storage=$2 AND to_storage=$3 AND from_bucket=$4 AND to_bucket=$5)`,
				id.User, id.FromStorage, id.ToStorage, id.FromBucket, id.ToBucket)
			if err == nil && !ok {
				h, idx, member := statusKeys(id)
				ops = []RedisOp{{Op: OpDel, Key: h}, {Op: OpSRem, Key: idx, Members: []string{member}}}
				object = replScopeKey(id.User, id.FromBucket)
			}
		default:
			continue
		}
		if err != nil {
			return 0, err
		}
		if len(ops) == 0 {
			continue
		}
		if err := enqueue(object, ops); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("controlplane: commit prune: %w", err)
	}
	return n, nil
}
