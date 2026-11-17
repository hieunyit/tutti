// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package lifecycle_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/lifecycle"
)

func TestComputeNextRun(t *testing.T) {
	ref := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Default schedule
	next, err := lifecycle.ComputeNextRun("", ref)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, 2, next.Hour())
	require.Equal(t, 0, next.Minute())

	// Custom schedule (every hour at :15)
	next, err = lifecycle.ComputeNextRun("15 * * * *", ref)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, 0, next.Hour())
	require.Equal(t, 15, next.Minute())

	// Invalid schedule
	_, err = lifecycle.ComputeNextRun("invalid cron", ref)
	require.Error(t, err)
	require.True(t, errors.Is(err, dom.ErrInvalidArg))
}

type mockLifecycleStore struct {
	mu        sync.Mutex
	rules     map[int64]controlplane.LifecycleRule
	audits    []controlplane.LifecycleAuditEntry
	nextID    int64
	dueRules  []controlplane.LifecycleRule
	lastStats map[int64]controlplane.LifecycleRunStats
}

func newMockLifecycleStore() *mockLifecycleStore {
	return &mockLifecycleStore{
		rules:     make(map[int64]controlplane.LifecycleRule),
		nextID:    1,
		lastStats: make(map[int64]controlplane.LifecycleRunStats),
	}
}

func (m *mockLifecycleStore) CreateRule(ctx context.Context, r controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.ID = m.nextID
	m.nextID++
	now := time.Now()
	r.CreatedAt = now
	r.UpdatedAt = now
	m.rules[r.ID] = r
	return r, nil
}

func (m *mockLifecycleStore) GetRule(ctx context.Context, id int64) (controlplane.LifecycleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rules[id]
	if !ok {
		return controlplane.LifecycleRule{}, dom.ErrNotFound
	}
	return r, nil
}

func (m *mockLifecycleStore) ListRules(ctx context.Context, storageName, bucket string) ([]controlplane.LifecycleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []controlplane.LifecycleRule
	for _, r := range m.rules {
		if storageName != "" && r.StorageName != storageName {
			continue
		}
		if bucket != "" && r.Bucket != bucket {
			continue
		}
		res = append(res, r)
	}
	return res, nil
}

func (m *mockLifecycleStore) UpdateRule(ctx context.Context, r controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rules[r.ID]
	if !ok {
		return controlplane.LifecycleRule{}, dom.ErrNotFound
	}
	cur.Name = r.Name
	cur.Prefix = r.Prefix
	cur.ExpirationDays = r.ExpirationDays
	cur.Schedule = r.Schedule
	cur.Enabled = r.Enabled
	cur.UpdatedAt = time.Now()
	m.rules[r.ID] = cur
	return cur, nil
}

func (m *mockLifecycleStore) DeleteRule(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[id]; !ok {
		return dom.ErrNotFound
	}
	delete(m.rules, id)
	return nil
}

func (m *mockLifecycleStore) SetRuleEnabled(ctx context.Context, id int64, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rules[id]
	if !ok {
		return dom.ErrNotFound
	}
	cur.Enabled = enabled
	m.rules[id] = cur
	return nil
}

func (m *mockLifecycleStore) UpdateRuleRun(ctx context.Context, id int64, nextRunAt, lastRunAt time.Time, stats controlplane.LifecycleRunStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rules[id]
	if !ok {
		return dom.ErrNotFound
	}
	cur.NextRunAt = &nextRunAt
	cur.LastRunAt = &lastRunAt
	cur.LastRunStats = &stats
	m.rules[id] = cur
	m.lastStats[id] = stats
	return nil
}

func (m *mockLifecycleStore) ListDueRules(ctx context.Context, now time.Time) ([]controlplane.LifecycleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dueRules, nil
}

func (m *mockLifecycleStore) ClaimRule(ctx context.Context, id int64, now time.Time, lease time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rules[id]
	if !ok || !cur.Enabled {
		return false, nil
	}
	if cur.NextRunAt != nil && cur.NextRunAt.After(now) {
		return false, nil
	}
	claimUntil := now.Add(lease)
	cur.NextRunAt = &claimUntil
	m.rules[id] = cur
	return true, nil
}

func (m *mockLifecycleStore) RecordAudit(ctx context.Context, a controlplane.LifecycleAuditEntry) error {
	return m.RecordAudits(ctx, []controlplane.LifecycleAuditEntry{a})
}

func (m *mockLifecycleStore) RecordAudits(ctx context.Context, entries []controlplane.LifecycleAuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range entries {
		a.ID = int64(len(m.audits) + 1)
		m.audits = append(m.audits, a)
	}
	return nil
}

