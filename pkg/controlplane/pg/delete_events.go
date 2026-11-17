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

type deleteEvents struct {
	rt *runner
}

func (d *deleteEvents) Create(ctx context.Context, e controlplane.DeleteEvent) (int64, error) {
	if e.ReplicationID == "" {
		return 0, fmt.Errorf("%w: replication_id is required", dom.ErrInvalidArg)
	}
	if e.Bucket == "" || e.ObjectKey == "" {
		return 0, fmt.Errorf("%w: bucket and object_key are required", dom.ErrInvalidArg)
	}
	if e.DeletionType == "" {
		e.DeletionType = "hard"
	}
	if e.Status == "" {
		e.Status = "pending"
	}
	if e.SourceTimestamp.IsZero() {
		e.SourceTimestamp = time.Now()
	}
	e.SourceTimestamp = e.SourceTimestamp.UTC()

	var id int64
	err := d.rt.write(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `INSERT INTO delete_events (replication_id, bucket, object_key, version_id, deletion_type, source_timestamp, status, retry_count, last_error, generation)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
			e.ReplicationID, e.Bucket, e.ObjectKey, e.VersionID, e.DeletionType, e.SourceTimestamp, e.Status, e.RetryCount, e.LastError, e.Generation).Scan(&id)
	})
	return id, err
}

func (d *deleteEvents) UpdateStatus(ctx context.Context, id int64, status, lastError string) error {
	return d.rt.write(ctx, func(tx pgx.Tx) error {
		var retryInc int
		if status == "failed" {
			retryInc = 1
		}
		tag, err := tx.Exec(ctx, `UPDATE delete_events SET status=$2, last_error=$3, retry_count=retry_count+$4, updated_at=NOW() WHERE id=$1`,
			id, status, lastError, retryInc)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: delete event %d", dom.ErrNotFound, id)
		}
		return nil
	})
}

func (d *deleteEvents) List(ctx context.Context, f controlplane.DeleteEventFilter) ([]controlplane.DeleteEvent, int64, error) {
	q := `SELECT id, replication_id, bucket, object_key, COALESCE(version_id, ''), deletion_type, source_timestamp, status, retry_count, COALESCE(last_error, ''), created_at, updated_at, generation
		FROM delete_events WHERE 1=1`
	countQ := `SELECT COUNT(*) FROM delete_events WHERE 1=1`
	var args []any
	argIdx := 1

	if f.ReplicationID != "" {
		q += fmt.Sprintf(" AND replication_id=$%d", argIdx)
		countQ += fmt.Sprintf(" AND replication_id=$%d", argIdx)
		args = append(args, f.ReplicationID)
		argIdx++
	}
	if f.Status != "" {
		q += fmt.Sprintf(" AND status=$%d", argIdx)
		countQ += fmt.Sprintf(" AND status=$%d", argIdx)
		args = append(args, f.Status)
		argIdx++
	}
	if f.BeforeID > 0 {
		q += fmt.Sprintf(" AND id<$%d", argIdx)
		countQ += fmt.Sprintf(" AND id<$%d", argIdx)
		args = append(args, f.BeforeID)
		argIdx++
	}

	var total int64
	if err := d.rt.q().QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q += " ORDER BY id DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, f.Limit)
		argIdx++
	} else {
		q += " LIMIT 100"
	}
	if f.Offset > 0 {
		q += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, f.Offset)
		argIdx++
	}

	rows, err := d.rt.q().Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events := make([]controlplane.DeleteEvent, 0)
	for rows.Next() {
		var e controlplane.DeleteEvent
		if err := rows.Scan(&e.ID, &e.ReplicationID, &e.Bucket, &e.ObjectKey, &e.VersionID, &e.DeletionType,
			&e.SourceTimestamp, &e.Status, &e.RetryCount, &e.LastError, &e.CreatedAt, &e.UpdatedAt, &e.Generation); err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}

// EnqueueRetry serializes retry requests and rolls the pending transition back when enqueue fails.
func (d *deleteEvents) EnqueueRetry(ctx context.Context, id int64, retryCount int32, enqueue func() error) (bool, error) {
	claimed := false
	err := d.rt.write(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE delete_events SET status='pending', last_error='', updated_at=NOW()
			WHERE id=$1 AND status='failed' AND retry_count=$2`, id, retryCount)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if err := enqueue(); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed && err == nil, err
}

func (d *deleteEvents) RecordAudit(ctx context.Context, a controlplane.DeleteAuditEntry) error {
	return d.rt.write(ctx, func(tx pgx.Tx) error {
		if a.DeleteEventID != 0 {
			status := "success"
			lastErr := ""
			retryInc := 0
			switch a.Action {
			case "skipped":
				status = "skipped"
			case "failed":
				status = "failed"
				retryInc = 1
				lastErr, _ = a.Metadata["error"].(string)
			}
			// The queue write may succeed even when the retry claim rolls back.
			// Accept that task's outcome on the same failed row and retry count.
			tag, err := tx.Exec(ctx, `UPDATE delete_events SET status=$2, last_error=$3, retry_count=retry_count+$4, updated_at=NOW()
				WHERE id=$1 AND retry_count=$5 AND status IN ('pending', 'failed')`,
				a.DeleteEventID, status, lastErr, retryInc, a.DeleteEventRetryCount)
			if err != nil {
				return fmt.Errorf("failed to update delete event status: %w", err)
			}
			if tag.RowsAffected() == 0 {
				return nil
			}
		}

		var metaJSON []byte
		if a.Metadata != nil {
			metaJSON, _ = json.Marshal(a.Metadata)
		} else {
			metaJSON = []byte("{}")
		}
		ts := a.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		_, err := tx.Exec(ctx, `INSERT INTO delete_audit_log (replication_id, bucket, object_key, action, destination_storage, timestamp, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			a.ReplicationID, a.Bucket, a.ObjectKey, a.Action, a.DestinationStorage, ts.UTC(), metaJSON)
		return err
	})
}
