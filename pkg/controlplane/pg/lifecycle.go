// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

type lifecycleStore struct {
	rt *runner
}

const lifecycleRuleCols = `id, name, storage_name, bucket, prefix, expiration_days, schedule, enabled, next_run_at, last_run_at, last_run_stats, created_at, updated_at`

func scanLifecycleRule(scan func(dest ...any) error, r *controlplane.LifecycleRule) error {
	var statsJSON []byte
	err := scan(&r.ID, &r.Name, &r.StorageName, &r.Bucket, &r.Prefix, &r.ExpirationDays,
		&r.Schedule, &r.Enabled, &r.NextRunAt, &r.LastRunAt, &statsJSON, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return err
	}
	if len(statsJSON) > 0 && string(statsJSON) != "null" {
		var stats controlplane.LifecycleRunStats
		if err := json.Unmarshal(statsJSON, &stats); err != nil {
			return fmt.Errorf("%w: lifecycle rule stats corrupted: %v", dom.ErrInternal, err)
		}
		r.LastRunStats = &stats
	}
	return nil
}

func (s *lifecycleStore) CreateRule(ctx context.Context, r controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
	if r.Name == "" {
		return r, fmt.Errorf("%w: name is required", dom.ErrInvalidArg)
	}
	if r.StorageName == "" || r.Bucket == "" {
		return r, fmt.Errorf("%w: storage_name and bucket are required", dom.ErrInvalidArg)
	}
	if r.ExpirationDays <= 0 {
		return r, fmt.Errorf("%w: expiration_days must be positive", dom.ErrInvalidArg)
	}
	if r.Schedule == "" {
		r.Schedule = "0 2 * * *"
	}

	var created controlplane.LifecycleRule
	err := s.rt.write(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `INSERT INTO lifecycle_rules (name, storage_name, bucket, prefix, expiration_days, schedule, enabled, next_run_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+lifecycleRuleCols,
			r.Name, r.StorageName, r.Bucket, r.Prefix, r.ExpirationDays, r.Schedule, r.Enabled, r.NextRunAt)
		if err := scanLifecycleRule(row.Scan, &created); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("%w: lifecycle rule for storage %q, bucket %q and prefix %q already exists", dom.ErrAlreadyExists, r.StorageName, r.Bucket, r.Prefix)
			}
			return err
		}
		return nil
	})
	return created, err
}

func (s *lifecycleStore) GetRule(ctx context.Context, id int64) (controlplane.LifecycleRule, error) {
	var r controlplane.LifecycleRule
	row := s.rt.q().QueryRow(ctx, `SELECT `+lifecycleRuleCols+` FROM lifecycle_rules WHERE id=$1`, id)
	if err := scanLifecycleRule(row.Scan, &r); errors.Is(err, pgx.ErrNoRows) {
		return r, fmt.Errorf("%w: lifecycle rule %d", dom.ErrNotFound, id)
	} else if err != nil {
		return r, err
	}
	return r, nil
}

func (s *lifecycleStore) ListRules(ctx context.Context, storageName, bucket string) ([]controlplane.LifecycleRule, error) {
	q := `SELECT ` + lifecycleRuleCols + ` FROM lifecycle_rules WHERE 1=1`
	var args []any
	argIdx := 1
	if storageName != "" {
		q += fmt.Sprintf(" AND storage_name=$%d", argIdx)
		args = append(args, storageName)
		argIdx++
	}
	if bucket != "" {
		q += fmt.Sprintf(" AND bucket=$%d", argIdx)
		args = append(args, bucket)
		argIdx++
	}
	q += " ORDER BY id ASC"

	rows, err := s.rt.q().Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]controlplane.LifecycleRule, 0)
	for rows.Next() {
		var r controlplane.LifecycleRule
		if err := scanLifecycleRule(rows.Scan, &r); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *lifecycleStore) UpdateRule(ctx context.Context, r controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
	if r.ID <= 0 {
		return r, fmt.Errorf("%w: rule id is required", dom.ErrInvalidArg)
	}
	if r.Name == "" {
		return r, fmt.Errorf("%w: name is required", dom.ErrInvalidArg)
	}
	if r.ExpirationDays <= 0 {
		return r, fmt.Errorf("%w: expiration_days must be positive", dom.ErrInvalidArg)
	}
	if r.Schedule == "" {
		r.Schedule = "0 2 * * *"
	}

	var updated controlplane.LifecycleRule
	err := s.rt.write(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `UPDATE lifecycle_rules SET name=$2, prefix=$3, expiration_days=$4, schedule=$5, enabled=$6, next_run_at=COALESCE($7, next_run_at), updated_at=NOW()
			WHERE id=$1 RETURNING `+lifecycleRuleCols,
			r.ID, r.Name, r.Prefix, r.ExpirationDays, r.Schedule, r.Enabled, r.NextRunAt)
		if err := scanLifecycleRule(row.Scan, &updated); errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: lifecycle rule %d", dom.ErrNotFound, r.ID)
		} else if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("%w: lifecycle rule for storage %q, bucket %q and prefix %q already exists", dom.ErrAlreadyExists, r.StorageName, r.Bucket, r.Prefix)
			}
			return err
		}
		return nil
	})
	return updated, err
}

func (s *lifecycleStore) DeleteRule(ctx context.Context, id int64) error {
	return s.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM lifecycle_rules WHERE id=$1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: lifecycle rule %d", dom.ErrNotFound, id)
		}
		return nil
	})
}

func (s *lifecycleStore) SetRuleEnabled(ctx context.Context, id int64, enabled bool) error {
	return s.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE lifecycle_rules SET enabled=$2, updated_at=NOW() WHERE id=$1`, id, enabled)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: lifecycle rule %d", dom.ErrNotFound, id)
		}
		return nil
	})
}

