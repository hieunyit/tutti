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
const FreezeKey = "tutti:protection:automatic-frozen"

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
