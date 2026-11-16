// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestNormalizeRescanInterval(t *testing.T) {
	for _, tc := range []struct {
		in      time.Duration
		want    time.Duration
		invalid bool
	}{
		{0, 0, false},
		{MinRescanInterval, MinRescanInterval, false},
		{time.Hour, time.Hour, false},
		{MaxRescanInterval, MaxRescanInterval, false},
		{MinRescanInterval - time.Second, 0, true},
		{MaxRescanInterval + time.Second, 0, true},
		{-time.Hour, 0, true},
		{time.Second, 0, true},
		{time.Hour + 1500*time.Millisecond, time.Hour + time.Second, false},
	} {
		got, err := NormalizeRescanInterval(tc.in)
		if tc.invalid {
			require.ErrorIs(t, err, dom.ErrInvalidArg, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		require.Equal(t, tc.want, got)
	}
}
