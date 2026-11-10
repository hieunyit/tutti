// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

// kvSpec describes one document table (k text PRIMARY KEY, doc jsonb, version, created_at, updated_at).
type kvSpec[T any] struct {
	table      string // also the audit object type
	label      string // for error messages
	key        func(*T) string
	setKey     func(*T, string)
	normalize  func(*T) error                   // validates and canonicalizes before storing
	project    func(key string, v *T) []RedisOp // v == nil: deleted
	extraCol   string                           // optional additional nullable timestamptz column merged into reads
	applyExtra func(*T, *time.Time)
}

type kv[T any] struct {
	rt   *runner
	spec kvSpec[T]
}

func (r *kv[T]) cols() string {
	c := "doc, version, created_at, updated_at"
	if r.spec.extraCol != "" {
		c += ", " + r.spec.extraCol
	}
	return c
}

type scanner interface{ Scan(...any) error }

func (r *kv[T]) scan(key string, row scanner) (controlplane.Versioned[T], error) {
	var (
		out   controlplane.Versioned[T]
		doc   []byte
		extra *time.Time
	)
	dest := []any{&doc, &out.Version, &out.CreatedAt, &out.UpdatedAt}
	if r.spec.extraCol != "" {
		dest = append(dest, &extra)
	}
	if err := row.Scan(dest...); err != nil {
		return out, err
	}
	if err := json.Unmarshal(doc, &out.Value); err != nil {
		return out, fmt.Errorf("%w: stored %s %q is corrupted: %v", dom.ErrInternal, r.spec.label, key, err)
	}
	r.spec.setKey(&out.Value, key)
	if r.spec.applyExtra != nil {
		r.spec.applyExtra(&out.Value, extra)
	}
	return out, nil
}

func (r *kv[T]) notFound(key string) error {
	return fmt.Errorf("%w: %s %q", dom.ErrNotFound, r.spec.label, key)
}

func (r *kv[T]) prepare(v *T) (string, []byte, error) {
	if r.spec.normalize != nil {
		if err := r.spec.normalize(v); err != nil {
			return "", nil, err
		}
	}
	key := r.spec.key(v)
	if key == "" {
		return "", nil, fmt.Errorf("%w: %s: key is required", dom.ErrInvalidArg, r.spec.label)
	}
	doc, err := json.Marshal(v)
	if err != nil {
		return "", nil, err
	}
	return key, doc, nil
}

func (r *kv[T]) Get(ctx context.Context, key string) (controlplane.Versioned[T], error) {
	row := r.rt.q().QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE k=$1`, r.cols(), r.spec.table), key)
	v, err := r.scan(key, row)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, r.notFound(key)
	}
	if err != nil {
		return v, fmt.Errorf("controlplane: get %s: %w", r.spec.label, err)
	}
	return v, nil
}

func (r *kv[T]) List(ctx context.Context) ([]controlplane.Versioned[T], error) {
	rows, err := r.rt.q().Query(ctx, fmt.Sprintf(`SELECT k, %s FROM %s ORDER BY k`, r.cols(), r.spec.table))
	if err != nil {
		return nil, fmt.Errorf("controlplane: list %s: %w", r.spec.label, err)
	}
	defer rows.Close()
	var res []controlplane.Versioned[T]
	for rows.Next() {
		var key string
		var out controlplane.Versioned[T]
		var doc []byte
		var extra *time.Time
		dest := []any{&key, &doc, &out.Version, &out.CreatedAt, &out.UpdatedAt}
		if r.spec.extraCol != "" {
			dest = append(dest, &extra)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(doc, &out.Value); err != nil {
			return nil, fmt.Errorf("%w: stored %s %q is corrupted: %v", dom.ErrInternal, r.spec.label, key, err)
		}
		r.spec.setKey(&out.Value, key)
		if r.spec.applyExtra != nil {
			r.spec.applyExtra(&out.Value, extra)
		}
		res = append(res, out)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(res, func(i, j int) bool { return r.spec.key(&res[i].Value) < r.spec.key(&res[j].Value) })
	return res, nil
}

func (r *kv[T]) Create(ctx context.Context, v T) (controlplane.Versioned[T], error) {
	key, doc, err := r.prepare(&v)
	if err != nil {
		return controlplane.Versioned[T]{}, err
	}
	out := controlplane.Versioned[T]{Value: v}
	err = r.rt.write(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s (k, doc) VALUES ($1,$2) ON CONFLICT (k) DO NOTHING
			RETURNING version, created_at, updated_at`, r.spec.table), key, json.RawMessage(doc)).Scan(&out.Version, &out.CreatedAt, &out.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %s %q", dom.ErrAlreadyExists, r.spec.label, key)
		}
		if err != nil {
			return fmt.Errorf("controlplane: create %s: %w", r.spec.label, err)
		}
		return record(ctx, tx, mutation{action: "create", objectType: r.spec.table, objectID: key, after: doc, ops: r.project(key, &v)})
	})
	return out, err
}

