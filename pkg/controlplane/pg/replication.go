// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

// Redis keys of the replication stores (pkg/store/replication_*.go). "<k>:<user>" means a per-user key.
const (
	KeyReplUserFrom          = "p:repl_user_from:" // trailing ":" like the routing keys
	KeyReplUserTo            = "p:repl_user_to"
	KeyReplUserGen           = "p:repl_user_gen"
	KeyReplBucketFrom        = "p:repl_bucket_from"
	KeyReplBucketTo          = "p:repl_bucket_to"
	KeyReplBucketDestIndex   = "p:repl_bucket_to_dest_index"
	KeyReplBucketGen         = "p:repl_bucket_gen"
	KeyReplUserStatusIndex   = "p:repl_user_status_index:"
	KeyReplUserStatus        = "p:repl_status_user"
	KeyReplBucketStatusIndex = "p:repl_bucket_status_index"
	KeyReplBucketStatus      = "p:repl_status_bucket"
)

type replications struct{ rt *runner }

func (r *replications) check(id controlplane.ReplicationID) error {
	if id.FromStorage == "" || id.ToStorage == "" {
		return fmt.Errorf("%w: replication: from and to storage are required", dom.ErrInvalidArg)
	}
	if (id.FromBucket == "") != (id.ToBucket == "") {
		return fmt.Errorf("%w: replication: from and to bucket must be both set or both empty", dom.ErrInvalidArg)
	}
	if id.IsUserLevel() && id.FromStorage == id.ToStorage {
		return fmt.Errorf("%w: replication: from/to storages should differ", dom.ErrInvalidArg)
	}
	return nil
}

func dest(toStorage, toBucket string) string { return toStorage + ":" + toBucket }

// replScopeKey is the outbox object key of every replication entry of one user.
// The projection of a scope (destination set, generations, per-user index) is absolute, so all entries of the user
// must be applied in commit order: one key per user, never one key per destination or bucket.
func replScopeKey(user, _ string) string {
	return "replication_user/" + user
}

// lockScope serializes the writers of one replication scope until the end of the transaction. The absolute
// projection computed by scopeOps is read after the lock is held, so under READ COMMITTED it includes every
// committed change of the scope and concurrent Add/Remove/generation changes cannot compute from stale state.
func lockScope(ctx context.Context, tx pgx.Tx, user, fromBucket string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, replScopeKey(user, fromBucket))
	return err
}

