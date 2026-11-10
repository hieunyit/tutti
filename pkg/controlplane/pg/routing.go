// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

// Redis keys of the routing stores (pkg/store/routing_stores.go). A bucket-level key is "<prefix>:<user>".
const (
	// the global (user-level) keys have a trailing ":" because the stores build them with an empty ID token
	KeyRouteUser       = "p:route_user:"
	KeyRouteBlockUser  = "p:route_block_user:"
	KeyRouteBucket     = "p:route_bucket"
	KeyRouteBlockBuckt = "p:route_block_bucket"
)

type routing struct{ rt *runner }

func scopeID(user, bucket string) string {
	if bucket == "" {
		return user
	}
	return user + "/" + bucket
}

func routeOps(user, bucket, storage string, del bool) []RedisOp {
	key, field := KeyRouteUser, user
	if bucket != "" {
		key, field = KeyRouteBucket+":"+user, bucket
	}
	if del {
		return []RedisOp{opHDel(key, field), opSnapshot()}
	}
	return []RedisOp{{Op: OpHSet, Key: key, Field: field, Value: storage}, opSnapshot()}
}

func blockOps(user, bucket string, add bool) []RedisOp {
	key, member := KeyRouteBlockUser, user
	if bucket != "" {
		key, member = KeyRouteBlockBuckt+":"+user, bucket
	}
	op := OpSRem
	if add {
		op = OpSAdd
	}
	return []RedisOp{{Op: op, Key: key, Members: []string{member}}, opSnapshot()}
}

func routeDoc(user, bucket, storage string) []byte {
	b, _ := json.Marshal(map[string]string{"user": user, "bucket": bucket, "storage": storage})
	return b
}

func checkScope(user, bucket string) error {
	if user == "" {
		return fmt.Errorf("%w: routing: user is required", dom.ErrInvalidArg)
	}
	_ = bucket
	return nil
}

func (r *routing) Get(ctx context.Context, user, bucket string) (string, error) {
	var blocked bool
	if err := r.rt.q().QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM route_blocks WHERE user_name=$1 AND bucket=$2)`, user, bucket).Scan(&blocked); err != nil {
		return "", err
	}
	if blocked {
		return "", dom.ErrRoutingBlock
	}
	var storage string
	err := r.rt.q().QueryRow(ctx, `SELECT storage FROM routes WHERE user_name=$1 AND bucket=$2`, user, bucket).Scan(&storage)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: routing of %q", dom.ErrNotFound, scopeID(user, bucket))
	}
	return storage, err
}

func (r *routing) Set(ctx context.Context, user, bucket, storage string) error {
	if err := checkScope(user, bucket); err != nil {
		return err
	}
	if storage == "" {
		return fmt.Errorf("%w: routing: storage is required", dom.ErrInvalidArg)
	}
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		var prev *string
		err := tx.QueryRow(ctx, `SELECT storage FROM routes WHERE user_name=$1 AND bucket=$2 FOR UPDATE`, user, bucket).Scan(&prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO routes (user_name, bucket, storage) VALUES ($1,$2,$3)
			ON CONFLICT (user_name, bucket) DO UPDATE SET storage=EXCLUDED.storage, version=routes.version+1, updated_at=now()`, user, bucket, storage); err != nil {
			return err
		}
		m := mutation{action: "set", objectType: "routes", objectID: scopeID(user, bucket), after: routeDoc(user, bucket, storage), ops: routeOps(user, bucket, storage, false)}
		if prev != nil {
			m.before = routeDoc(user, bucket, *prev)
		}
		return record(ctx, tx, m)
	})
}

