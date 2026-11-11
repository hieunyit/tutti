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

// Redacted is what secret values are replaced with in audit rows.
const Redacted = "***"

// secretKeys are JSON keys whose non-empty string values never reach the audit log. The values are sealed already,
// but the audit log is shown in the UI and must not allow offline attacks on the sealed blobs or password hashes.
var secretKeys = map[string]bool{
	"secret": true, "previoussecret": true, "connectionaccount": true, "previousaccount": true,
	"passwordhash": true, "sealed": true, "secretaccesskey": true, "accesssecret": true, "password": true, "token": true,
	"salt": true,
}

// Redact returns the JSON document with the values of secret keys replaced by Redacted.
func Redact(doc []byte) []byte {
	if len(doc) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return []byte(`"` + Redacted + `"`) // not JSON: never store it
	}
	out, err := json.Marshal(redactValue(v))
	if err != nil {
		return []byte(`"` + Redacted + `"`)
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if s, ok := x.(string); ok && secretKeys[strings.ToLower(k)] {
				if s != "" {
					t[k] = Redacted
				}
				continue
			}
			t[k] = redactValue(x)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
		return t
	}
	return v
}

// mutation describes one change for the audit log and the outbox.
type mutation struct {
	action     string
	objectType string
	objectID   string
	before     any // marshaled; nil = absent
	after      any
	ops        []RedisOp
	// noAudit skips the audit row (high-frequency, low-value results such as storage connection tests); the outbox
	// entry is still written.
	noAudit bool
	// key overrides the outbox object key (default objectType/objectID); entries of one key are applied in order.
	key string
}

func marshalOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case json.RawMessage:
		return t, nil
	case []byte:
		return t, nil
	}
	return json.Marshal(v)
}

// record writes the audit row and the outbox entry in the caller's transaction.
func record(ctx context.Context, tx pgx.Tx, m mutation) error {
	b, err := marshalOrNil(m.before)
	if err != nil {
		return err
	}
	a, err := marshalOrNil(m.after)
	if err != nil {
		return err
	}
	if !m.noAudit {
		act := controlplane.ActorFrom(ctx)
		var rb, ra any
		if b != nil {
			rb = json.RawMessage(Redact(b))
		}
		if a != nil {
			ra = json.RawMessage(Redact(a))
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_log (actor, request_id, action, object_type, object_id, before, after)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, act.Name, act.RequestID, m.action, m.objectType, m.objectID, rb, ra); err != nil {
			return fmt.Errorf("controlplane: audit: %w", err)
		}
	}
	if len(m.ops) > 0 && !controlplane.SkipOutbox(ctx) {
		raw, err := json.Marshal(m.ops)
		if err != nil {
			return err
		}
		key := m.key
		if key == "" {
			key = m.objectType + "/" + m.objectID
		}
		// shared with the writers, exclusive for EnqueueSnapshot: a snapshot cannot be ordered before a change that
		// it did not see
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared($1)`, snapshotLockID); err != nil {
			return fmt.Errorf("controlplane: outbox lock: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox (object_key, ops) VALUES ($1,$2)`, key, json.RawMessage(raw)); err != nil {
			return fmt.Errorf("controlplane: outbox: %w", err)
		}
	}
	return nil
}

type auditRepo struct{ rt *runner }

func (a *auditRepo) List(ctx context.Context, f controlplane.AuditFilter) ([]controlplane.AuditEntry, error) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.ObjectType != "" {
		add("object_type=$%d", f.ObjectType)
	}
	if f.ObjectID != "" {
		add("object_id=$%d", f.ObjectID)
	}
	if f.Actor != "" {
		add("actor=$%d", f.Actor)
	}
	if f.Action != "" {
		add("lower(action)=lower($%d)", f.Action)
	}
	if !f.Since.IsZero() {
		add("at>=$%d", f.Since)
	}
	if !f.Until.IsZero() {
		add("at<$%d", f.Until)
	}
	if f.BeforeID > 0 {
		add("id<$%d", f.BeforeID)
	}
	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	q := `SELECT id, at, actor, request_id, action, object_type, object_id, before, after FROM audit_log`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY id DESC LIMIT %d", limit)
	rows, err := a.rt.q().Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("controlplane: audit list: %w", err)
	}
	defer rows.Close()
	var res []controlplane.AuditEntry
	for rows.Next() {
		var e controlplane.AuditEntry
		var b, af []byte
		if err := rows.Scan(&e.ID, &e.At, &e.Actor, &e.RequestID, &e.Action, &e.ObjectType, &e.ObjectID, &b, &af); err != nil {
			return nil, err
		}
		e.Before, e.After = b, af
		res = append(res, e)
	}
	return res, rows.Err()
}

func (a *auditRepo) Record(ctx context.Context, action, objectType, objectID string, details any) error {
	act := controlplane.ActorFrom(ctx)
	var ra any
	if details != nil {
		b, err := marshalOrNil(details)
		if err == nil {
			ra = json.RawMessage(Redact(b))
		}
	}
	return a.rt.write(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO audit_log (actor, request_id, action, object_type, object_id, before, after)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, act.Name, act.RequestID, action, objectType, objectID, nil, ra)
		if err != nil {
			return fmt.Errorf("controlplane: record audit: %w", err)
		}
		return nil
	})
}
