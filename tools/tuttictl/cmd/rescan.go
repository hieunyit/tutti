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
	"fmt"
	"strings"
	"time"
)

const (
	minRescanEvery = 15 * time.Minute
	maxRescanEvery = 24 * time.Hour
)

// parseRescanEvery converts --rescan-every / --every to a duration. It accepts
// Go durations (15m, 1h, 90m, 1h30m); "off" and "0" return 0 only when allowOff
// is set. The server allows 15m..24h.
func parseRescanEvery(flag, value string, allowOff bool) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if allowOff && (strings.EqualFold(value, "off") || value == "0") {
		return 0, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		hint := "use 15m, 30m, 1h, 3h, 6h, 12h or 24h"
		if allowOff {
			hint += ", or off"
		}
		return 0, fmt.Errorf("--%s %q is not valid: %s", flag, value, hint)
	}
	if d < minRescanEvery || d > maxRescanEvery {
		return 0, fmt.Errorf("--%s %q is out of range: the interval must be between 15m and 24h", flag, value)
	}
	return d, nil
}
