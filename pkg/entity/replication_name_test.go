// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestNormalizeReplicationName(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		bad            bool
	}{
		{name: "empty", in: "", want: ""},
		{name: "blank is empty", in: "  \t ", want: ""},
		{name: "trimmed", in: "  prod → dr  ", want: "prod → dr"},
		{name: "64 runes", in: strings.Repeat("é", 64), want: strings.Repeat("é", 64)},
		{name: "65 runes", in: strings.Repeat("a", 65), bad: true},
		{name: "newline inside", in: "a\nb", bad: true},
		{name: "nul", in: "a\x00b", bad: true},
		{name: "invalid utf8", in: "a\xffb", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeReplicationName(tc.in)
			if tc.bad {
				require.ErrorIs(t, err, dom.ErrInvalidArg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
