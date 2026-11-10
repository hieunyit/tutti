// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
// Package protectiongroups owns durable desired replication topology and promotion state.
package protectiongroups

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

const GroupsKey = "tutti:protection:groups"
const PromotionKey = "tutti:protection:promotion"
const FreezeKey = "tutti:protection:automatic-frozen"

// GroupSpec holds the desired configuration of a protection group.
// When controlPlane.backend is "postgres", PostgreSQL is the source of truth for GroupSpec.
type GroupSpec struct {
	ID   string `json:"id"`
	User string `json:"user"`
	// Empty means all buckets of this user. Bucket lists contain exact names, not globs.
	Buckets     []string      `json:"buckets,omitempty"`
	Members     []string      `json:"members"`
	Copies      int           `json:"copies"`
	MaxEventLag int           `json:"maxEventLag"`
	Mode        string        `json:"mode"`
	After       time.Duration `json:"after"`
	// Electorate is explicit and never shrinks when a proxy stops reporting.
	Electorate         []string `json:"electorate,omitempty"`
	MaxSwitchesPerHour int      `json:"maxSwitchesPerHour"`
}

// PromotionState holds the runtime promotion and write-fence state of a protection group.
// It lives in Redis so the proxy admission Lua script can inspect primary/frozen/state/generation at line rate.
type PromotionState struct {
	Primary     string           `json:"primary"`
	Generation  int64            `json:"generation"`
	Revision    int64            `json:"revision"`
	State       string           `json:"state"`
	Target      string           `json:"target,omitempty"`
	Automatic   bool             `json:"automatic,omitempty"`
	Frozen      bool             `json:"frozen"`
	Seeded      map[string]int64 `json:"seeded,omitempty"`
	Verified    map[string]int64 `json:"verified,omitempty"`
	Quarantined []string         `json:"quarantined,omitempty"`
	History     []Event          `json:"history,omitempty"`
	LastError   string           `json:"lastError,omitempty"`
}

type Group struct {
	ID   string `json:"id"`
	User string `json:"user"`
	// Empty means all buckets of this user. Bucket lists contain exact names, not globs.
	Buckets     []string      `json:"buckets,omitempty"`
	Members     []string      `json:"members"`
	Copies      int           `json:"copies"`
	MaxEventLag int           `json:"maxEventLag"`
	Mode        string        `json:"mode"`
	After       time.Duration `json:"after"`
	// Electorate is explicit and never shrinks when a proxy stops reporting.
	Electorate         []string `json:"electorate,omitempty"`
	MaxSwitchesPerHour int      `json:"maxSwitchesPerHour"`
	Primary            string   `json:"primary"`
	Generation         int64    `json:"generation"`
	Revision           int64    `json:"revision"`
	State              string   `json:"state"`
	Target             string   `json:"target,omitempty"`
	Automatic          bool     `json:"automatic,omitempty"`
	Frozen             bool     `json:"frozen"`
	// Seeded records successfully enqueued initial migrations by replication ID and generation.
	Seeded   map[string]int64 `json:"seeded,omitempty"`
	Verified map[string]int64 `json:"verified,omitempty"`
	// Quarantined members may contain writes newer than a lossy promotion; never overwrite automatically.
	Quarantined []string `json:"quarantined,omitempty"`
	History     []Event  `json:"history,omitempty"`
	LastError   string   `json:"lastError,omitempty"`
}

// Spec extracts the desired configuration state of the group.
func (g Group) Spec() GroupSpec {
	return GroupSpec{
		ID:                 g.ID,
		User:               g.User,
		Buckets:            slices.Clone(g.Buckets),
		Members:            slices.Clone(g.Members),
		Copies:             g.Copies,
		MaxEventLag:        g.MaxEventLag,
		Mode:               g.Mode,
		After:              g.After,
		Electorate:         slices.Clone(g.Electorate),
		MaxSwitchesPerHour: g.MaxSwitchesPerHour,
	}
}

// Promotion extracts the runtime promotion state of the group.
func (g Group) Promotion() PromotionState {
	var seeded, verified map[string]int64
	if g.Seeded != nil {
		seeded = make(map[string]int64, len(g.Seeded))
		for k, v := range g.Seeded {
			seeded[k] = v
		}
	}
	if g.Verified != nil {
		verified = make(map[string]int64, len(g.Verified))
		for k, v := range g.Verified {
			verified[k] = v
		}
	}
	return PromotionState{
		Primary:     g.Primary,
		Generation:  g.Generation,
		Revision:    g.Revision,
		State:       g.State,
		Target:      g.Target,
		Automatic:   g.Automatic,
		Frozen:      g.Frozen,
		Seeded:      seeded,
		Verified:    verified,
		Quarantined: slices.Clone(g.Quarantined),
		History:     slices.Clone(g.History),
		LastError:   g.LastError,
	}
}

// ApplySpec overwrites the desired configuration fields of g from spec.
func (g *Group) ApplySpec(spec GroupSpec) {
	g.ID = spec.ID
	g.User = spec.User
	g.Buckets = slices.Clone(spec.Buckets)
	g.Members = slices.Clone(spec.Members)
	g.Copies = spec.Copies
	g.MaxEventLag = spec.MaxEventLag
	g.Mode = spec.Mode
	g.After = spec.After
	g.Electorate = slices.Clone(spec.Electorate)
	g.MaxSwitchesPerHour = spec.MaxSwitchesPerHour
}

