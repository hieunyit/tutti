// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"errors"
	"fmt"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/rs/zerolog"
	"slices"
	"time"
)

// reportMaxAge is how old a proxy status report may be to count (availability, votes, liveness).
const reportMaxAge = 10 * time.Second

// maxHistory bounds the promotion history stored in the group document.
const maxHistory = 100

type Backend interface {
	Validate(context.Context, Group, bool) error
	Ensure(context.Context, Group, entity.UniversalReplicationID) (int64, error)
	Seed(context.Context, entity.UniversalReplicationID, int64) error
	Replication(context.Context, entity.UniversalReplicationID) (entity.ReplicationStatusExtended, error)
	Pause(context.Context, Group) error
	// Commit atomically replaces routing, removes the old topology, and records group+history.
	Commit(context.Context, Group, *Group) error
}
type Resyncer interface {
	Resync(context.Context, Group, string) error
}
type Service struct {
	Store               *Store
	Backend             Backend
	Resyncer            Resyncer
	Reports             func(context.Context) ([]proxystatus.Status, error)
	ValidateCredentials func(context.Context, Group) error
	// ValidateLegacyFailover rejects a group over storages used by an enabled legacy failover policy.
	ValidateLegacyFailover func(context.Context, Group) error
	Now                    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// WithGroup serializes controllers and API state changes. Revision CAS remains the
// commit authority, so an expired leadership lease cannot overwrite a successor.
func (s *Service) WithGroup(ctx context.Context, id string, fn func(context.Context) error) error {
	l := store.NewRedisIDKeyLocker[string](s.Store.Redis, "tutti:protection:controller", store.StringToSingleTokenConverter, store.SingleTokenToStringConverter, 5*time.Second)
	lock, err := l.Lock(ctx, id, store.WithDuration(15*time.Second))
	if err != nil {
		return err
	}
	defer lock.Release(ctx)
	return lock.DoCtx(ctx, 10*time.Second, fn)
}
func (s *Service) Create(ctx context.Context, g Group, adopt bool) (Group, error) {
	return s.create(ctx, g, adopt, false)
}

// create implements Create; skipLegacy omits the enabled-legacy-policy check (see CreateForMigration).
func (s *Service) create(ctx context.Context, g Group, adopt, skipLegacy bool) (Group, error) {
	if err := g.Validate(); err != nil {
		return Group{}, err
	}
	// Server-owned lifecycle fields cannot be injected by callers.
	g.Generation = 1
	g.Revision = 0
	g.State = "initializing"
	g.Frozen = true
	g.Target = ""
	g.History = nil
	g.Seeded = map[string]int64{}
	g.Verified = map[string]int64{}
	g.Quarantined = nil
	err := s.WithGroup(ctx, g.ID, func(ctx context.Context) error {
		if _, err := s.Store.Get(ctx, g.ID); err == nil {
			return dom.ErrAlreadyExists
		} else if !errors.Is(err, dom.ErrNotFound) {
			return err
		}
		if s.ValidateCredentials != nil {
			if err := s.ValidateCredentials(ctx, g); err != nil {
				return err
			}
		}
		if s.ValidateLegacyFailover != nil && !skipLegacy {
			if err := s.ValidateLegacyFailover(ctx, g); err != nil {
				return err
			}
		}

		if preflight, ok := s.Resyncer.(interface {
			Validate(context.Context, Group) error
		}); ok {
			if err := preflight.Validate(ctx, g); err != nil {
				return err
			}
		}
		if err := s.Backend.Validate(ctx, g, adopt); err != nil {
			return err
		}
		if adopt {
			for _, id := range g.Replications(g.Primary) {
				if _, err := s.Backend.Replication(ctx, id); errors.Is(err, dom.ErrNotFound) {
					continue
				} else if err != nil {
					return err
				}
				gen, err := s.Backend.Ensure(ctx, g, id)
				if err != nil {
					return err
				}
				g.Seeded[id.AsString()] = gen
			}
		}
		return s.Store.Save(ctx, &g)
	})
	return g, err
}

func (s *Service) Update(ctx context.Context, input Group) (Group, error) {
	var result Group
	err := s.WithGroup(ctx, input.ID, func(ctx context.Context) error {
		g, err := s.Store.Get(ctx, input.ID)
		if err != nil {
			return err
		}
		if input.Revision != g.Revision {
			return fmt.Errorf("%w: group changed; reload", dom.ErrFailedPrecondition)
		}
		if g.State != "ready" {
			return fmt.Errorf("%w: group is not ready", dom.ErrFailedPrecondition)
		}
		// Membership/scope changes require detaching then explicitly adopting a new topology.
		if input.User != g.User || !slices.Equal(input.Buckets, g.Buckets) || !slices.Equal(input.Members, g.Members) {
			return fmt.Errorf("%w: scope and members are immutable; detach and adopt explicitly", dom.ErrFailedPrecondition)
		}
		g.Copies = input.Copies
		g.Mode = input.Mode
		g.After = input.After
		g.MaxEventLag = input.MaxEventLag
		g.Electorate = input.Electorate
		g.MaxSwitchesPerHour = input.MaxSwitchesPerHour
		if err = g.Validate(); err != nil {
			return err
		}
		err = s.Store.Save(ctx, &g)
		result = g
		return err
	})
	return result, err
}
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.WithGroup(ctx, id, func(ctx context.Context) error {
		g, err := s.Store.Get(ctx, id)
		if err != nil {
			return err
		}
		return s.Store.Delete(ctx, g)
	})
}
func (s *Service) AbortSwitch(ctx context.Context, id string) error {
	return s.WithGroup(ctx, id, func(ctx context.Context) error {
		g, err := s.Store.Get(ctx, id)
		if err != nil {
			return err
		}
		if g.Target == "" {
			return fmt.Errorf("%w: no uncommitted switch to abort", dom.ErrFailedPrecondition)
		}
		g.Target = ""
		g.Automatic = false
		g.Frozen = false
		g.State = "ready"
		g.LastError = ""
		return s.Store.Save(ctx, &g)
	})
}