// scopeOps rebuilds the Redis lookup keys of one scope (user-level when fromBucket is empty) from the PostgreSQL
// state, so that the projection is absolute and idempotent whatever order the entries are applied in.
func scopeOps(ctx context.Context, tx pgx.Tx, user, fromBucket string) ([]RedisOp, error) {
	rows, err := tx.Query(ctx, `SELECT from_storage, to_storage, to_bucket FROM replication_policies WHERE user_name=$1 AND from_bucket=$2 ORDER BY to_storage, to_bucket`, user, fromBucket)
	if err != nil {
		return nil, err
	}
	var from string
	var tos []string
	for rows.Next() {
		var f, ts, tb string
		if err := rows.Scan(&f, &ts, &tb); err != nil {
			rows.Close()
			return nil, err
		}
		from = f
		if fromBucket == "" {
			tos = append(tos, ts)
		} else {
			tos = append(tos, dest(ts, tb))
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	gens := map[string]string{}
	grows, err := tx.Query(ctx, `SELECT to_storage, to_bucket, generation FROM replication_generations WHERE user_name=$1 AND from_bucket=$2`, user, fromBucket)
	if err != nil {
		return nil, err
	}
	for grows.Next() {
		var ts, tb string
		var g int64
		if err := grows.Scan(&ts, &tb, &g); err != nil {
			grows.Close()
			return nil, err
		}
		field := ts
		if fromBucket != "" {
			field = dest(ts, tb)
		}
		gens[field] = fmt.Sprint(g)
	}
	grows.Close()
	if err := grows.Err(); err != nil {
		return nil, err
	}
	var ops []RedisOp
	if fromBucket == "" {
		fromKey, toKey, genKey := KeyReplUserFrom, KeyReplUserTo+":"+user, KeyReplUserGen+":"+user
		if len(tos) == 0 {
			ops = append(ops, opHDel(fromKey, user), RedisOp{Op: OpDel, Key: toKey})
		} else {
			ops = append(ops, RedisOp{Op: OpHSet, Key: fromKey, Field: user, Value: from}, RedisOp{Op: OpSReplace, Key: toKey, Members: tos})
		}
		ops = append(ops, RedisOp{Op: OpHReplace, Key: genKey, Fields: gens})
		return append(ops, opSnapshot()), nil
	}
	fromKey, toKey, genKey := KeyReplBucketFrom+":"+user, KeyReplBucketTo+":"+user+":"+fromBucket, KeyReplBucketGen+":"+user+":"+fromBucket
	if len(tos) == 0 {
		ops = append(ops, opHDel(fromKey, fromBucket), RedisOp{Op: OpDel, Key: toKey})
	} else {
		ops = append(ops, RedisOp{Op: OpHSet, Key: fromKey, Field: fromBucket, Value: from}, RedisOp{Op: OpSReplace, Key: toKey, Members: tos})
	}
	ops = append(ops, RedisOp{Op: OpHReplace, Key: genKey, Fields: gens})
	// the per-user index of all destinations
	drows, err := tx.Query(ctx, `SELECT to_storage, to_bucket FROM replication_policies WHERE user_name=$1 AND from_bucket<>'' ORDER BY to_storage, to_bucket`, user)
	if err != nil {
		return nil, err
	}
	var all []string
	for drows.Next() {
		var ts, tb string
		if err := drows.Scan(&ts, &tb); err != nil {
			drows.Close()
			return nil, err
		}
		all = append(all, dest(ts, tb))
	}
	drows.Close()
	if err := drows.Err(); err != nil {
		return nil, err
	}
	ops = append(ops, RedisOp{Op: OpSReplace, Key: KeyReplBucketDestIndex + ":" + user, Members: all})
	return append(ops, opSnapshot()), nil
}

// statusKeys returns the status hash key, the index key and the index member. The user-level index holds the JSON of
// entity.UserReplicationPolicy (UserReplicationPolicyToStringConverter), the bucket-level one the universal id.
func statusKeys(id controlplane.ReplicationID) (hashKey, indexKey, member string) {
	if id.IsUserLevel() {
		m, _ := json.Marshal(struct{ User, FromStorage, ToStorage string }{id.User, id.FromStorage, id.ToStorage})
		return KeyReplUserStatus + ":" + id.String(), KeyReplUserStatusIndex, string(m)
	}
	return KeyReplBucketStatus + ":" + id.String(), KeyReplBucketStatusIndex + ":" + id.User, id.String()
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// statusFields encodes the config fields like the Redis store writes entity.ReplicationStatus (go-redis encoding:
// RFC 3339 times, "1"/"0" bools; empty optional fields are absent). It returns the fields to set and those to remove.
func statusFields(s controlplane.ReplicationStatus) (set map[string]string, del []string) {
	set = map[string]string{"created_at": s.CreatedAt.UTC().Format(time.RFC3339Nano), "archived": boolStr(s.Archived)}
	opt := func(name, v string) {
		if v == "" {
			del = append(del, name)
		} else {
			set[name] = v
		}
	}
	if s.ArchivedAt != nil {
		opt("archived_at", s.ArchivedAt.UTC().Format(time.RFC3339Nano))
	} else {
		opt("archived_at", "")
	}
	opt("agent_url", s.AgentURL)
	opt("event_source", s.EventSource)
	return set, del
}

func statusOps(id controlplane.ReplicationID, s controlplane.ReplicationStatus) []RedisOp {
	h, idx, member := statusKeys(id)
	set, del := statusFields(s)
	return []RedisOp{{Op: OpHMSet, Key: h, Fields: set, Del: del}, {Op: OpSAdd, Key: idx, Members: []string{member}}}
}

func replDoc(id controlplane.ReplicationID, extra map[string]any) []byte {
	m := map[string]any{"id": id.String(), "user": id.User, "fromStorage": id.FromStorage, "toStorage": id.ToStorage, "fromBucket": id.FromBucket, "toBucket": id.ToBucket}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

func statusDoc(s controlplane.ReplicationStatus) map[string]any {
	return map[string]any{"status": map[string]any{"createdAt": s.CreatedAt, "archived": s.Archived, "archivedAt": s.ArchivedAt, "agentUrl": s.AgentURL, "eventSource": s.EventSource}}
}

func (r *replications) Add(ctx context.Context, id controlplane.ReplicationID, st controlplane.ReplicationStatus) error {
	if err := r.check(id); err != nil {
		return err
	}
	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now()
	}
	st.CreatedAt = st.CreatedAt.UTC()
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		if err := lockScope(ctx, tx, id.User, id.FromBucket); err != nil {
			return err
		}
		if same, err := unchanged(ctx, tx, id, st); err != nil || same {
			return err
		}
		// the source is single-valued per scope (Redis: one HSET field), a new source replaces the old one
		if _, err := tx.Exec(ctx, `UPDATE replication_policies SET from_storage=$3 WHERE user_name=$1 AND from_bucket=$2`, id.User, id.FromBucket, id.FromStorage); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO replication_policies (user_name, from_bucket, to_storage, to_bucket, from_storage) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (user_name, from_bucket, to_storage, to_bucket) DO UPDATE SET from_storage=EXCLUDED.from_storage`,
			id.User, id.FromBucket, id.ToStorage, id.ToBucket, id.FromStorage); err != nil {
			return err
		}
		if err := upsertStatus(ctx, tx, id, st); err != nil {
			return err
		}
		ops, err := scopeOps(ctx, tx, id.User, id.FromBucket)
		if err != nil {
			return err
		}
		return record(ctx, tx, mutation{action: "create", objectType: "replication", objectID: id.String(), after: replDoc(id, statusDoc(st)), ops: append(ops, statusOps(id, st)...), key: replScopeKey(id.User, id.FromBucket)})
	})
}

// unchanged reports whether the policy and the status are stored exactly like this already (re-running a migration
// must not write audit rows or outbox entries).
func unchanged(ctx context.Context, tx pgx.Tx, id controlplane.ReplicationID, st controlplane.ReplicationStatus) (bool, error) {
	var from string
	err := tx.QueryRow(ctx, `SELECT from_storage FROM replication_policies WHERE user_name=$1 AND from_bucket=$2 AND to_storage=$3 AND to_bucket=$4`,
		id.User, id.FromBucket, id.ToStorage, id.ToBucket).Scan(&from)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && from != id.FromStorage) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return sameStatus(ctx, tx, id, st)
}

func sameStatus(ctx context.Context, tx pgx.Tx, id controlplane.ReplicationID, st controlplane.ReplicationStatus) (bool, error) {
	var cur controlplane.ReplicationStatus
	err := tx.QueryRow(ctx, `SELECT created_at, archived, archived_at, agent_url, event_source FROM replication_status
		WHERE user_name=$1 AND from_storage=$2 AND to_storage=$3 AND from_bucket=$4 AND to_bucket=$5`,
		id.User, id.FromStorage, id.ToStorage, id.FromBucket, id.ToBucket).Scan(&cur.CreatedAt, &cur.Archived, &cur.ArchivedAt, &cur.AgentURL, &cur.EventSource)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return cur.CreatedAt.Equal(st.CreatedAt) && cur.Archived == st.Archived && cur.AgentURL == st.AgentURL && cur.EventSource == st.EventSource &&
		(cur.ArchivedAt == nil) == (st.ArchivedAt == nil) && (cur.ArchivedAt == nil || cur.ArchivedAt.Equal(*st.ArchivedAt)), nil
}

func (r *replications) PutStatus(ctx context.Context, id controlplane.ReplicationID, st controlplane.ReplicationStatus) error {
	if err := r.check(id); err != nil {
		return err
	}
	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now()
	}
	st.CreatedAt = st.CreatedAt.UTC()
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		if err := lockScope(ctx, tx, id.User, id.FromBucket); err != nil {
			return err
		}
		if same, err := sameStatus(ctx, tx, id, st); err != nil || same {
			return err
		}
		if err := upsertStatus(ctx, tx, id, st); err != nil {
			return err
		}
		return record(ctx, tx, mutation{action: "set", objectType: "replication_status", objectID: id.String(), after: replDoc(id, statusDoc(st)), ops: statusOps(id, st), key: replScopeKey(id.User, id.FromBucket)})
	})
}

func upsertStatus(ctx context.Context, tx pgx.Tx, id controlplane.ReplicationID, st controlplane.ReplicationStatus) error {
	_, err := tx.Exec(ctx, `INSERT INTO replication_status (user_name, from_storage, to_storage, from_bucket, to_bucket, created_at, archived, archived_at, agent_url, event_source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (user_name, from_storage, to_storage, from_bucket, to_bucket) DO UPDATE SET created_at=EXCLUDED.created_at, archived=EXCLUDED.archived,
		archived_at=EXCLUDED.archived_at, agent_url=EXCLUDED.agent_url, event_source=EXCLUDED.event_source`,
		id.User, id.FromStorage, id.ToStorage, id.FromBucket, id.ToBucket, st.CreatedAt, st.Archived, st.ArchivedAt, st.AgentURL, st.EventSource)
	return err
}

func (r *replications) Remove(ctx context.Context, id controlplane.ReplicationID) error {
	if err := r.check(id); err != nil {
		return err
	}
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		if err := lockScope(ctx, tx, id.User, id.FromBucket); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM replication_policies WHERE user_name=$1 AND from_bucket=$2 AND to_storage=$3 AND to_bucket=$4`, id.User, id.FromBucket, id.ToStorage, id.ToBucket)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		ops, err := scopeOps(ctx, tx, id.User, id.FromBucket)
		if err != nil {
			return err
		}
		return record(ctx, tx, mutation{action: "delete", objectType: "replication", objectID: id.String(), before: replDoc(id, nil), ops: ops, key: replScopeKey(id.User, id.FromBucket)})
	})
}

func (r *replications) Archive(ctx context.Context, id controlplane.ReplicationID) error {
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		if err := lockScope(ctx, tx, id.User, id.FromBucket); err != nil {
			return err
		}
		var st controlplane.ReplicationStatus
		err := tx.QueryRow(ctx, `UPDATE replication_status SET archived=true, archived_at=now() WHERE user_name=$1 AND from_storage=$2 AND to_storage=$3 AND from_bucket=$4 AND to_bucket=$5
			RETURNING created_at, archived, archived_at, agent_url, event_source`, id.User, id.FromStorage, id.ToStorage, id.FromBucket, id.ToBucket).
			Scan(&st.CreatedAt, &st.Archived, &st.ArchivedAt, &st.AgentURL, &st.EventSource)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: replication status %q", dom.ErrNotFound, id.String())
		}
		if err != nil {
			return err
		}
		return record(ctx, tx, mutation{action: "archive", objectType: "replication_status", objectID: id.String(), after: replDoc(id, statusDoc(st)), ops: statusOps(id, st), key: replScopeKey(id.User, id.FromBucket)})
	})
}

