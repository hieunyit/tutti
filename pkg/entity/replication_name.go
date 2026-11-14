// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hieunyit/tutti/pkg/dom"
)

// MaxReplicationNameLen is the maximum number of characters (runes) of a replication display name.
const MaxReplicationNameLen = 64

// NormalizeReplicationName trims the optional display name of a replication and validates it. The empty name is
// valid (unnamed). The name is a label only: it is never part of the replication identity.
func NormalizeReplicationName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("%w: replication name is not valid UTF-8", dom.ErrInvalidArg)
	}
	if n := utf8.RuneCountInString(name); n > MaxReplicationNameLen {
		return "", fmt.Errorf("%w: replication name is too long: %d characters, max %d", dom.ErrInvalidArg, n, MaxReplicationNameLen)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: replication name must not contain control characters", dom.ErrInvalidArg)
		}
	}
	return name, nil
}
