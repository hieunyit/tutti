/*
 * Copyright © 2023 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 10, 17, 14, 3, 27, 0, time.UTC)
	plus7 := time.FixedZone("UTC+7", 7*3600)

	ok := map[string]string{
		"30d":                       "2026-09-17T14:03:27Z",
		" 7d ":                      "2026-10-10T14:03:27Z",
		"12h":                       "2026-10-17T02:03:27Z",
		"2026-10-17":                "2026-10-16T17:00:00Z", // local midnight in UTC+7
		"2026-10-17T10:00:00Z":      "2026-10-17T10:00:00Z",
		"2026-10-17T10:00:00+02:00": "2026-10-17T08:00:00Z",
		"2000-01-01T00:00:00Z":      "2000-01-01T00:00:00Z",
	}
	for in, want := range ok {
		got, err := parseSince(in, now, plus7)
		require.NoError(t, err, in)
		require.Equal(t, want, got.Format(time.RFC3339), in)
	}

	bad := map[string]string{
		"0d":                   "1 or more",
		"0h":                   "1 or more",
		"99999999999d":         "1 or more",
		"-3d":                  "not valid",
		"30":                   "not valid",
		"1w":                   "not valid",
		"yesterday":            "not valid",
		"":                     "not valid",
		"2026-10-18":           "future",
		"2026-10-17T14:03:28Z": "future",
		"1999-12-31":           "too old",
		"1999-12-31T23:59:59Z": "too old",
		"20000d":               "too old",
	}
	for in, msg := range bad {
		_, err := parseSince(in, now, plus7)
		require.Error(t, err, in)
		require.Contains(t, err.Error(), msg, in)
	}
}
