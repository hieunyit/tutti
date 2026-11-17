// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/robfig/cron/v3"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// ComputeNextRun computes the next run time from the given cron schedule expression.
func ComputeNextRun(schedule string, from time.Time) (*time.Time, error) {
	if schedule == "" {
		schedule = "0 2 * * *" // default 2 AM daily
	}
	s, err := cronParser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid cron schedule %q: %v", dom.ErrInvalidArg, schedule, err)
	}
	next := s.Next(from.UTC())
	if next.IsZero() {
		return nil, fmt.Errorf("%w: cron schedule %q produces no future runs", dom.ErrInvalidArg, schedule)
	}
	return &next, nil
}

// ShouldExpire checks if an object should expire according to the Amazon S3 Lifecycle specification:
// Object expires at midnight 00:00:00 UTC of the day following (LastModified + expirationDays).
func ShouldExpire(lastModified time.Time, expirationDays int32, now time.Time) bool {
	if expirationDays <= 0 || lastModified.IsZero() {
		return false
	}
	expDate := lastModified.UTC().Truncate(24*time.Hour).AddDate(0, 0, int(expirationDays)+1)
	return !now.UTC().Before(expDate)
}

type DryRunRequest struct {
	StorageName    string
	Bucket         string
	Prefix         string
	ExpirationDays int32
	SampleSize     int32
}

type DryRunResult struct {
	EstimatedDeletedCount int64
	EstimatedBytesFreed   int64
	SampleObjects         []string
}

type Service interface {
	CreateRule(ctx context.Context, rule controlplane.LifecycleRule) (controlplane.LifecycleRule, error)
	GetRule(ctx context.Context, id int64) (controlplane.LifecycleRule, error)
	ListRules(ctx context.Context, storageName, bucket string) ([]controlplane.LifecycleRule, error)
	UpdateRule(ctx context.Context, rule controlplane.LifecycleRule) (controlplane.LifecycleRule, error)
	DeleteRule(ctx context.Context, id int64) error
	SetRuleEnabled(ctx context.Context, id int64, enabled bool) error
	DryRun(ctx context.Context, req DryRunRequest) (DryRunResult, error)
	ListAuditLogs(ctx context.Context, ruleID int64, limit, offset int64) ([]controlplane.LifecycleAuditEntry, int64, error)
	ExecuteRule(ctx context.Context, id int64) (controlplane.LifecycleRunStats, error)
}

type service struct {
	store    controlplane.LifecycleStore
	clients  objstore.Clients
	credsSvc objstore.CredsService
}

func NewService(store controlplane.LifecycleStore, clients objstore.Clients, credsSvc objstore.CredsService) Service {
	return &service{
		store:    store,
		clients:  clients,
		credsSvc: credsSvc,
	}
}

func (s *service) CreateRule(ctx context.Context, rule controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
	if rule.ExpirationDays < 1 || rule.ExpirationDays > 36500 {
		return rule, fmt.Errorf("%w: expiration_days must be between 1 and 36500", dom.ErrInvalidArg)
	}
	if rule.Schedule == "" {
		rule.Schedule = "0 2 * * *"
	}
	nextRun, err := ComputeNextRun(rule.Schedule, time.Now())
	if err != nil {
		return rule, err
	}
	rule.NextRunAt = nextRun
	return s.store.CreateRule(ctx, rule)
}

func (s *service) GetRule(ctx context.Context, id int64) (controlplane.LifecycleRule, error) {
	return s.store.GetRule(ctx, id)
}

func (s *service) ListRules(ctx context.Context, storageName, bucket string) ([]controlplane.LifecycleRule, error) {
	return s.store.ListRules(ctx, storageName, bucket)
}

func (s *service) UpdateRule(ctx context.Context, rule controlplane.LifecycleRule) (controlplane.LifecycleRule, error) {
	if rule.ExpirationDays < 1 || rule.ExpirationDays > 36500 {
		return rule, fmt.Errorf("%w: expiration_days must be between 1 and 36500", dom.ErrInvalidArg)
	}
	if rule.Schedule != "" {
		nextRun, err := ComputeNextRun(rule.Schedule, time.Now())
		if err != nil {
			return rule, fmt.Errorf("%w: invalid cron schedule %q: %v", dom.ErrInvalidArg, rule.Schedule, err)
		}
		rule.NextRunAt = nextRun
	}
	return s.store.UpdateRule(ctx, rule)
}

func (s *service) DeleteRule(ctx context.Context, id int64) error {
	return s.store.DeleteRule(ctx, id)
}

func (s *service) SetRuleEnabled(ctx context.Context, id int64, enabled bool) error {
	return s.store.SetRuleEnabled(ctx, id, enabled)
}

func (s *service) ListAuditLogs(ctx context.Context, ruleID int64, limit, offset int64) ([]controlplane.LifecycleAuditEntry, int64, error) {
	return s.store.ListAuditLogs(ctx, ruleID, limit, offset)
}

