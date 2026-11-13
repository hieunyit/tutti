// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package replscope

import (
	"fmt"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Edge is a directed replication edge between two storages.
type Edge struct{ From, To string }

// Scope is the set of directed edges and buckets a replication-producing object covers.
type Scope struct {
	Edges      []Edge
	Buckets    []string // ignored when AllBuckets
	AllBuckets bool
	// User is the real user whose buckets the scope covers; empty means storage-level (every bucket of the storage).
	User string
}

// WithUser returns a copy of the scope attributed to a real user (empty = storage-level).
func (s Scope) WithUser(user string) Scope {
	s.User = user
	return s
}

// Owner is an existing object that owns a Scope.
type Owner struct {
	Kind  string // "group" | "replication"
	ID    string // group id, or UniversalReplicationID.AsString()
	Scope Scope
}

// Reason tells the two kinds of conflict apart.
type Reason string

const (
	// ReasonOverlap: same destination or reversed edge on a common bucket.
	ReasonOverlap Reason = "overlap"
	// ReasonSourceStorage: two storage-level scopes with different source storages share a bucket (or are both
	// all-buckets). The policy stores keep one source storage per bucket, so they cannot coexist.
	ReasonSourceStorage Reason = "source_storage"
)

// ConflictError reports that a proposed scope overlaps an existing owner.
type ConflictError struct {
	OwnerKind, OwnerID, Bucket string
	Edge, OtherEdge            Edge
	Reason                     Reason
}

func (e *ConflictError) Error() string {
	if e.Reason == ReasonSourceStorage {
		if e.Bucket == "*" {
			return fmt.Sprintf("storage-level replications must share one source storage per bucket: all buckets are already replicated from %q by %s %q", e.OtherEdge.From, e.OwnerKind, e.OwnerID)
		}
		return fmt.Sprintf("storage-level replications must share one source storage per bucket: bucket %q is already replicated from %q by %s %q", e.Bucket, e.OtherEdge.From, e.OwnerKind, e.OwnerID)
	}
	return fmt.Sprintf("bucket %q on %s->%s is already covered by %s %q", e.Bucket, e.Edge.From, e.Edge.To, e.OwnerKind, e.OwnerID)
}

func (e *ConflictError) Unwrap() error { return dom.ErrAlreadyExists }

// GroupScope returns every ordered pair of distinct members; empty buckets means all buckets.
func GroupScope(members []string, buckets []string) Scope {
	var edges []Edge
	for _, a := range members {
		for _, b := range members {
			if a != b {
				edges = append(edges, Edge{From: a, To: b})
			}
		}
	}
	return Scope{Edges: edges, Buckets: buckets, AllBuckets: len(buckets) == 0}
}

// ReplicationScope returns a single-edge scope; empty buckets means all buckets.
func ReplicationScope(from, to string, buckets []string) Scope {
	return Scope{Edges: []Edge{{From: from, To: to}}, Buckets: buckets, AllBuckets: len(buckets) == 0}
}

func edgesClash(a, b Edge) bool {
	return a.To == b.To || (a.From == b.To && a.To == b.From)
}

func commonBucket(a, b Scope) (string, bool) {
	switch {
	case a.AllBuckets && b.AllBuckets:
		return "*", true
	case a.AllBuckets:
		return firstBucket(b.Buckets)
	case b.AllBuckets:
		return firstBucket(a.Buckets)
	}
	set := make(map[string]struct{}, len(b.Buckets))
	for _, x := range b.Buckets {
		set[x] = struct{}{}
	}
	for _, x := range a.Buckets {
		if _, ok := set[x]; ok {
			return x, true
		}
	}
	return "", false
}

func firstBucket(bs []string) (string, bool) {
	if len(bs) == 0 {
		return "", false
	}
	return bs[0], true
}

// Conflicts reports whether two scopes overlap, with the first offending bucket and edges (ea from a, eb from b).
// See conflict2 for the rules.
func Conflicts(a, b Scope) (conflict bool, bucket string, ea, eb Edge) {
	conflict, bucket, ea, eb, _ = conflict2(a, b)
	return
}

// conflict2 is Conflicts plus the reason.
// Two scopes of different real users overlap only on explicitly listed common buckets: "all buckets" of a real
// user means only that user's buckets. A storage-level scope (empty User) covers every bucket of the storage.
// Two storage-level scopes with different source storages also conflict on a common bucket (or both all-buckets):
// the policy stores keep one source storage per bucket (and per user), and storage-level ones share the empty user.
func conflict2(a, b Scope) (conflict bool, bucket string, ea, eb Edge, reason Reason) {
	var ok bool
	if a.User != "" && b.User != "" && a.User != b.User {
		if a.AllBuckets || b.AllBuckets {
			return
		}
	}
	bucket, ok = commonBucket(a, b)
	if !ok {
		return false, "", Edge{}, Edge{}, ""
	}
	for _, x := range a.Edges {
		for _, y := range b.Edges {
			if edgesClash(x, y) {
				return true, bucket, x, y, ReasonOverlap
			}
		}
	}
	if a.User == "" && b.User == "" && (a.AllBuckets == b.AllBuckets) {
		for _, x := range a.Edges {
			for _, y := range b.Edges {
				if x.From != y.From {
					return true, bucket, x, y, ReasonSourceStorage
				}
			}
		}
	}
	return false, "", Edge{}, Edge{}, ""
}
