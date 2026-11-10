// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"time"
)

// Quorum requires a strict majority of a configured electorate, fresh reports, and an outage
// sustained for After in each affirmative report. Expired/missing voters never shrink the
// denominator. Unknown storage health and future-dated reports are not affirmative votes.
func Quorum(g Group, storage string, reports []proxystatus.Status, now time.Time, maxAge time.Duration) bool {
	if len(g.Electorate) < 3 {
		return false
	}
	votes := map[string]bool{}
	for _, r := range reports {
		if r.UpdatedAt.IsZero() || now.Sub(r.UpdatedAt) > maxAge || r.UpdatedAt.After(now) {
			continue
		}
		for _, st := range r.Storages {
			if st.Name == storage && !st.Available && !st.LastChange.IsZero() && !st.LastChange.After(now) && now.Sub(st.LastChange) >= g.After {
				votes[r.AppID] = true
			}
		}
	}
	n := 0
	for _, id := range g.Electorate {
		if votes[id] {
			n++
		}
	}
	return n > len(g.Electorate)/2
}
func Available(storage string, reports []proxystatus.Status, now time.Time, maxAge time.Duration) bool {
	for _, r := range reports {
		if !fresh(r, now, maxAge) {
			continue
		}
		for _, s := range r.Storages {
			if s.Name == storage && s.Available {
				return true
			}
		}
	}
	return false
}

func fresh(r proxystatus.Status, now time.Time, maxAge time.Duration) bool {
	return !r.UpdatedAt.IsZero() && now.Sub(r.UpdatedAt) <= maxAge && !r.UpdatedAt.After(now)
}