func (s *service) getUserForStorage(storage string) string {
	if s.credsSvc != nil {
		users := s.credsSvc.ListUsers(storage)
		if len(users) > 0 {
			return users[0]
		}
	}
	return ""
}

func (s *service) DryRun(ctx context.Context, req DryRunRequest) (DryRunResult, error) {
	if req.StorageName == "" || req.Bucket == "" {
		return DryRunResult{}, fmt.Errorf("%w: storage and bucket are required", dom.ErrInvalidArg)
	}
	if req.ExpirationDays < 1 || req.ExpirationDays > 36500 {
		return DryRunResult{}, fmt.Errorf("%w: expiration_days must be between 1 and 36500", dom.ErrInvalidArg)
	}
	if s.clients == nil {
		return DryRunResult{}, fmt.Errorf("clients registry unavailable")
	}

	user := s.getUserForStorage(req.StorageName)
	client, err := s.clients.AsS3(ctx, req.StorageName, user)
	if err != nil {
		return DryRunResult{}, fmt.Errorf("storage client error: %w", err)
	}

	sampleSize := req.SampleSize
	if sampleSize <= 0 {
		sampleSize = 10
	} else if sampleSize > 100 {
		sampleSize = 100
	}

	now := time.Now()
	var result DryRunResult
	listCtx, cancelList := context.WithCancel(ctx)
	defer cancelList()

	const maxScanned = 50000
	scanned := 0
	for obj := range client.S3().ListObjects(listCtx, req.Bucket, minio.ListObjectsOptions{Prefix: req.Prefix, Recursive: true}) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if obj.Err != nil {
			return result, obj.Err
		}
		scanned++
		if ShouldExpire(obj.LastModified, req.ExpirationDays, now) {
			result.EstimatedDeletedCount++
			result.EstimatedBytesFreed += obj.Size
			if int32(len(result.SampleObjects)) < sampleSize {
				result.SampleObjects = append(result.SampleObjects, obj.Key)
			}
		}
		if scanned >= maxScanned {
			break
		}
	}

	return result, nil
}

// scheduledRunKey distinguishes scheduler due checks from explicit ExecuteRule calls.
type scheduledRunKey struct{}

const lifecycleLeaseDuration = 15 * time.Minute
const lifecycleRenewMargin = 5 * time.Minute

// leaseGuard serializes renewals and tracks a conservative local deadline. The
// deadline starts before the database request, so network latency cannot make
// us believe a lease lasts longer than the database's lease.
type leaseGuard struct {
	mu         sync.Mutex
	store      controlplane.LifecycleLeaseStore
	id         int64
	token      string
	validUntil time.Time
}

func (g *leaseGuard) renew(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.renewLocked(ctx)
}

func (g *leaseGuard) renewLocked(ctx context.Context) error {
	started := time.Now()
	renewCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ok, err := g.store.RenewLifecycleLease(renewCtx, g.id, g.token, lifecycleLeaseDuration)
	if err != nil {
		return fmt.Errorf("lifecycle lease renewal: %w", err)
	}
	if !ok {
		return fmt.Errorf("lifecycle lease lost")
	}
	g.validUntil = started.Add(lifecycleLeaseDuration)
	return nil
}

func (g *leaseGuard) ensureValid(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if time.Until(g.validUntil) <= lifecycleRenewMargin {
		return g.renewLocked(ctx)
	}
	return nil
}