func (s *lifecycleStore) UpdateRuleRun(ctx context.Context, id int64, nextRunAt, lastRunAt time.Time, stats controlplane.LifecycleRunStats) error {
	return s.rt.write(ctx, func(tx pgx.Tx) error {
		statsJSON, _ := json.Marshal(stats)
		tag, err := tx.Exec(ctx, `UPDATE lifecycle_rules SET next_run_at=$2, last_run_at=$3, last_run_stats=$4, updated_at=NOW() WHERE id=$1`,
			id, nextRunAt.UTC(), lastRunAt.UTC(), statsJSON)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: lifecycle rule %d", dom.ErrNotFound, id)
		}
		return nil
	})
}

func (s *lifecycleStore) ListDueRules(ctx context.Context, now time.Time) ([]controlplane.LifecycleRule, error) {
	rows, err := s.rt.q().Query(ctx, `SELECT `+lifecycleRuleCols+` FROM lifecycle_rules
		WHERE enabled=true AND (next_run_at IS NULL OR next_run_at <= $1) AND (lease_until IS NULL OR lease_until <= clock_timestamp()) ORDER BY id ASC`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]controlplane.LifecycleRule, 0)
	for rows.Next() {
		var r controlplane.LifecycleRule
		if err := scanLifecycleRule(rows.Scan, &r); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *lifecycleStore) ClaimRule(ctx context.Context, id int64, now time.Time, lease time.Duration) (bool, error) {
	claimUntil := now.Add(lease)
	var claimed bool
	err := s.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE lifecycle_rules SET next_run_at = $2, updated_at = NOW()
			WHERE id = $1 AND enabled = true AND (next_run_at IS NULL OR next_run_at <= $3)`,
			id, claimUntil.UTC(), now.UTC())
		if err != nil {
			return err
		}
		claimed = tag.RowsAffected() > 0
		return nil
	})
	return claimed, err
}

func (s *lifecycleStore) AcquireLifecycleLease(ctx context.Context, id int64, token string, lease time.Duration, dueOnly bool) (bool, error) {
	var ok bool
	err := s.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE lifecycle_rules SET lease_token=$2, lease_until=clock_timestamp()+$3 * interval '1 microsecond', updated_at=NOW()
   WHERE id=$1 AND enabled=true AND (lease_until IS NULL OR lease_until <= clock_timestamp())
   AND (NOT $4 OR next_run_at IS NULL OR next_run_at <= clock_timestamp())`, id, token, lease.Microseconds(), dueOnly)
		ok = tag.RowsAffected() > 0
		return err
	})
	return ok, err
}

