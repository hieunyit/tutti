// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestNormalizeReplicationSince(t *testing.T) {
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	at := func(t time.Time) *time.Time { return &t }

	got, err := NormalizeReplicationSince(nil, now)
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = NormalizeReplicationSince(at(now.Add(-30*24*time.Hour)), now)
	require.NoError(t, err)
	require.Equal(t, now.Add(-30*24*time.Hour), *got)

	got, err = NormalizeReplicationSince(at(now), now)
	require.NoError(t, err, "now is not the future")
	require.NotNil(t, got)

	_, err = NormalizeReplicationSince(at(now.Add(time.Second)), now)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = NormalizeReplicationSince(at(time.Unix(0, 0)), now)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = NormalizeReplicationSince(at(MinReplicationSince.Add(-time.Second)), now)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = NormalizeReplicationSince(at(MinReplicationSince), now)
	require.NoError(t, err)
}
