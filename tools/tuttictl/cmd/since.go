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
	"regexp"
	"strconv"
	"strings"
	"time"
)

// minSince is the earliest cutoff the server accepts (year 2000).
var minSince = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

var relativeSince = regexp.MustCompile(`^([0-9]+)([dh])$`)

// parseSince converts the --since value to an absolute UTC timestamp:
//
//	30d, 12h             that long before now
//	2026-10-17           the start of that day in the local timezone
//	2026-10-17T14:03:00Z RFC3339
//
// Future values, anything before year 2000 and unknown formats are rejected.
func parseSince(value string, now time.Time, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	var t time.Time
	if m := relativeSince.FindStringSubmatch(value); m != nil {
		n, err := strconv.ParseInt(m[1], 10, 32)
		if err != nil || n < 1 {
			return time.Time{}, fmt.Errorf("--since %q: the number of %s must be a whole number of 1 or more", value, unitName(m[2]))
		}
		unit := time.Hour
		if m[2] == "d" {
			unit = 24 * time.Hour
		}
		t = now.Add(-time.Duration(n) * unit)
	} else if d, err := time.ParseInLocation("2006-01-02", value, loc); err == nil {
		t = d
	} else if d, err := time.Parse(time.RFC3339, value); err == nil {
		t = d
	} else {
		return time.Time{}, fmt.Errorf("--since %q is not valid: use 30d, 12h, a date like 2026-10-17 or an RFC3339 time like 2026-10-17T14:03:00Z", value)
	}
	t = t.UTC().Truncate(time.Second)
	if t.After(now) {
		return time.Time{}, fmt.Errorf("--since %q is in the future: choose a moment that has already passed", value)
	}
	if t.Before(minSince) {
		return time.Time{}, fmt.Errorf("--since %q is too old: choose a date in the year 2000 or later", value)
	}
	return t, nil
}

func unitName(u string) string {
	if u == "h" {
		return "hours"
	}
	return "days"
}