func (s *lifecycleStore) RenewLifecycleLease(ctx context.Context, id int64, token string, lease time.Duration) (bool, error) {
	var ok bool
	err := s.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE lifecycle_rules SET lease_until=clock_timestamp()+$3 * interval '1 microsecond'
   WHERE id=$1 AND lease_token=$2 AND lease_until > clock_timestamp() AND enabled=true`, id, token, lease.Microseconds())
		ok = tag.RowsAffected() > 0
		return err
	})
	return ok, err
}

func (s *lifecycleStore) CompleteLifecycleLease(ctx context.Context, id int64, token string, nextRunAt, lastRunAt time.Time, stats controlplane.LifecycleRunStats) (bool, error) {
	var ok bool
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return false, err
	}
	err = s.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE lifecycle_rules SET next_run_at=$3, last_run_at=$4, last_run_stats=$5, lease_token=NULL, lease_until=NULL, updated_at=NOW()
   WHERE id=$1 AND lease_token=$2 AND lease_until > clock_timestamp()`, id, token, nextRunAt.UTC(), lastRunAt.UTC(), statsJSON)
		ok = tag.RowsAffected() > 0
		return err
	})
	return ok, err
}

func (s *lifecycleStore) RecordAudit(ctx context.Context, a controlplane.LifecycleAuditEntry) error {
	return s.RecordAudits(ctx, []controlplane.LifecycleAuditEntry{a})
}

func (s *lifecycleStore) RecordAudits(ctx context.Context, entries []controlplane.LifecycleAuditEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return s.rt.write(ctx, func(tx pgx.Tx) error {
		rows := make([][]any, len(entries))
		for i, a := range entries {
			delAt := a.DeletedAt
			if delAt.IsZero() {
				delAt = time.Now()
			}
			lastMod := a.LastModified
			if lastMod.IsZero() {
				lastMod = time.Now()
			}
			rows[i] = []any{a.RuleID, a.Bucket, a.ObjectKey, a.SizeBytes, lastMod.UTC(), delAt.UTC()}
		}
		_, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"lifecycle_audit_log"},
			[]string{"rule_id", "bucket", "object_key", "size_bytes", "last_modified", "deleted_at"},
			pgx.CopyFromRows(rows),
		)
		return err
	})
}

func (s *lifecycleStore) ListAuditLogs(ctx context.Context, ruleID int64, limit, offset int64) ([]controlplane.LifecycleAuditEntry, int64, error) {
	q := `SELECT id, rule_id, bucket, object_key, size_bytes, last_modified, deleted_at FROM lifecycle_audit_log WHERE 1=1`
	countQ := `SELECT COUNT(*) FROM lifecycle_audit_log WHERE 1=1`
	var args []any
	argIdx := 1
	if ruleID > 0 {
		q += fmt.Sprintf(" AND rule_id=$%d", argIdx)
		countQ += fmt.Sprintf(" AND rule_id=$%d", argIdx)
		args = append(args, ruleID)
		argIdx++
	}

	var total int64
	if err := s.rt.q().QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q += " ORDER BY id DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
		argIdx++
	} else {
		q += " LIMIT 100"
	}
	if offset > 0 {
		q += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, offset)
		argIdx++
	}

	rows, err := s.rt.q().Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := make([]controlplane.LifecycleAuditEntry, 0)
	for rows.Next() {
		var e controlplane.LifecycleAuditEntry
		if err := rows.Scan(&e.ID, &e.RuleID, &e.Bucket, &e.ObjectKey, &e.SizeBytes, &e.LastModified, &e.DeletedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
