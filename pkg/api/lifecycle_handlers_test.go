// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package api_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/api"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/lifecycle"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

type testLifecycleStore struct {
	mu     sync.Mutex
	rules  map[int64]controlplane.LifecycleRule
	audits []controlplane.LifecycleAuditEntry
	nextID int64
}

func newTestLifecycleStore() *testLifecycleStore {
	return &testLifecycleStore{
		rules:  make(map[int64]controlplane.LifecycleRule),
		nextID: 1,
	}
}

func (m *testLifecycleStore) CreateRule(ctx context.Context, r controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.ID = m.nextID
	m.nextID++
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	m.rules[r.ID] = r
	return r, nil
}

func (m *testLifecycleStore) GetRule(ctx context.Context, id int64) (controlplane.LifecycleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rules[id]
	if !ok {
		return controlplane.LifecycleRule{}, dom.ErrNotFound
	}
	return r, nil
}

func (m *testLifecycleStore) ListRules(ctx context.Context, storageName, bucket string) ([]controlplane.LifecycleRule, error) {
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

func (m *testLifecycleStore) UpdateRule(ctx context.Context, r controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
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

func (m *testLifecycleStore) DeleteRule(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[id]; !ok {
		return dom.ErrNotFound
	}
	delete(m.rules, id)
	return nil
}

func (m *testLifecycleStore) SetRuleEnabled(ctx context.Context, id int64, enabled bool) error {
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

func (m *testLifecycleStore) UpdateRuleRun(ctx context.Context, id int64, nextRunAt, lastRunAt time.Time, stats controlplane.LifecycleRunStats) error {
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
	return nil
}

func (m *testLifecycleStore) ListDueRules(ctx context.Context, now time.Time) ([]controlplane.LifecycleRule, error) {
	return nil, nil
}

func (m *testLifecycleStore) ClaimRule(ctx context.Context, id int64, now time.Time, lease time.Duration) (bool, error) {
	return true, nil
}

func (m *testLifecycleStore) RecordAudit(ctx context.Context, a controlplane.LifecycleAuditEntry) error {
	return m.RecordAudits(ctx, []controlplane.LifecycleAuditEntry{a})
}

func (m *testLifecycleStore) RecordAudits(ctx context.Context, entries []controlplane.LifecycleAuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range entries {
		a.ID = int64(len(m.audits) + 1)
		m.audits = append(m.audits, a)
	}
	return nil
}

func (m *testLifecycleStore) ListAuditLogs(ctx context.Context, ruleID int64, limit, offset int64) ([]controlplane.LifecycleAuditEntry, int64, error) {
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

func TestLifecycleHandlers(t *testing.T) {
	ctx := context.Background()
	store := newTestLifecycleStore()
	svc := lifecycle.NewService(store, nil, nil)
	handler := api.LifecycleHandlers(svc)

	// 1. CreateRule
	created, err := handler.CreateRule(ctx, &pb.CreateLifecycleRuleRequest{
		Name:           "cleanup-tmp",
		StorageName:    "store1",
		Bucket:         "bucket1",
		Prefix:         "temp/",
		ExpirationDays: 7,
		Schedule:       "0 2 * * *",
		Enabled:        true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.GetId())
	require.Equal(t, "cleanup-tmp", created.GetName())
	require.Equal(t, int32(7), created.GetExpirationDays())

	// 2. GetRule
	got, err := handler.GetRule(ctx, &pb.GetLifecycleRuleRequest{Id: created.GetId()})
	require.NoError(t, err)
	require.Equal(t, created.GetId(), got.GetId())
	require.Equal(t, "cleanup-tmp", got.GetName())

	// 3. ListRules
	list, err := handler.ListRules(ctx, &pb.ListLifecycleRulesRequest{
		StorageName: "store1",
		Bucket:      "bucket1",
	})
	require.NoError(t, err)
	require.Len(t, list.GetRules(), 1)

	// 4. UpdateRule
	updated, err := handler.UpdateRule(ctx, &pb.UpdateLifecycleRuleRequest{
		Id:             created.GetId(),
		Name:           "cleanup-tmp-v2",
		Prefix:         "temp/",
		ExpirationDays: 14,
		Schedule:       "0 3 * * *",
		Enabled:        true,
	})
	require.NoError(t, err)
	require.Equal(t, "cleanup-tmp-v2", updated.GetName())
	require.Equal(t, int32(14), updated.GetExpirationDays())

	// 5. Disable and Enable
	_, err = handler.DisableRule(ctx, &pb.DisableLifecycleRuleRequest{Id: created.GetId()})
	require.NoError(t, err)
	got, err = handler.GetRule(ctx, &pb.GetLifecycleRuleRequest{Id: created.GetId()})
	require.NoError(t, err)
	require.False(t, got.GetEnabled())

	_, err = handler.EnableRule(ctx, &pb.EnableLifecycleRuleRequest{Id: created.GetId()})
	require.NoError(t, err)
	got, err = handler.GetRule(ctx, &pb.GetLifecycleRuleRequest{Id: created.GetId()})
	require.NoError(t, err)
	require.True(t, got.GetEnabled())

	// 6. Audit logs
	_ = store.RecordAudit(ctx, controlplane.LifecycleAuditEntry{
		RuleID:       created.GetId(),
		Bucket:       "bucket1",
		ObjectKey:    "temp/file.txt",
		SizeBytes:    2048,
		LastModified: time.Now().Add(-10 * 24 * time.Hour),
		DeletedAt:    time.Now(),
	})
	logs, err := handler.ListAuditLogs(ctx, &pb.ListLifecycleAuditLogsRequest{
		RuleId: created.GetId(),
		Limit:  10,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), logs.GetTotal())
	require.Len(t, logs.GetEntries(), 1)
	require.Equal(t, "temp/file.txt", logs.GetEntries()[0].GetObjectKey())

	// 7. Validation errors
	_, err = handler.CreateRule(ctx, &pb.CreateLifecycleRuleRequest{
		Name:           "invalid-ttl",
		StorageName:    "s1",
		Bucket:         "bucket1",
		ExpirationDays: 0,
	})
	require.Error(t, err)

	_, err = handler.CreateRule(ctx, &pb.CreateLifecycleRuleRequest{
		Name:           "invalid-ttl-large",
		StorageName:    "s1",
		Bucket:         "bucket1",
		ExpirationDays: 50000,
	})
	require.Error(t, err)

	_, err = handler.ListAuditLogs(ctx, &pb.ListLifecycleAuditLogsRequest{
		RuleId: created.GetId(),
		Offset: -1,
	})
	require.Error(t, err)

	// 8. DeleteRule
	_, err = handler.DeleteRule(ctx, &pb.DeleteLifecycleRuleRequest{Id: created.GetId()})
	require.NoError(t, err)
	_, err = handler.GetRule(ctx, &pb.GetLifecycleRuleRequest{Id: created.GetId()})
	require.Error(t, err)
}
