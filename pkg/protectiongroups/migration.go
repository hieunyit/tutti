// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/failover"
)

// Migration of a legacy failover policy to protection groups ("path A, step 1").
//
// A legacy policy (per SOURCE storage) fails over every scope (user or bucket) that has a replication from the
// source; failover.DiscoverScopes is the single definition of those scopes. The migration proposes one group per
// scope whose members are the source plus the policy targets that already replicate that scope. Groups ADOPT the
// existing replications (Service.Create with adopt=true records them as seeded at their current generation), so
// no data is copied: reconcile never enqueues an initial migration for an adopted replication. Targets without a
// replication for a scope are left out of that group rather than created (creating one would copy data).

// Migration scope statuses.
const (
	MigrationOK       = "ok"       // can be created
	MigrationMigrated = "migrated" // an identical group already exists
	MigrationConflict = "conflict" // cannot be created until the reason is resolved
	MigrationSkip     = "skip"     // nothing to protect (no replication to any target)
)

// MigrationScope is one scope of the legacy policy and the group proposed for it.
type MigrationScope struct {
	User   string `json:"user"`
	Bucket string `json:"bucket,omitempty"` // empty: whole user
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	// Group is the proposed group (also set for migrated scopes: the existing one).
	Group Group `json:"group"`
	// SkippedTargets are policy targets left out because they have no replication for the scope.
	SkippedTargets []string `json:"skippedTargets,omitempty"`
}

// MigrationPlan is the dry-run result.
type MigrationPlan struct {
	Source string           `json:"source"`
	Policy failover.Policy  `json:"policy"`
	Scopes []MigrationScope `json:"scopes"`
	// NotCovered lists what the migration does not cover (conflicting or skipped scopes and standing caveats).
	NotCovered []string `json:"notCovered,omitempty"`
}

// Complete reports whether every scope is (or can be) migrated.
func (p MigrationPlan) Complete() bool {
	for _, s := range p.Scopes {
		if s.Status == MigrationConflict || s.Status == MigrationSkip {
			return false
		}
	}
	return true
}

// MigrationResult is the outcome of Apply.
type MigrationResult struct {
	Plan MigrationPlan `json:"plan"`
	// Created are the IDs of the groups created by this call; Migrated those that already existed.
	Created  []string `json:"created,omitempty"`
	Migrated []string `json:"migrated,omitempty"`
	// Uncovered are the scopes that were not migrated (conflict or skip, or a failed creation).
	Uncovered      []MigrationScope `json:"uncovered,omitempty"`
	PolicyDisabled bool             `json:"policyDisabled"`
}

// MigrationOptions tune the proposed groups.
type MigrationOptions struct {
	// Mode of the groups: "auto" (default, like the legacy policy) or "manual".
	Mode string
	// Electorate overrides the voters of automatic groups; empty means Migrator.Electorate().
	Electorate []string
}

// Migrator plans and applies the migration.
type Migrator struct {
	Service     *Service
	Policies    failover.Repository
	Source      failover.ScopeSource
	MainStorage string
	// Users returns users that may own bucket replications (see failover.DiscoverOptions.Users).
	Users func() []string
	// Electorate returns the proxies voting in automatic groups (the live proxies).
	Electorate func(ctx context.Context) []string
	// Create creates one group adopting its existing replications. Nil: Service.CreateForMigration. The API
	// supplies a variant that holds the user and storage locks.
	Create func(ctx context.Context, g Group) (Group, error)
	Log    zerolog.Logger
}

// GroupID is the deterministic ID of the group migrated from the scope of a policy, so that a re-run recognizes it.
func GroupID(source, user, bucket string) string {
	id := "legacy-" + source + "-" + user
	if bucket != "" {
		id += "-" + bucket
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(":/\x00 \t\r\n", r) {
			return '-'
		}
		return r
	}, id)
}