func (s *Service) Status(ctx context.Context, id string) (Status, error) {
	g, err := s.Store.Get(ctx, id)
	if err != nil {
		return Status{}, err
	}
	reports, err := s.reports(ctx)
	if err != nil {
		return Status{}, err
	}
	return s.status(ctx, g, reports)
}
func (s *Service) reports(ctx context.Context) ([]proxystatus.Status, error) {
	if s.Reports == nil {
		return nil, nil
	}
	return s.Reports(ctx)
}
func (s *Service) status(ctx context.Context, g Group, reports []proxystatus.Status) (Status, error) {
	out := Status{Group: g}
	n, err := s.Store.ActiveWriters(ctx, g.ID)
	if err != nil {
		return out, err
	}
	out.ActiveWriters = n
	for _, member := range g.Members {
		st := MemberStatus{Storage: member, Available: Available(member, reports, s.now(), reportMaxAge), Initialized: true}
		if member == g.Primary {
			st.Current = st.Available
			out.Members = append(out.Members, st)
			if st.Current {
				out.HealthyCopies++
			}
			continue
		}
		for _, id := range g.Replications(g.Primary) {
			if id.ToStorage() != member {
				continue
			}
			info, err := s.Backend.Replication(ctx, id)
			if errors.Is(err, dom.ErrNotFound) {
				st.Initialized = false
				continue
			}
			if err != nil {
				return out, err
			}
			if !info.InitDone() || info.IsPaused || info.IsArchived || info.Switch != nil || g.Seeded[id.AsString()] == 0 {
				st.Initialized = false
			}
			st.EventLag += info.EventMigration.Unprocessed
		}
		st.Current = st.Available && st.Initialized && st.EventLag == 0 && g.Verified[member] == g.Generation && !slices.Contains(g.Quarantined, member)
		if st.Current {
			out.HealthyCopies++
		}
		out.Members = append(out.Members, st)
	}
	return out, nil
}

