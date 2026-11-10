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

type dynCreds struct{ rt *runner }

func credDoc(c controlplane.DynamicCredential) []byte {
	b, _ := json.Marshal(map[string]string{"storageType": c.StorageType, "storage": c.Storage, "user": c.User, "sealed": c.Sealed})
	return b
}

func (d *dynCreds) Set(ctx context.Context, c controlplane.DynamicCredential) error {
	if c.StorageType == "" || c.Storage == "" || c.User == "" || c.Sealed == "" {
		return fmt.Errorf("%w: dynamic credential: storage type, storage, user and value are required", dom.ErrInvalidArg)
	}
	return d.rt.write(ctx, func(tx pgx.Tx) error {
		var prev *string
		err := tx.QueryRow(ctx, `SELECT sealed FROM dynamic_credentials WHERE storage_type=$1 AND storage=$2 AND user_name=$3 FOR UPDATE`,
			c.StorageType, c.Storage, c.User).Scan(&prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO dynamic_credentials (storage_type, storage, user_name, sealed) VALUES ($1,$2,$3,$4)
			ON CONFLICT (storage_type, storage, user_name) DO UPDATE SET sealed=EXCLUDED.sealed, version=dynamic_credentials.version+1, updated_at=now()`,
			c.StorageType, c.Storage, c.User, c.Sealed); err != nil {
			return fmt.Errorf("controlplane: set dynamic credential: %w", err)
		}
		m := mutation{action: "set", objectType: "dynamic_credentials", objectID: c.RedisField(), after: credDoc(c),
			ops: []RedisOp{{Op: OpHSet, Key: KeyDynamicCreds, Field: c.RedisField(), Value: c.Sealed}, {Op: OpHIncr, Key: KeyDynamicCreds, Field: "version"}}}
		if prev != nil {
			old := c
			old.Sealed = *prev
			m.before = credDoc(old)
		}
		return record(ctx, tx, m)
	})
}

func (d *dynCreds) Get(ctx context.Context, storageType, storage, user string) (controlplane.DynamicCredential, error) {
	c := controlplane.DynamicCredential{StorageType: storageType, Storage: storage, User: user}
	err := d.rt.q().QueryRow(ctx, `SELECT sealed FROM dynamic_credentials WHERE storage_type=$1 AND storage=$2 AND user_name=$3`,
		storageType, storage, user).Scan(&c.Sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("%w: dynamic credential %s", dom.ErrNotFound, c.RedisField())
	}
	return c, err
}

func (d *dynCreds) Delete(ctx context.Context, storage, user string) error {
	return d.rt.write(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `DELETE FROM dynamic_credentials WHERE storage=$1 AND user_name=$2 RETURNING storage_type`, storage, user)
		if err != nil {
			return err
		}
		var types []string
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				rows.Close()
				return err
			}
			types = append(types, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		ops := []RedisOp{{Op: OpHIncr, Key: KeyDynamicCreds, Field: "version"}}
		for _, t := range []string{"S3", "SWIFT"} {
			ops = append(ops, opHDel(KeyDynamicCreds, t+":"+storage+":"+user))
		}
		for _, t := range types {
			if err := record(ctx, tx, mutation{action: "delete", objectType: "dynamic_credentials", objectID: t + ":" + storage + ":" + user,
				before: credDoc(controlplane.DynamicCredential{StorageType: t, Storage: storage, User: user, Sealed: "x"}), ops: ops}); err != nil {
				return err
			}
			ops = nil // one projection is enough
		}
		return nil
	})
}

func (d *dynCreds) List(ctx context.Context) ([]controlplane.DynamicCredential, error) {
	rows, err := d.rt.q().Query(ctx, `SELECT storage_type, storage, user_name, sealed FROM dynamic_credentials ORDER BY storage_type, storage, user_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []controlplane.DynamicCredential
	for rows.Next() {
		var c controlplane.DynamicCredential
		if err := rows.Scan(&c.StorageType, &c.Storage, &c.User, &c.Sealed); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, rows.Err()
}

func (d *dynCreds) Salt(ctx context.Context) (string, bool, error) {
	var s string
	err := d.rt.q().QueryRow(ctx, `SELECT value FROM dynamic_credentials_meta WHERE name='salt'`).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return s, err == nil, err
}

func (d *dynCreds) SetSaltIfAbsent(ctx context.Context, salt string) (bool, error) {
	var stored bool
	err := d.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO dynamic_credentials_meta (name, value) VALUES ('salt',$1) ON CONFLICT DO NOTHING`, salt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		stored = true
		b, _ := json.Marshal(map[string]string{"salt": salt})
		return record(ctx, tx, mutation{action: "create", objectType: "dynamic_credentials_salt", objectID: "salt", after: b,
			ops: []RedisOp{{Op: OpHSetNX, Key: KeyDynamicCreds, Field: "salt", Value: salt}}})
	})
	return stored, err
}

func (d *dynCreds) Fingerprint(ctx context.Context) (string, error) {
	var fp string
	err := d.rt.q().QueryRow(ctx, `SELECT coalesce(md5(string_agg(storage_type || ':' || storage || ':' || user_name || ':' || version::text, ',' ORDER BY storage_type, storage, user_name)), 'empty') FROM dynamic_credentials`).Scan(&fp)
	return fp, err
}