func (s *service) ExecuteRule(ctx context.Context, id int64) (stats controlplane.LifecycleRunStats, err error) {
	rule, err := s.store.GetRule(ctx, id)
	if err != nil {
		return stats, err
	}
	if !rule.Enabled {
		return stats, nil
	}
	leases, ok := s.store.(controlplane.LifecycleLeaseStore)
	if !ok {
		return stats, fmt.Errorf("lifecycle store does not support fenced leases")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return stats, err
	}
	token := hex.EncodeToString(nonce[:])
	dueOnly, _ := ctx.Value(scheduledRunKey{}).(bool)
	claimStarted := time.Now()
	claimed, err := leases.AcquireLifecycleLease(ctx, id, token, lifecycleLeaseDuration, dueOnly)
	if err != nil {
		return stats, fmt.Errorf("acquire lifecycle lease: %w", err)
	}
	if !claimed {
		return stats, fmt.Errorf("lifecycle rule is already leased or no longer due")
	}
	start := time.Now()
	guard := &leaseGuard{store: leases, id: id, token: token, validUntil: claimStarted.Add(lifecycleLeaseDuration)}
	runCtx, cancelRun := context.WithCancel(ctx)
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if e := guard.renew(runCtx); e != nil {
					cancelRun()
					return
				}
			}
		}
	}()
	defer func() {
		close(stopHeartbeat)
		cancelRun()
		<-heartbeatDone
		now := time.Now()
		stats.DurationMs = now.Sub(start).Milliseconds()
		next := now.Add(10 * time.Minute)
		if err == nil {
			scheduled, e := ComputeNextRun(rule.Schedule, now)
			if e != nil {
				err = e
			} else {
				next = *scheduled
			}
		}
		completeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		completed, e := leases.CompleteLifecycleLease(completeCtx, id, token, next, now, stats)
		if e != nil {
			err = errors.Join(err, fmt.Errorf("complete lifecycle lease: %w", e))
		} else if !completed {
			err = errors.Join(err, fmt.Errorf("lifecycle lease lost before completion"))
		}
	}()
	if s.clients == nil {
		return stats, fmt.Errorf("clients registry unavailable")
	}
	client, err := s.clients.AsS3(runCtx, rule.StorageName, s.getUserForStorage(rule.StorageName))
	if err != nil {
		return stats, fmt.Errorf("storage client error: %w", err)
	}
	versioning, err := client.S3().GetBucketVersioning(runCtx, rule.Bucket)
	if err != nil {
		return stats, fmt.Errorf("check bucket versioning: %w", err)
	}
	if !versioning.Enabled() {
		return stats, fmt.Errorf("lifecycle requires bucket versioning enabled for safe deletion")
	}

	// A version-specific delete cannot erase a concurrent overwrite. Refuse a key
	// with any older live version: deleting its latest version would reveal it.
	now := time.Now()
	var key string
	var latest *minio.ObjectInfo
	var olderLive bool
	var runErr error
	failureCount := 0
	recordFailure := func(e error) {
		failureCount++
		if failureCount <= 10 {
			runErr = errors.Join(runErr, e)
		}
	}
	defer func() {
		if failureCount > 10 {
			err = errors.Join(err, fmt.Errorf("%d additional lifecycle errors omitted", failureCount-10))
		}
	}()
	// Keep audit writes batched even though safe deletion targets individual versions.
	audits := make([]controlplane.LifecycleAuditEntry, 0, 500)
	flushAudits := func() error {
		if len(audits) == 0 {
			return nil
		}
		batch := audits
		audits = nil
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(runCtx), 5*time.Second)
		defer cancel()
		if e := s.store.RecordAudits(auditCtx, batch); e != nil {
			return fmt.Errorf("record lifecycle audit: %w", e)
		}
		return nil
	}
	defer func() { err = errors.Join(err, flushAudits()) }()
	flush := func() {
		if latest == nil || !ShouldExpire(latest.LastModified, rule.ExpirationDays, now) {
			return
		}
		if olderLive {
			recordFailure(fmt.Errorf("object %q has older live versions", key))
			return
		}
		if latest.VersionID == "" || latest.VersionID == "null" {
			recordFailure(fmt.Errorf("object %q has no immutable version ID", key))
			return
		}
		if e := guard.ensureValid(runCtx); e != nil {
			recordFailure(e)
			cancelRun()
			return
		}
		if e := client.S3().RemoveObject(runCtx, rule.Bucket, key, minio.RemoveObjectOptions{VersionID: latest.VersionID}); e != nil {
			recordFailure(fmt.Errorf("delete object %q: %w", key, e))
			return
		}
		stats.DeletedCount++
		stats.BytesFreed += latest.Size
		audits = append(audits, controlplane.LifecycleAuditEntry{RuleID: id, Bucket: rule.Bucket, ObjectKey: key, SizeBytes: latest.Size, LastModified: latest.LastModified, DeletedAt: time.Now()})
		if len(audits) >= 500 {
			if e := flushAudits(); e != nil {
				recordFailure(e)
				cancelRun()
			}
		}
	}
	listCtx, cancelList := context.WithCancel(runCtx)
	defer cancelList()
	for obj := range client.S3().ListObjects(listCtx, rule.Bucket, minio.ListObjectsOptions{Prefix: rule.Prefix, Recursive: true, WithVersions: true}) {
		if runCtx.Err() != nil {
			return stats, errors.Join(runErr, runCtx.Err())
		}
		if obj.Err != nil {
			return stats, errors.Join(runErr, fmt.Errorf("list object versions: %w", obj.Err))
		}
		if key != "" && obj.Key != key {
			flush()
			latest = nil
			olderLive = false
		}
		key = obj.Key
		if obj.IsLatest && !obj.IsDeleteMarker {
			copy := obj
			latest = &copy
		} else if !obj.IsLatest && !obj.IsDeleteMarker {
			olderLive = true
		}
	}
	if runCtx.Err() != nil {
		return stats, errors.Join(runErr, runCtx.Err())
	}
	flush()
	if runCtx.Err() != nil {
		return stats, errors.Join(runErr, runCtx.Err())
	}
	return stats, runErr
}