func (r *kv[T]) project(key string, v *T) []RedisOp {
	if r.spec.project == nil {
		return nil
	}
	return r.spec.project(key, v)
}

// lock loads the current document of key FOR UPDATE.
func (r *kv[T]) lock(ctx context.Context, tx pgx.Tx, key string) (doc []byte, version int64, found bool, err error) {
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT doc, version FROM %s WHERE k=$1 FOR UPDATE`, r.spec.table), key).Scan(&doc, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, false, nil
	}
	return doc, version, err == nil, err
}

// store writes doc to an existing locked row, bumping the version.
func (r *kv[T]) store(ctx context.Context, tx pgx.Tx, key string, v *T, doc, before []byte) (controlplane.Versioned[T], error) {
	out := controlplane.Versioned[T]{Value: *v}
	err := tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s SET doc=$2, version=version+1, updated_at=now() WHERE k=$1
		RETURNING version, created_at, updated_at`, r.spec.table), key, json.RawMessage(doc)).Scan(&out.Version, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return out, fmt.Errorf("controlplane: update %s: %w", r.spec.label, err)
	}
	return out, record(ctx, tx, mutation{action: "update", objectType: r.spec.table, objectID: key, before: before, after: doc, ops: r.project(key, v)})
}

func (r *kv[T]) Put(ctx context.Context, v T) (controlplane.Versioned[T], error) {
	key, doc, err := r.prepare(&v)
	if err != nil {
		return controlplane.Versioned[T]{}, err
	}
	var out controlplane.Versioned[T]
	err = r.rt.write(ctx, func(tx pgx.Tx) error {
		before, _, found, err := r.lock(ctx, tx, key)
		if err != nil {
			return err
		}
		if found {
			out, err = r.store(ctx, tx, key, &v, doc, before)
			return err
		}
		out = controlplane.Versioned[T]{Value: v}
		// a concurrent Put of the same new key conflicts on the primary key: the loser waits for the winner and updates
		err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s (k, doc) VALUES ($1,$2)
			ON CONFLICT (k) DO UPDATE SET doc=EXCLUDED.doc, version=%s.version+1, updated_at=now()
			RETURNING version, created_at, updated_at`, r.spec.table, r.spec.table), key, json.RawMessage(doc)).Scan(&out.Version, &out.CreatedAt, &out.UpdatedAt)
		if err != nil {
			return fmt.Errorf("controlplane: put %s: %w", r.spec.label, err)
		}
		return record(ctx, tx, mutation{action: "create", objectType: r.spec.table, objectID: key, after: doc, ops: r.project(key, &v)})
	})
	return out, err
}

func (r *kv[T]) Update(ctx context.Context, v T, expectedVersion int64) (controlplane.Versioned[T], error) {
	key, doc, err := r.prepare(&v)
	if err != nil {
		return controlplane.Versioned[T]{}, err
	}
	var out controlplane.Versioned[T]
	err = r.rt.write(ctx, func(tx pgx.Tx) error {
		before, version, found, err := r.lock(ctx, tx, key)
		if err != nil {
			return err
		}
		if !found {
			return r.notFound(key)
		}
		if version != expectedVersion {
			return fmt.Errorf("%w: %s %q is at version %d, expected %d", controlplane.ErrVersionConflict, r.spec.label, key, version, expectedVersion)
		}
		out, err = r.store(ctx, tx, key, &v, doc, before)
		return err
	})
	return out, err
}

func (r *kv[T]) Mutate(ctx context.Context, key string, fn func(*T) error) (controlplane.Versioned[T], error) {
	var out controlplane.Versioned[T]
	err := r.rt.write(ctx, func(tx pgx.Tx) error {
		before, _, found, err := r.lock(ctx, tx, key)
		if err != nil {
			return err
		}
		if !found {
			return r.notFound(key)
		}
		var v T
		if err := json.Unmarshal(before, &v); err != nil {
			return fmt.Errorf("%w: stored %s %q is corrupted: %v", dom.ErrInternal, r.spec.label, key, err)
		}
		r.spec.setKey(&v, key)
		if err := fn(&v); err != nil {
			return err
		}
		r.spec.setKey(&v, key) // the key is immutable
		_, doc, err := r.prepare(&v)
		if err != nil {
			return err
		}
		out, err = r.store(ctx, tx, key, &v, doc, before)
		return err
	})
	return out, err
}

func (r *kv[T]) Delete(ctx context.Context, key string) error {
	return r.rt.write(ctx, func(tx pgx.Tx) error {
		return r.deleteTx(ctx, tx, key)
	})
}

func (r *kv[T]) deleteTx(ctx context.Context, tx pgx.Tx, key string) error {
	before, _, found, err := r.lock(ctx, tx, key)
	if err != nil {
		return err
	}
	if !found {
		return r.notFound(key)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE k=$1`, r.spec.table), key); err != nil {
		return fmt.Errorf("controlplane: delete %s: %w", r.spec.label, err)
	}
	return record(ctx, tx, mutation{action: "delete", objectType: r.spec.table, objectID: key, before: before, ops: r.project(key, nil)})
}
