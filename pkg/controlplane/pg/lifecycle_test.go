// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func TestLifecycleRulesCRUD(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Repos().Lifecycle

	// 1. Create Rule
	rule := controlplane.LifecycleRule{
		Name:           "cleanup-temp",
		StorageName:    "minio1",
		Bucket:         "logs-bucket",
		Prefix:         "tmp/",
		ExpirationDays: 7,
		Schedule:       "0 3 * * *",
		Enabled:        true,
	}
	created, err := repo.CreateRule(ctx, rule)
	require.NoError(t, err)
	require.Greater(t, created.ID, int64(0))
	require.Equal(t, "cleanup-temp", created.Name)
	require.Equal(t, int32(7), created.ExpirationDays)

	// 2. Get Rule
	got, err := repo.GetRule(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "cleanup-temp", got.Name)

	// 3. List Rules
	rules, err := repo.ListRules(ctx, "minio1", "logs-bucket")
	require.NoError(t, err)
	require.Len(t, rules, 1)

	// 4. Update Rule
	created.Name = "cleanup-temp-updated"
	created.ExpirationDays = 14
	updated, err := repo.UpdateRule(ctx, created)
	require.NoError(t, err)
	require.Equal(t, "cleanup-temp-updated", updated.Name)
	require.Equal(t, int32(14), updated.ExpirationDays)

	// 5. Update Run stats
	now := time.Now()
	next := now.Add(24 * time.Hour)
	err = repo.UpdateRuleRun(ctx, created.ID, next, now, controlplane.LifecycleRunStats{
		DeletedCount: 42,
		BytesFreed:   102400,
		DurationMs:   250,
	})
	require.NoError(t, err)

	got, err = repo.GetRule(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LastRunStats)
	require.Equal(t, int64(42), got.LastRunStats.DeletedCount)
	require.Equal(t, int64(102400), got.LastRunStats.BytesFreed)

	// 6. List Due Rules
	due, err := repo.ListDueRules(ctx, now.Add(48*time.Hour))
	require.NoError(t, err)
	require.Len(t, due, 1)

	notDue, err := repo.ListDueRules(ctx, now)
	require.NoError(t, err)
	require.Empty(t, notDue)

	// 7. Disable Rule
	err = repo.SetRuleEnabled(ctx, created.ID, false)
	require.NoError(t, err)
	got, err = repo.GetRule(ctx, created.ID)
	require.NoError(t, err)
	require.False(t, got.Enabled)

	// 8. Record and list audit log
	err = repo.RecordAudit(ctx, controlplane.LifecycleAuditEntry{
		RuleID:       created.ID,
		Bucket:       "logs-bucket",
		ObjectKey:    "tmp/test.log",
		SizeBytes:    1024,
		LastModified: now.Add(-10 * 24 * time.Hour),
		DeletedAt:    now,
	})
	require.NoError(t, err)

	auditLogs, total, err := repo.ListAuditLogs(ctx, created.ID, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, auditLogs, 1)
	require.Equal(t, "tmp/test.log", auditLogs[0].ObjectKey)

	// 9. Delete Rule
	err = repo.DeleteRule(ctx, created.ID)
	require.NoError(t, err)
	_, err = repo.GetRule(ctx, created.ID)
	require.Error(t, err)
}

func TestLifecycleLeaseFencing(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Repos().Lifecycle
	rule, err := repo.CreateRule(ctx, controlplane.LifecycleRule{Name: "lease", StorageName: "s", Bucket: "bucket", ExpirationDays: 1, Enabled: true})
	require.NoError(t, err)
	leases, ok := repo.(controlplane.LifecycleLeaseStore)
	require.True(t, ok)
	acquired, err := leases.AcquireLifecycleLease(ctx, rule.ID, "owner-a", 15*time.Minute, true)
	require.NoError(t, err)
	require.True(t, acquired)
	renewed, err := leases.RenewLifecycleLease(ctx, rule.ID, "owner-a", 15*time.Minute)
	require.NoError(t, err)
	require.True(t, renewed)
	acquired, err = leases.AcquireLifecycleLease(ctx, rule.ID, "owner-b", 15*time.Minute, true)
	require.NoError(t, err)
	require.False(t, acquired)
	_, err = s.Pool().Exec(ctx, `UPDATE lifecycle_rules SET lease_until=now()-interval '1 second' WHERE id=$1`, rule.ID)
	require.NoError(t, err)
	acquired, err = leases.AcquireLifecycleLease(ctx, rule.ID, "owner-b", 15*time.Minute, true)
	require.NoError(t, err)
	require.True(t, acquired)
	completed, err := leases.CompleteLifecycleLease(ctx, rule.ID, "owner-a", time.Now().Add(time.Hour), time.Now(), controlplane.LifecycleRunStats{DeletedCount: 99})
	require.NoError(t, err)
	require.False(t, completed)
	completed, err = leases.CompleteLifecycleLease(ctx, rule.ID, "owner-b", time.Now().Add(time.Hour), time.Now(), controlplane.LifecycleRunStats{DeletedCount: 1})
	require.NoError(t, err)
	require.True(t, completed)
	got, err := repo.GetRule(ctx, rule.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, got.LastRunStats.DeletedCount)
}