func (s *Service) RequestSwitch(ctx context.Context, id, target string) error {
	return s.WithGroup(ctx, id, func(ctx context.Context) error {
		g, err := s.Store.Get(ctx, id)
		if err != nil {
			return err
		}
		if g.State != "ready" {
			return fmt.Errorf("%w: group is %s", dom.ErrFailedPrecondition, g.State)
		}
		if target == g.Primary || !slices.Contains(g.Members, target) || slices.Contains(g.Quarantined, target) {
			return fmt.Errorf("%w: invalid or quarantined target", dom.ErrInvalidArg)
		}
		// Planned switches may wait for an initializing destination; write freeze starts only once its baseline is ready.
		g.Target = target
		g.Automatic = false
		g.State = "waiting"
		return s.Store.Save(ctx, &g)
	})
}

func (s *Service) Reconcile(ctx context.Context, id string) error {
	return s.WithGroup(ctx, id, func(ctx context.Context) error {
		g, err := s.Store.Get(ctx, id)
		if err != nil {
			return err
		}
		exported := false
		err = s.reconcile(ctx, &g, &exported)
		if !exported {
			// The step failed or returned before exporting (e.g. while promoting): export the current
			// status anyway so the gauges are never left stale.
			s.exportCurrent(ctx, g)
		}
		// The state may have changed after the status snapshot (or the step returned early).
		metrics.SetProtectionFrozen(g.ID, g.Frozen)
		if err != nil {
			metrics.CountProtectionReconcileError(g.ID)
			// Record only the error on the stored document: g may hold uncommitted in-memory
			// transitions of the failed step that must not be persisted by this side channel.
			if current, e := s.Store.Get(ctx, id); e == nil && current.LastError != err.Error() {
				current.LastError = err.Error()
				_ = s.Store.Save(ctx, &current)
			}
		}
		return err
	})
}

// controlPlaneSyncer is implemented by backends that commit promotions to Redis first and must retry the
// PostgreSQL part (policy.protectionBackend).
type controlPlaneSyncer interface {
	SyncControlPlane(context.Context, Group) error
}

// exportCurrent exports the gauges of g from a fresh status. Without proxy reports members count as
// unavailable, which is what the reconciler acts on too.
func (s *Service) exportCurrent(ctx context.Context, g Group) {
	reports, err := s.reports(ctx)
	if err != nil {
		reports = nil
	}
	st, err := s.status(ctx, g, reports)
	if err != nil {
		return
	}
	exportStatus(st)
}