func (r *replications) DeleteStatus(ctx context.Context, id controlplane.ReplicationID) error {
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		if err := lockScope(ctx, tx, id.User, id.FromBucket); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM replication_status WHERE user_name=$1 AND from_storage=$2 AND to_storage=$3 AND from_bucket=$4 AND to_bucket=$5`,
			id.User, id.FromStorage, id.ToStorage, id.FromBucket, id.ToBucket)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		h, idx, member := statusKeys(id)
		return record(ctx, tx, mutation{action: "delete", objectType: "replication_status", objectID: id.String(), before: replDoc(id, nil),
			ops: []RedisOp{{Op: OpDel, Key: h}, {Op: OpSRem, Key: idx, Members: []string{member}}}, key: replScopeKey(id.User, id.FromBucket)})
	})
}

func (r *replications) GetStatus(ctx context.Context, id controlplane.ReplicationID) (controlplane.ReplicationStatus, error) {
	var st controlplane.ReplicationStatus
	err := r.rt.q().QueryRow(ctx, `SELECT created_at, archived, archived_at, agent_url, event_source FROM replication_status
		WHERE user_name=$1 AND from_storage=$2 AND to_storage=$3 AND from_bucket=$4 AND to_bucket=$5`,
		id.User, id.FromStorage, id.ToStorage, id.FromBucket, id.ToBucket).Scan(&st.CreatedAt, &st.Archived, &st.ArchivedAt, &st.AgentURL, &st.EventSource)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, fmt.Errorf("%w: replication status %q", dom.ErrNotFound, id.String())
	}
	return st, err
}

func (r *replications) ListStatuses(ctx context.Context, user string) (map[controlplane.ReplicationID]controlplane.ReplicationStatus, error) {
	q := `SELECT user_name, from_storage, to_storage, from_bucket, to_bucket, created_at, archived, archived_at, agent_url, event_source FROM replication_status`
	var args []any
	if user != "" {
		q += ` WHERE user_name=$1`
		args = append(args, user)
	}
	rows, err := r.rt.q().Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[controlplane.ReplicationID]controlplane.ReplicationStatus{}
	for rows.Next() {
		var id controlplane.ReplicationID
		var st controlplane.ReplicationStatus
		if err := rows.Scan(&id.User, &id.FromStorage, &id.ToStorage, &id.FromBucket, &id.ToBucket, &st.CreatedAt, &st.Archived, &st.ArchivedAt, &st.AgentURL, &st.EventSource); err != nil {
			return nil, err
		}
		res[id] = st
	}
	return res, rows.Err()
}

func (r *replications) Policies(ctx context.Context, user, fromBucket string) ([]controlplane.ReplicationID, error) {
	rows, err := r.rt.q().Query(ctx, `SELECT from_storage, to_storage, to_bucket FROM replication_policies WHERE user_name=$1 AND from_bucket=$2 ORDER BY to_storage, to_bucket`, user, fromBucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []controlplane.ReplicationID
	for rows.Next() {
		id := controlplane.ReplicationID{User: user, FromBucket: fromBucket}
		if err := rows.Scan(&id.FromStorage, &id.ToStorage, &id.ToBucket); err != nil {
			return nil, err
		}
		res = append(res, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("%w: replication of %q", dom.ErrNotFound, scopeID(user, fromBucket))
	}
	return res, nil
}

func (r *replications) ListAll(ctx context.Context) ([]controlplane.ReplicationID, error) {
	rows, err := r.rt.q().Query(ctx, `SELECT user_name, from_storage, from_bucket, to_storage, to_bucket FROM replication_policies ORDER BY user_name, from_bucket, to_storage, to_bucket`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []controlplane.ReplicationID
	for rows.Next() {
		var id controlplane.ReplicationID
		if err := rows.Scan(&id.User, &id.FromStorage, &id.FromBucket, &id.ToStorage, &id.ToBucket); err != nil {
			return nil, err
		}
		res = append(res, id)
	}
	return res, rows.Err()
}

func (r *replications) BucketReplicationExistsForUser(ctx context.Context, user string) (bool, error) {
	var ok bool
	err := r.rt.q().QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM replication_policies WHERE user_name=$1 AND from_bucket<>'')`, user).Scan(&ok)
	return ok, err
}