// ApplyPromotion overwrites the runtime promotion fields of g from p.
func (g *Group) ApplyPromotion(p PromotionState) {
	g.Primary = p.Primary
	g.Generation = p.Generation
	g.Revision = p.Revision
	g.State = p.State
	g.Target = p.Target
	g.Automatic = p.Automatic
	g.Frozen = p.Frozen
	g.Seeded = p.Seeded
	g.Verified = p.Verified
	g.Quarantined = slices.Clone(p.Quarantined)
	g.History = slices.Clone(p.History)
	g.LastError = p.LastError
}

// MergeGroup combines a desired GroupSpec and runtime PromotionState into a Group.
func MergeGroup(spec GroupSpec, p PromotionState) Group {
	var g Group
	g.ApplySpec(spec)
	g.ApplyPromotion(p)
	if g.Primary == "" && len(g.Members) > 0 {
		g.Primary = g.Members[0]
	}
	if g.Generation == 0 {
		g.Generation = 1
	}
	if g.State == "" {
		g.State = "initializing"
		g.Frozen = true
	}
	return g
}

func (s GroupSpec) Equal(other GroupSpec) bool {
	return s.ID == other.ID &&
		s.User == other.User &&
		slices.Equal(s.Buckets, other.Buckets) &&
		slices.Equal(s.Members, other.Members) &&
		s.Copies == other.Copies &&
		s.MaxEventLag == other.MaxEventLag &&
		s.Mode == other.Mode &&
		s.After == other.After &&
		slices.Equal(s.Electorate, other.Electorate) &&
		s.MaxSwitchesPerHour == other.MaxSwitchesPerHour
}

type Event struct {
	From       string    `json:"from"`
	To         string    `json:"to"`
	Generation int64     `json:"generation"`
	At         time.Time `json:"at"`
	Automatic  bool      `json:"automatic"`
	EventLag   int       `json:"eventLag"`
}

type MemberStatus struct {
	Storage     string `json:"storage"`
	Available   bool   `json:"available"`
	Initialized bool   `json:"initialized"`
	EventLag    int    `json:"eventLag"`
	Current     bool   `json:"current"`
}
type Status struct {
	Group         Group          `json:"group"`
	Members       []MemberStatus `json:"members"`
	HealthyCopies int            `json:"healthyCopies"`
	ActiveWriters int64          `json:"activeWriters"`
}

func (g *Group) Validate() error {
	bad := func(s string) error { return fmt.Errorf("%w: protection group: %s", dom.ErrInvalidArg, s) }
	for _, s := range append([]string{g.ID, g.User}, g.Members...) {
		if s == "" || strings.ContainsAny(s, ":/\x00 \t\r\n") {
			return bad("invalid or empty identifier")
		}
	}
	if len(g.Members) < 2 {
		return bad("at least two members required")
	}
	seen := map[string]bool{}
	for _, s := range g.Members {
		if seen[s] {
			return bad("duplicate member")
		}
		seen[s] = true
	}
	if g.Primary == "" {
		g.Primary = g.Members[0]
	}
	if !seen[g.Primary] {
		return bad("primary must be a member")
	}
	if g.Copies < 2 || g.Copies > len(g.Members) {
		return bad("copies must be between 2 and member count")
	}
	if g.MaxEventLag < 0 {
		return bad("negative maxEventLag")
	}
	if g.Mode == "" {
		g.Mode = "manual"
	}
	if g.Mode != "manual" && g.Mode != "auto" {
		return bad("mode must be manual or auto")
	}
	if g.After == 0 {
		g.After = 30 * time.Second
	}
	if g.After < 0 {
		return bad("negative after")
	}
	if g.MaxSwitchesPerHour == 0 {
		g.MaxSwitchesPerHour = 3
	}
	if g.MaxSwitchesPerHour < 1 {
		return bad("maxSwitchesPerHour must be positive")
	}
	seen = map[string]bool{}
	for _, b := range g.Buckets {
		if b == "" || strings.ContainsAny(b, "*?[]:/\x00 \t\r\n") || seen[b] {
			return bad("buckets must be unique exact names")
		}
		seen[b] = true
	}
	seen = map[string]bool{}
	for _, p := range g.Electorate {
		if p == "" || seen[p] {
			return bad("electorate must contain unique proxy IDs")
		}
		seen[p] = true
	}
	if g.Mode == "auto" && len(g.Electorate) < 3 {
		return bad("automatic failover needs at least three explicit proxy voters")
	}
	return nil
}

func (g Group) Covers(user, bucket string) bool {
	return g.User == user && (len(g.Buckets) == 0 || slices.Contains(g.Buckets, bucket))
}
func (g Group) Overlaps(other Group) bool {
	// S3 bucket names identify physical data independent of the Tutti identity. Conservatively
	// reject overlapping scopes on a common member even for different users; account-specific
	// namespaces cannot safely be inferred from access keys or bucket-glob expressions.
	if g.User != other.User {
		shared := false
		for _, m := range g.Members {
			if slices.Contains(other.Members, m) {
				shared = true
			}
		}
		if !shared {
			return false
		}
	}
	if len(g.Buckets) == 0 || len(other.Buckets) == 0 {
		return true
	}
	for _, b := range g.Buckets {
		if slices.Contains(other.Buckets, b) {
			return true
		}
	}
	return false
}
func (g Group) Replications(primary string) []entity.UniversalReplicationID {
	var out []entity.UniversalReplicationID
	for _, member := range g.Members {
		if member == primary {
			continue
		}
		if len(g.Buckets) == 0 {
			out = append(out, entity.UniversalFromUserReplication(entity.UserReplicationPolicy{User: g.User, FromStorage: primary, ToStorage: member}))
			continue
		}
		for _, b := range g.Buckets {
			out = append(out, entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: g.User, FromStorage: primary, ToStorage: member, FromBucket: b, ToBucket: b}))
		}
	}
	return out
}