func (s *Service) reconcile(ctx context.Context, g *Group, exported *bool) error {
	reports, err := s.reports(ctx)
	if err != nil {
		return err
	}
	if g.Frozen || g.State == "promoting" {
		// Only a frozen group waits for writers; drop records of processes proven gone.
		n, err := s.Store.ReapWriters(ctx, g.ID, reports, s.now(), reportMaxAge)
		metrics.CountReapedWriters(n)
		if err != nil {
			return err
		}
	}
	if g.State == "promoting" {
		return s.finishFrozen(ctx, g, reports)
	}
	if syncer, ok := s.Backend.(controlPlaneSyncer); ok {
		// Retry the PostgreSQL part of an earlier promotion before (re)creating replications from the new
		// primary. A failure must not stop the group (Redis is authoritative); it is retried every cycle.
		if err := syncer.SyncControlPlane(ctx, *g); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Str("group", g.ID).Msg("protection group: PostgreSQL sync of the last promotion failed; will retry")
		}
	}
	// Reconcile every healthy member, not just Copies-1. Missing members remain desired and
	// are retried after recovery. Failed/pending seeds are durable and never count as current.
	for _, id := range g.Replications(g.Primary) {
		if !Available(id.ToStorage(), reports, s.now(), reportMaxAge) || slices.Contains(g.Quarantined, id.ToStorage()) {
			continue
		}
		gen, err := s.Backend.Ensure(ctx, *g, id)
		if err != nil {
			return err
		}
		if g.Seeded == nil {
			g.Seeded = map[string]int64{}
		}
		if g.Seeded[id.AsString()] != gen {
			if err = s.Backend.Seed(ctx, id, gen); err != nil {
				return err
			}
			g.Seeded[id.AsString()] = gen
			if err = s.Store.Save(ctx, g); err != nil {
				return err
			}
		}
	}
	status, err := s.status(ctx, *g, reports)
	if err != nil {
		return err
	}
	exportStatus(status)
	*exported = true
	if g.State == "ready" && g.Mode == "auto" {
		frozen, err := s.Store.AutomaticFrozen(ctx)
		if err != nil {
			return err
		}
		metrics.SetAutomaticFailoverFrozen(frozen)
		hasQuorum := Quorum(*g, g.Primary, reports, s.now(), reportMaxAge)
		underLimit := s.underLimit(*g)
		if hasQuorum && frozen {
			metrics.CountProtectionSwitchBlocked(g.ID, "global_freeze")
		} else if hasQuorum && !underLimit {
			metrics.CountProtectionSwitchBlocked(g.ID, "rate_limit")
		}
		if !frozen && hasQuorum && underLimit {
			for _, member := range g.Members {
				for _, st := range status.Members {
					if st.Storage == member && member != g.Primary && st.Available && st.Initialized && st.EventLag <= g.MaxEventLag && g.Verified[member] == g.Generation && !slices.Contains(g.Quarantined, member) {
						g.Target = member
						g.Automatic = true
						g.State = "draining"
						g.Frozen = true
						return s.Store.Save(ctx, g)
					}
				}
			}
		}
	}
	if g.State == "waiting" {
		for _, st := range status.Members {
			if st.Storage == g.Target && st.Available && st.Initialized {
				g.State = "draining"
				g.Frozen = true
				return s.Store.Save(ctx, g)
			}
		}
		return nil
	}
	if g.State == "ready" {
		// Recovered members require an exact audit under a short write freeze before eligibility.
		for _, st := range status.Members {
			if st.Storage != g.Primary && st.Available && st.Initialized && st.EventLag == 0 && g.Verified[st.Storage] != g.Generation && !slices.Contains(g.Quarantined, st.Storage) {
				g.State = "initializing"
				g.Frozen = true
				return s.Store.Save(ctx, g)
			}
		}
		return nil
	}
	if g.State != "initializing" && g.State != "draining" {
		return fmt.Errorf("%w: unknown group state %s", dom.ErrFailedPrecondition, g.State)
	}
	if g.State == "initializing" && g.Target == "" {
		// Catch-up first: the bulk initial migration and event backlog run while writes are
		// admitted (to the primary only; replication captures them). The write freeze starts only
		// once every available member is initialized with zero lag, so the frozen window is the
		// drain of in-flight writes plus the final resync of a caught-up member.
		if !caughtUp(*g, status) {
			if g.Frozen {
				g.Frozen = false
				return s.Store.Save(ctx, g)
			}
			return nil
		}
		if !g.Frozen {
			g.Frozen = true
			return s.Store.Save(ctx, g)
		}
	}
	if status.ActiveWriters != 0 {
		return nil
	}
	if g.Target != "" {
		ready := false
		for _, st := range status.Members {
			if st.Storage == g.Target && st.Available && st.Initialized && (st.EventLag == 0 || (g.Automatic && st.EventLag <= g.MaxEventLag)) {
				ready = true
			}
		}
		if !ready {
			return nil
		}
	} else if !caughtUp(*g, status) {
		return nil
	}
	// Block worker admission before pausing queues. In-flight worker records remain until
	// their handlers really return, independent of asynq's task lease accounting.
	g.State = "promoting"
	if err = s.Store.Save(ctx, g); err != nil {
		return err
	}
	return s.finishFrozen(ctx, g, reports)
}