func (r *replications) IsDestinationInUse(ctx context.Context, user, toStorage, toBucket string) (bool, error) {
	var ok bool
	err := r.rt.q().QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM replication_policies WHERE user_name=$1 AND from_bucket<>'' AND to_storage=$2 AND to_bucket=$3)`, user, toStorage, toBucket).Scan(&ok)
	return ok, err
}

func (r *replications) Generation(ctx context.Context, id controlplane.ReplicationID) (int64, error) {
	var g int64
	err := r.rt.q().QueryRow(ctx, `SELECT generation FROM replication_generations WHERE user_name=$1 AND from_bucket=$2 AND to_storage=$3 AND to_bucket=$4`,
		id.User, id.FromBucket, id.ToStorage, id.ToBucket).Scan(&g)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil // missing generation equals 0
	}
	return g, err
}

func (r *replications) IncrementGeneration(ctx context.Context, id controlplane.ReplicationID) (int64, error) {
	return r.setGen(ctx, id, 0, true)
}

func (r *replications) SetGeneration(ctx context.Context, id controlplane.ReplicationID, generation int64) error {
	_, err := r.setGen(ctx, id, generation, false)
	return err
}

func (r *replications) setGen(ctx context.Context, id controlplane.ReplicationID, value int64, incr bool) (int64, error) {
	// generations are keyed without the source storage
	if id.ToStorage == "" || (id.FromBucket == "") != (id.ToBucket == "") {
		return 0, fmt.Errorf("%w: replication generation: destination is required", dom.ErrInvalidArg)
	}
	var g int64
	err := r.rt.write(ctx, func(tx pgx.Tx) error {
		if err := lockScope(ctx, tx, id.User, id.FromBucket); err != nil {
			return err
		}
		var prev int64
		err := tx.QueryRow(ctx, `SELECT generation FROM replication_generations WHERE user_name=$1 AND from_bucket=$2 AND to_storage=$3 AND to_bucket=$4 FOR UPDATE`,
			id.User, id.FromBucket, id.ToStorage, id.ToBucket).Scan(&prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		g = value
		if incr {
			g = prev + 1
		}
		if !incr && err == nil && g == prev {
			return nil // unchanged
		}
		if _, err := tx.Exec(ctx, `INSERT INTO replication_generations (user_name, from_bucket, to_storage, to_bucket, generation) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (user_name, from_bucket, to_storage, to_bucket) DO UPDATE SET generation=EXCLUDED.generation`,
			id.User, id.FromBucket, id.ToStorage, id.ToBucket, g); err != nil {
			return err
		}
		ops, err := scopeOps(ctx, tx, id.User, id.FromBucket)
		if err != nil {
			return err
		}
		return record(ctx, tx, mutation{action: "generation", objectType: "replication_generation", objectID: id.String(),
			before: replDoc(id, map[string]any{"generation": prev}), after: replDoc(id, map[string]any{"generation": g}), ops: ops, key: replScopeKey(id.User, id.FromBucket)})
	})
	return g, err
}

func (r *replications) ListGenerations(ctx context.Context) (map[controlplane.ReplicationID]int64, error) {
	rows, err := r.rt.q().Query(ctx, `SELECT user_name, from_bucket, to_storage, to_bucket, generation FROM replication_generations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[controlplane.ReplicationID]int64{}
	for rows.Next() {
		var id controlplane.ReplicationID
		var g int64
		if err := rows.Scan(&id.User, &id.FromBucket, &id.ToStorage, &id.ToBucket, &g); err != nil {
			return nil, err
		}
		res[id] = g
	}
	return res, rows.Err()
}
