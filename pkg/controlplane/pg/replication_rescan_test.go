// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

func TestReplicationRescanInterval(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos().Replications
	id := controlplane.ReplicationID{FromStorage: "s1", ToStorage: "s2", FromBucket: "b", ToBucket: "b"}

	require.NoError(t, r.Add(ctx, id, controlplane.ReplicationStatus{RescanSeconds: 3600}))
	st, err := r.GetStatus(ctx, id)
	require.NoError(t, err)
	require.EqualValues(t, 3600, st.RescanSeconds)

	require.NoError(t, r.SetRescanInterval(ctx, id, 900))
	st, _ = r.GetStatus(ctx, id)
	require.EqualValues(t, 900, st.RescanSeconds)
	all, err := r.ListStatuses(ctx, "")
	require.NoError(t, err)
	require.EqualValues(t, 900, all[id].RescanSeconds)

	// rename keeps it
	require.NoError(t, r.SetName(ctx, id, "n"))
	st, _ = r.GetStatus(ctx, id)
	require.EqualValues(t, 900, st.RescanSeconds)

	require.NoError(t, r.SetRescanInterval(ctx, id, 0))
	st, _ = r.GetStatus(ctx, id)
	require.EqualValues(t, 0, st.RescanSeconds)

	require.ErrorIs(t, r.SetRescanInterval(ctx, controlplane.ReplicationID{FromStorage: "x", ToStorage: "y"}, 900), dom.ErrNotFound)

	// a replication added without it reads back 0 (also rows created before migration 0006)
	other := controlplane.ReplicationID{FromStorage: "s1", ToStorage: "s3", FromBucket: "b", ToBucket: "b"}
	require.NoError(t, r.Add(ctx, other, controlplane.ReplicationStatus{}))
	st, err = r.GetStatus(ctx, other)
	require.NoError(t, err)
	require.EqualValues(t, 0, st.RescanSeconds)
}