func (s *Service) finishFrozen(ctx context.Context, g *Group, reports []proxystatus.Status) error {
	if err := s.Backend.Pause(ctx, *g); err != nil {
		return err
	}
	clients, err := s.Store.ActiveWriters(ctx, g.ID)
	if err != nil {
		return err
	}
	workers, err := s.Store.Redis.HLen(ctx, writerKey(g.ID)+":workers").Result()
	if err != nil {
		return err
	}
	if clients != 0 || workers != 0 {
		return nil
	}
	if g.Target == "" {
		if s.Resyncer == nil {
			return fmt.Errorf("%w: exact resync is not configured", dom.ErrFailedPrecondition)
		}
		if g.Verified == nil {
			g.Verified = map[string]int64{}
		}
		for _, member := range g.Members {
			if member == g.Primary || !Available(member, reports, s.now(), reportMaxAge) || slices.Contains(g.Quarantined, member) {
				continue
			}
			if err = s.Resyncer.Resync(ctx, *g, member); err != nil {
				return err
			}
			g.Verified[member] = g.Generation
			if err = s.Store.Save(ctx, g); err != nil {
				return err
			}
		}
		next := *g
		next.State = "ready"
		next.Frozen = false
		next.LastError = ""
		if err = s.Backend.Commit(ctx, *g, &next); err != nil {
			return err
		}
		*g = next
		return nil
	}
	lag := 0
	for _, id := range g.Replications(g.Primary) {
		if id.ToStorage() != g.Target {
			continue
		}
		info, e := s.Backend.Replication(ctx, id)
		if e != nil {
			return e
		}
		if !info.InitDone() {
			return fmt.Errorf("%w: target initial migration became incomplete", dom.ErrFailedPrecondition)
		}
		lag += info.EventMigration.Unprocessed
	}
	if lag > 0 && (!g.Automatic || lag > g.MaxEventLag) {
		return fmt.Errorf("%w: destination lag changed while draining", dom.ErrFailedPrecondition)
	}
	if !g.Automatic {
		if s.Resyncer == nil {
			return fmt.Errorf("%w: exact resync is not configured", dom.ErrFailedPrecondition)
		}
		if err = s.Resyncer.Resync(ctx, *g, g.Target); err != nil {
			return err
		}
	}
	next := *g
	next.Primary = g.Target
	next.Target = ""
	next.Generation++
	next.State = "initializing"
	next.Frozen = true
	next.Seeded = map[string]int64{}
	next.Verified = map[string]int64{}
	next.LastError = ""
	if lag > 0 {
		for _, m := range g.Members {
			if m != next.Primary {
				next.Quarantined = append(next.Quarantined, m)
			}
		}
	}
	next.History = append(append([]Event(nil), g.History...), Event{From: g.Primary, To: next.Primary, At: s.now().UTC(), Generation: next.Generation, Automatic: g.Automatic, EventLag: lag})
	// The document is decoded by every admission: keep it bounded. The newest entries suffice for
	// the hourly switch limit.
	if keep := max(maxHistory, g.MaxSwitchesPerHour); len(next.History) > keep {
		next.History = next.History[len(next.History)-keep:]
	}
	if err = s.Backend.Commit(ctx, *g, &next); err != nil {
		return err
	}
	*g = next
	reason := "manual"
	if next.History[len(next.History)-1].Automatic {
		reason = "automatic"
	}
	metrics.CountProtectionPrimaryChange(g.ID, reason)
	return nil
}

// exportStatus publishes the gauges of one group.
func exportStatus(st Status) {
	lag := 0
	for _, m := range st.Members {
		lag = max(lag, m.EventLag)
	}
	metrics.SetProtectionGroup(st.Group.ID, metrics.ProtectionGroupState{
		Healthy: st.HealthyCopies, Wanted: st.Group.Copies, MaxEventLag: lag,
		Frozen: st.Group.Frozen, ActiveWriters: st.ActiveWriters,
	})
}

// caughtUp reports whether every available, non-quarantined secondary is initialized with zero lag.
func caughtUp(g Group, status Status) bool {
	for _, st := range status.Members {
		if st.Storage == g.Primary || !st.Available || slices.Contains(g.Quarantined, st.Storage) {
			continue
		}
		if !st.Initialized || st.EventLag != 0 {
			return false
		}
	}
	return true
}

func (s *Service) underLimit(g Group) bool {
	n := 0
	for _, e := range g.History {
		if s.now().Sub(e.At) < time.Hour {
			n++
		}
	}
	return n < g.MaxSwitchesPerHour
}

func (s *Service) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		groups, err := s.Store.List(ctx)
		if err == nil {
			ids := make([]string, 0, len(groups))
			for _, g := range groups {
				ids = append(ids, g.ID)
				_ = s.Reconcile(ctx, g.ID)
			}
			metrics.KeepProtectionGroups(ids)
			if frozen, e := s.Store.AutomaticFrozen(ctx); e == nil {
				metrics.SetAutomaticFailoverFrozen(frozen)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