func (m *Migrator) policy(ctx context.Context, source string) (failover.Policy, error) {
	if source == "" {
		return failover.Policy{}, fmt.Errorf("%w: failover migration: source is required", dom.ErrInvalidArg)
	}
	return m.Policies.GetFailoverPolicy(ctx, source) // dom.ErrNotFound if missing
}

// Plan computes the migration of the legacy policy of source without changing anything.
func (m *Migrator) Plan(ctx context.Context, source string, o MigrationOptions) (MigrationPlan, error) {
	pol, err := m.policy(ctx, source)
	if err != nil {
		return MigrationPlan{}, err
	}
	plan := MigrationPlan{Source: source, Policy: pol}
	opts := failover.DiscoverOptions{MainStorage: m.MainStorage}
	if m.Users != nil {
		opts.Users = m.Users()
	}
	scopes, err := failover.DiscoverScopes(ctx, m.Source, source, opts)
	if err != nil {
		return plan, err
	}
	existing, err := m.Service.Store.List(ctx)
	if err != nil {
		return plan, err
	}
	mode := o.Mode
	if mode == "" {
		mode = "auto"
	}
	var electorate []string
	if mode == "auto" {
		electorate = slices.Clone(o.Electorate)
		if len(electorate) == 0 && m.Electorate != nil {
			electorate = m.Electorate(ctx)
		}
	}
	maxLag := 0
	if pol.MaxEventLag != nil {
		maxLag = *pol.MaxEventLag
	}
	var proposed []Group
	for _, sc := range scopes {
		ms := MigrationScope{User: sc.User, Bucket: sc.Bucket}
		g := Group{ID: GroupID(source, sc.User, sc.Bucket), User: sc.User, Primary: source, Mode: mode,
			After: pol.After, MaxEventLag: maxLag, Electorate: slices.Clone(electorate)}
		if sc.Bucket != "" {
			g.Buckets = []string{sc.Bucket}
		}
		g.Members = []string{source}
		var archived []string
		for _, target := range pol.Targets {
			found := false
			for _, c := range sc.Candidates {
				if c.To != target {
					continue
				}
				found = true
				if c.Status.ReplicationStatus != nil && c.Status.IsArchived {
					archived = append(archived, target)
				}
			}
			if found {
				g.Members = append(g.Members, target)
			} else {
				ms.SkippedTargets = append(ms.SkippedTargets, target)
			}
		}
		g.Copies = len(g.Members)
		ms.Group = g
		switch {
		case len(g.Members) < 2:
			ms.Status, ms.Reason = MigrationSkip, "no replication from "+source+" to any policy target"
		case len(archived) != 0:
			ms.Status, ms.Reason = MigrationConflict, "replication to "+strings.Join(archived, ", ")+" is archived; recover it first"
		default:
			ms.Status, ms.Reason = m.check(ctx, &ms, existing, proposed)
			if ms.Status == MigrationOK {
				proposed = append(proposed, ms.Group)
			}
		}
		if ms.Status == MigrationOK && len(ms.SkippedTargets) != 0 {
			ms.Reason = "no replication to " + strings.Join(ms.SkippedTargets, ", ") + ": left out of the group (nothing is copied)"
		}
		plan.Scopes = append(plan.Scopes, ms)
	}
	for _, s := range plan.Scopes {
		switch s.Status {
		case MigrationConflict, MigrationSkip:
			plan.NotCovered = append(plan.NotCovered, fmt.Sprintf("%s: %s (%s)", scopeLabel(s), s.Status, s.Reason))
		}
	}
	plan.NotCovered = append(plan.NotCovered,
		"buckets or users that get a replication from "+source+" after the migration (the legacy policy covered them automatically); create a group for them from the Protection groups page",
		"scopes currently routed away from "+source+" (already failed over): the legacy controller does not act on them either")
	return plan, nil
}

func scopeLabel(s MigrationScope) string {
	if s.Bucket == "" {
		return s.User
	}
	return s.User + "/" + s.Bucket
}