func (m *mockLifecycleStore) ListAuditLogs(ctx context.Context, ruleID int64, limit, offset int64) ([]controlplane.LifecycleAuditEntry, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var filtered []controlplane.LifecycleAuditEntry
	for _, a := range m.audits {
		if a.RuleID == ruleID {
			filtered = append(filtered, a)
		}
	}
	total := int64(len(filtered))
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}

func TestLifecycleServiceCRUD(t *testing.T) {
	ctx := context.Background()
	store := newMockLifecycleStore()
	svc := lifecycle.NewService(store, nil, nil)

	// 1. Create rule with valid schedule
	rule := controlplane.LifecycleRule{
		Name:           "cleanup-cache",
		StorageName:    "s3-main",
		Bucket:         "cache-bucket",
		Prefix:         "cache/",
		ExpirationDays: 3,
		Schedule:       "0 4 * * *",
		Enabled:        true,
	}
	created, err := svc.CreateRule(ctx, rule)
	require.NoError(t, err)
	require.Equal(t, int64(1), created.ID)
	require.NotNil(t, created.NextRunAt)

	// 2. Get rule
	got, err := svc.GetRule(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cleanup-cache", got.Name)

	// 3. List rules
	list, err := svc.ListRules(ctx, "s3-main", "cache-bucket")
	require.NoError(t, err)
	require.Len(t, list, 1)

	// 4. Update rule with invalid schedule
	created.Schedule = "bad schedule"
	_, err = svc.UpdateRule(ctx, created)
	require.Error(t, err)

	// 5. Update rule with valid schedule
	created.Schedule = "30 2 * * *"
	created.ExpirationDays = 5
	updated, err := svc.UpdateRule(ctx, created)
	require.NoError(t, err)
	require.Equal(t, int32(5), updated.ExpirationDays)

	// 6. Set enabled
	err = svc.SetRuleEnabled(ctx, created.ID, false)
	require.NoError(t, err)
	got, err = svc.GetRule(ctx, created.ID)
	require.NoError(t, err)
	require.False(t, got.Enabled)

	// 7. Audit logs
	_ = store.RecordAudit(ctx, controlplane.LifecycleAuditEntry{
		RuleID:       created.ID,
		Bucket:       "cache-bucket",
		ObjectKey:    "cache/obj1",
		SizeBytes:    1024,
		LastModified: time.Now().Add(-5 * 24 * time.Hour),
		DeletedAt:    time.Now(),
	})
	logs, total, err := svc.ListAuditLogs(ctx, created.ID, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)

	// 8. Delete rule
	err = svc.DeleteRule(ctx, created.ID)
	require.NoError(t, err)
	_, err = svc.GetRule(ctx, created.ID)
	require.Error(t, err)
}

func TestLifecycleScheduler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := newMockLifecycleStore()
	svc := lifecycle.NewService(store, nil, nil)
	logger := zerolog.Nop()

	// Scheduler will tick fast
	sched := lifecycle.NewScheduler(svc, store, logger, lifecycle.WithSchedulerInterval(50*time.Millisecond))

	rule := controlplane.LifecycleRule{
		ID:             99,
		Name:           "disabled-rule",
		StorageName:    "s3",
		Bucket:         "b",
		ExpirationDays: 1,
		Enabled:        false, // disabled, so ExecuteRule returns nil without error
	}
	store.rules[99] = rule
	store.dueRules = []controlplane.LifecycleRule{rule}

	// Run scheduler in background for a short duration
	go sched.Run(ctx)
	time.Sleep(120 * time.Millisecond)
	cancel()
}

func TestShouldExpire(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	// Zero or negative expirationDays
	require.False(t, lifecycle.ShouldExpire(now.Add(-48*time.Hour), 0, now))
	require.False(t, lifecycle.ShouldExpire(now.Add(-48*time.Hour), -5, now))

	// Zero lastModified must never expire (guards against catastrophic data loss)
	require.False(t, lifecycle.ShouldExpire(time.Time{}, 30, now))

	// Normal expiration at midnight UTC following LastModified + expirationDays
	lm := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	// expDate = 2026-06-01 + 10d + 1d = 2026-06-12 00:00:00 UTC
	// Before expDate
	require.False(t, lifecycle.ShouldExpire(lm, 10, time.Date(2026, 6, 11, 23, 59, 59, 0, time.UTC)))
	// At and after expDate
	require.True(t, lifecycle.ShouldExpire(lm, 10, time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)))
	require.True(t, lifecycle.ShouldExpire(lm, 10, now))
}
