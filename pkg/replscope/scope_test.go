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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func rs(from, to string, b ...string) Scope { return ReplicationScope(from, to, b) }

func TestConflicts(t *testing.T) {
	group := GroupScope([]string{"A", "B", "C"}, nil)
	tests := []struct {
		name string
		a, b Scope
		want bool
	}{
		{"same edge", rs("A", "B"), rs("A", "B"), true},
		{"same dest", rs("A", "B"), rs("C", "B"), true},
		{"reverse", rs("A", "B"), rs("B", "A"), true},
		{"same source other dest", rs("A", "B"), rs("A", "C"), false},
		{"chain disjoint buckets", rs("A", "B", "x"), rs("B", "C", "y"), false},
		{"chain same bucket needs one source", rs("A", "B", "x"), rs("B", "C", "x"), true},
		{"unrelated disjoint buckets", rs("A", "B", "x"), rs("C", "D", "y"), false},
		{"disjoint buckets", rs("A", "B", "x"), rs("A", "B", "y"), false},
		{"intersecting buckets", rs("A", "B", "x", "y"), rs("A", "B", "y"), true},
		{"all vs bucket", rs("A", "B"), rs("A", "B", "x"), true},
		{"group all vs A->D bucket", group, rs("A", "D", "x"), false},
		{"group vs C->A", group, rs("C", "A"), true},
		{"group vs A->B", group, rs("A", "B"), true},
		{"diff source same bucket", rs("A", "B", "b1"), rs("C", "D", "b1"), true},
		{"diff source both all", rs("A", "B"), rs("C", "D"), true},
		{"same source fan-out bucket", rs("A", "B", "b1"), rs("A", "C", "b1"), false},
		{"diff source all vs bucket", rs("A", "B"), rs("C", "D", "b1"), false},
		{"diff source disjoint buckets", rs("A", "B", "b1"), rs("C", "D", "b2"), false},
		{"group vs diff source all", GroupScope([]string{"A", "B"}, nil), rs("C", "D"), true},
		{"group bucket vs diff source bucket", GroupScope([]string{"A", "B"}, []string{"b1"}), rs("C", "D", "b1"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _, _ := Conflicts(tt.a, tt.b)
			require.Equal(t, tt.want, got)
			got, _, _, _ = Conflicts(tt.b, tt.a)
			require.Equal(t, tt.want, got, "symmetric")
		})
	}
}

func TestConflicts_Bucket(t *testing.T) {
	_, b, _, _ := Conflicts(rs("A", "B"), rs("A", "B"))
	require.Equal(t, "*", b)
	_, b, _, _ = Conflicts(rs("A", "B"), rs("A", "B", "x"))
	require.Equal(t, "x", b)
}

func TestGroupScope(t *testing.T) {
	s := GroupScope([]string{"A", "B", "C"}, nil)
	require.Len(t, s.Edges, 6)
	require.True(t, s.AllBuckets)
	s = GroupScope([]string{"A", "B"}, []string{"x"})
	require.False(t, s.AllBuckets)
}

func TestConflicts_Users(t *testing.T) {
	u := func(user string, s Scope) Scope { return s.WithUser(user) }
	tests := []struct {
		name string
		a, b Scope
		want bool
	}{
		{"alice ALL vs bob ALL", u("alice", rs("A", "B")), u("bob", rs("A", "B")), false},
		{"alice ALL vs bob explicit", u("alice", rs("A", "B")), u("bob", rs("A", "B", "x")), false},
		{"alice x vs bob x", u("alice", rs("A", "B", "x")), u("bob", rs("A", "B", "x")), true},
		{"alice x vs bob y", u("alice", rs("A", "B", "x")), u("bob", rs("A", "B", "y")), false},
		{"alice x vs bob x other edge", u("alice", rs("A", "B", "x")), u("bob", rs("C", "D", "x")), false},
		{"storage-level ALL vs alice ALL", rs("A", "B"), u("alice", rs("A", "B")), true},
		{"storage-level ALL vs alice x", rs("A", "B"), u("alice", rs("A", "B", "x")), true},
		{"alice ALL vs alice ALL", u("alice", rs("A", "B")), u("alice", rs("A", "B")), true},
		{"alice ALL vs alice x", u("alice", rs("A", "B")), u("alice", rs("A", "B", "x")), true},
		{"group alice vs bob ALL", u("alice", GroupScope([]string{"A", "B"}, nil)), u("bob", rs("A", "B")), false},
		{"group alice vs alice ALL", u("alice", GroupScope([]string{"A", "B"}, nil)), u("alice", rs("B", "A")), true},
		{"alice diff source same bucket", u("alice", rs("A", "B", "b1")), u("alice", rs("C", "D", "b1")), true},
		{"alice diff source both all", u("alice", rs("A", "B")), u("alice", rs("C", "D")), true},
		{"alice diff source all vs bucket", u("alice", rs("A", "B")), u("alice", rs("C", "D", "b1")), false},
		{"alice diff source disjoint buckets", u("alice", rs("A", "B", "b1")), u("alice", rs("C", "D", "b2")), false},
		{"alice same source fan-out", u("alice", rs("A", "B", "b1")), u("alice", rs("A", "C", "b1")), false},
		{"alice ALL vs bob ALL diff source", u("alice", rs("A", "B")), u("bob", rs("C", "D")), false},
		{"alice b1 vs bob b1 diff source", u("alice", rs("A", "B", "b1")), u("bob", rs("C", "D", "b1")), false},
		{"group no user vs bob ALL", GroupScope([]string{"A", "B"}, nil), u("bob", rs("A", "B")), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _, _ := Conflicts(tt.a, tt.b)
			require.Equal(t, tt.want, got)
			got, _, _, _ = Conflicts(tt.b, tt.a)
			require.Equal(t, tt.want, got, "symmetric")
		})
	}
}

func TestConflictReason(t *testing.T) {
	_, _, _, _, r := conflict2(rs("A", "B", "b1"), rs("C", "D", "b1"))
	require.Equal(t, ReasonSourceStorage, r)
	_, _, _, _, r = conflict2(rs("A", "B"), rs("C", "B"))
	require.Equal(t, ReasonOverlap, r)
	err := Check(rs("C", "D", "b1"), []Owner{{Kind: "replication", ID: "x", Scope: rs("A", "B", "b1")}}, "")
	require.Equal(t, ReasonSourceStorage, err.Reason)
	require.Contains(t, err.Error(), `storage-level replications must share one source storage per bucket: bucket "b1" is already replicated from "A" by replication "x"`)
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
	u := func(user string, s Scope) Scope { return s.WithUser(user) }
	ok, _, _, _ := Conflicts(u("alice", rs("A", "B")), u("bob", rs("C", "D")))
	require.False(t, ok)
	ok, _, _, _ = Conflicts(u("alice", rs("A", "B")), u("alice", rs("C", "D")))
	require.True(t, ok, "same user, both all, different sources")
	_, _, _, _, r = conflict2(u("alice", rs("A", "B", "b1")), u("alice", rs("C", "D", "b1")))
	require.Equal(t, ReasonSourceStorage, r)
	err = Check(u("alice", rs("C", "D", "b1")), []Owner{{Kind: "replication", ID: "x", Scope: u("alice", rs("A", "B", "b1"))}}, "")
	require.Equal(t, ReasonSourceStorage, err.Reason)
	require.Contains(t, err.Error(), `replications of user "alice" must share one source storage per bucket: bucket "b1" is already replicated from "A" by replication "x"`)
}