// check validates the proposed group of ms and returns its status and reason.
func (m *Migrator) check(ctx context.Context, ms *MigrationScope, existing, proposed []Group) (string, string) {
	g := &ms.Group
	for _, o := range existing {
		if o.User == g.User && slices.Equal(o.Buckets, g.Buckets) && slices.Equal(o.Members, g.Members) {
			ms.Group = o
			return MigrationMigrated, "group " + o.ID + " already exists"
		}
	}
	for _, o := range existing {
		if g.Overlaps(o) {
			return MigrationConflict, "overlaps existing protection group " + o.ID
		}
	}
	for _, o := range proposed {
		if g.Overlaps(o) {
			return MigrationConflict, "overlaps the group proposed for another scope (" + o.ID + "): groups of different users must not share a member for the same bucket"
		}
	}
	v := *g
	if err := v.Validate(); err != nil {
		return MigrationConflict, trimErr(err)
	}
	ms.Group = v
	if m.Service.ValidateCredentials != nil {
		if err := m.Service.ValidateCredentials(ctx, v); err != nil {
			return MigrationConflict, trimErr(err)
		}
	}
	if err := m.Service.Backend.Validate(ctx, v, true); err != nil {
		return MigrationConflict, trimErr(err)
	}
	return MigrationOK, ""
}

func trimErr(err error) string {
	s := err.Error()
	for _, p := range []string{"protection group: "} {
		s = strings.ReplaceAll(s, p, "")
	}
	return s
}

func (m *Migrator) create(ctx context.Context, g Group) (Group, error) {
	if m.Create != nil {
		return m.Create(ctx, g)
	}
	return m.Service.CreateForMigration(ctx, g)
}

// Apply creates the missing groups of the plan and disables the legacy policy when every scope is migrated, or
// when force is set (the uncovered scopes are then listed in the result and lose their automatic failover).
// It is idempotent: existing identical groups are left alone.
func (m *Migrator) Apply(ctx context.Context, source string, force bool, o MigrationOptions) (MigrationResult, error) {
	plan, err := m.Plan(ctx, source, o)
	res := MigrationResult{Plan: plan}
	if err != nil {
		return res, err
	}
	for _, s := range plan.Scopes {
		switch s.Status {
		case MigrationMigrated:
			res.Migrated = append(res.Migrated, s.Group.ID)
		case MigrationOK:
			if _, err := m.create(ctx, s.Group); err != nil {
				if errors.Is(err, dom.ErrAlreadyExists) {
					res.Migrated = append(res.Migrated, s.Group.ID)
					continue
				}
				s.Status, s.Reason = MigrationConflict, "creating the group failed: "+trimErr(err)
				res.Uncovered = append(res.Uncovered, s)
				continue
			}
			res.Created = append(res.Created, s.Group.ID)
		default:
			res.Uncovered = append(res.Uncovered, s)
		}
	}
	if len(res.Uncovered) == 0 || force {
		if plan.Policy.Enabled {
			p := plan.Policy
			p.Enabled = false
			if err := m.Policies.SetFailoverPolicy(ctx, p); err != nil {
				return res, fmt.Errorf("groups created but the legacy policy could not be disabled: %w", err)
			}
		}
		res.PolicyDisabled = true
	}
	ev := m.Log.Info().Str("audit", "failover_migration").Str("source", source).Bool("force", force).
		Strs("created", res.Created).Strs("already_migrated", res.Migrated).Int("uncovered", len(res.Uncovered)).
		Bool("policy_disabled", res.PolicyDisabled).Time("at", time.Now().UTC())
	ev.Msg("audit")
	return res, nil
}

// CreateForMigration is Create with adoption of the existing replications. It skips the check against enabled
// legacy failover policies: the legacy controller never acts on a scope a group owns, and the policy being
// migrated is disabled once every scope has its group (or the operator forced it).
func (s *Service) CreateForMigration(ctx context.Context, g Group) (Group, error) {
	return s.create(ctx, g, true, true)
}
