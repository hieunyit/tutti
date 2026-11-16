// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func TestReplicationSince(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos().Replications
	id := controlplane.ReplicationID{FromStorage: "s1", ToStorage: "s2", FromBucket: "b", ToBucket: "b"}
	since := time.Date(2026, 10, 17, 14, 3, 5, 123456000, time.UTC)

	require.NoError(t, r.Add(ctx, id, controlplane.ReplicationStatus{Since: &since}))
	st, err := r.GetStatus(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, st.Since)
	require.True(t, since.Equal(*st.Since))
	all, err := r.ListStatuses(ctx, "")
	require.NoError(t, err)
	require.True(t, since.Equal(*all[id].Since))

	// renaming keeps the cutoff
	require.NoError(t, r.SetName(ctx, id, "n"))
	st, _ = r.GetStatus(ctx, id)
	require.True(t, since.Equal(*st.Since))

	// a replication without a cutoff reads back nil (also the state of rows created before migration 0005)
	other := controlplane.ReplicationID{FromStorage: "s1", ToStorage: "s3", FromBucket: "b", ToBucket: "b"}
	require.NoError(t, r.Add(ctx, other, controlplane.ReplicationStatus{}))
	st, err = r.GetStatus(ctx, other)
	require.NoError(t, err)
	require.Nil(t, st.Since)
}
