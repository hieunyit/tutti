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

func TestParseRescanEvery(t *testing.T) {
	ok := map[string]time.Duration{
		"15m": 15 * time.Minute, "30m": 30 * time.Minute, "1h": time.Hour,
		"90m": 90 * time.Minute, "24h": 24 * time.Hour, " 6h ": 6 * time.Hour,
	}
	for in, want := range ok {
		got, err := parseRescanEvery("rescan-every", in, false)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}

	for _, in := range []string{"14m", "25h", "0", "off", "", "1d", "abc"} {
		_, err := parseRescanEvery("rescan-every", in, false)
		require.Error(t, err, in)
		require.Contains(t, err.Error(), "--rescan-every", in)
	}

	for _, in := range []string{"off", "OFF", "0"} {
		got, err := parseRescanEvery("every", in, true)
		require.NoError(t, err, in)
		require.Zero(t, got, in)
	}
	_, err := parseRescanEvery("every", "5m", true)
	require.ErrorContains(t, err, "between 15m and 24h")
}