func (r *routing) Update(ctx context.Context, user, bucket, storage string) error {
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		var prev string
		err := tx.QueryRow(ctx, `SELECT storage FROM routes WHERE user_name=$1 AND bucket=$2 FOR UPDATE`, user, bucket).Scan(&prev)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: routing of %q", dom.ErrNotFound, scopeID(user, bucket))
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE routes SET storage=$3, version=version+1, updated_at=now() WHERE user_name=$1 AND bucket=$2`, user, bucket, storage); err != nil {
			return err
		}
		return record(ctx, tx, mutation{action: "update", objectType: "routes", objectID: scopeID(user, bucket),
			before: routeDoc(user, bucket, prev), after: routeDoc(user, bucket, storage), ops: routeOps(user, bucket, storage, false)})
	})
}

func (r *routing) DeleteRoute(ctx context.Context, user, bucket string) error {
	return r.rt.write(ctx, func(tx pgx.Tx) error { return deleteRoute(ctx, tx, user, bucket) })
}

func deleteRoute(ctx context.Context, tx pgx.Tx, user, bucket string) error {
	var prev string
	err := tx.QueryRow(ctx, `DELETE FROM routes WHERE user_name=$1 AND bucket=$2 RETURNING storage`, user, bucket).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // like HDEL of a missing field
	}
	if err != nil {
		return err
	}
	return record(ctx, tx, mutation{action: "delete", objectType: "routes", objectID: scopeID(user, bucket), before: routeDoc(user, bucket, prev), ops: routeOps(user, bucket, "", true)})
}

func (r *routing) setBlock(ctx context.Context, user, bucket string, block bool) error {
	if err := checkScope(user, bucket); err != nil {
		return err
	}
	return r.rt.write(ctx, func(tx pgx.Tx) error { return setBlock(ctx, tx, user, bucket, block) })
}

func setBlock(ctx context.Context, tx pgx.Tx, user, bucket string, block bool) error {
	var changed bool
	if block {
		tag, err := tx.Exec(ctx, `INSERT INTO route_blocks (user_name, bucket) VALUES ($1,$2) ON CONFLICT DO NOTHING`, user, bucket)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected() > 0
	} else {
		tag, err := tx.Exec(ctx, `DELETE FROM route_blocks WHERE user_name=$1 AND bucket=$2`, user, bucket)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected() > 0
	}
	if !changed {
		return nil
	}
	m := mutation{action: "unblock", objectType: "route_blocks", objectID: scopeID(user, bucket), before: routeDoc(user, bucket, ""), ops: blockOps(user, bucket, false)}
	if block {
		m = mutation{action: "block", objectType: "route_blocks", objectID: scopeID(user, bucket), after: routeDoc(user, bucket, ""), ops: blockOps(user, bucket, true)}
	}
	return record(ctx, tx, m)
}

func (r *routing) Block(ctx context.Context, user, bucket string) error {
	return r.setBlock(ctx, user, bucket, true)
}

func (r *routing) Unblock(ctx context.Context, user, bucket string) error {
	return r.setBlock(ctx, user, bucket, false)
}

func (r *routing) DeleteAllUser(ctx context.Context, user string) error {
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		if err := setBlock(ctx, tx, user, "", false); err != nil {
			return err
		}
		return deleteRoute(ctx, tx, user, "")
	})
}

func (r *routing) pairs(ctx context.Context, query string, args ...any) (map[string]string, error) {
	rows, err := r.rt.q().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		res[k] = v
	}
	return res, rows.Err()
}

func (r *routing) ListUserRoutes(ctx context.Context) (map[string]string, error) {
	return r.pairs(ctx, `SELECT user_name, storage FROM routes WHERE bucket=''`)
}

func (r *routing) ListBucketRoutes(ctx context.Context, user string) (map[string]string, error) {
	return r.pairs(ctx, `SELECT bucket, storage FROM routes WHERE user_name=$1 AND bucket<>''`, user)
}

func (r *routing) blocks(ctx context.Context, query string, args ...any) (map[string]bool, error) {
	m, err := r.pairs(ctx, query, args...)
	res := make(map[string]bool, len(m))
	for k := range m {
		res[k] = true
	}
	return res, err
}

func (r *routing) ListUserBlocks(ctx context.Context) (map[string]bool, error) {
	return r.blocks(ctx, `SELECT user_name, '' FROM route_blocks WHERE bucket=''`)
}

func (r *routing) ListBucketBlocks(ctx context.Context, user string) (map[string]bool, error) {
	return r.blocks(ctx, `SELECT bucket, '' FROM route_blocks WHERE user_name=$1 AND bucket<>''`, user)
}

func (r *routing) Export(ctx context.Context) (controlplane.RoutingExport, error) {
	var res controlplane.RoutingExport
	rows, err := r.rt.q().Query(ctx, `SELECT user_name, bucket, storage FROM routes ORDER BY user_name, bucket`)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var e controlplane.RouteEntry
		if err := rows.Scan(&e.User, &e.Bucket, &e.Storage); err != nil {
			rows.Close()
			return res, err
		}
		res.Routes = append(res.Routes, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	rows, err = r.rt.q().Query(ctx, `SELECT user_name, bucket FROM route_blocks ORDER BY user_name, bucket`)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var e controlplane.RouteEntry
		if err := rows.Scan(&e.User, &e.Bucket); err != nil {
			return res, err
		}
		res.Blocks = append(res.Blocks, e)
	}
	return res, rows.Err()
}
