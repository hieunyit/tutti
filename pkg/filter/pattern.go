// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package filter

import (
	"path"
	"strings"
)

// Filter specifies include and exclude glob patterns for object keys.
type Filter struct {
	IncludePatterns []string
	ExcludePatterns []string
}

// New creates a new Filter.
func New(include, exclude []string) *Filter {
	return &Filter{
		IncludePatterns: include,
		ExcludePatterns: exclude,
	}
}

// MatchGlob reports whether name matches a glob pattern.
func MatchGlob(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" || pattern == "**" {
		return true
	}
	// Prefix match like "data/*" where "data/file.txt" or "data/sub/file.txt" matches
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	if strings.HasSuffix(pattern, "/") {
		if strings.HasPrefix(name, pattern) {
			return true
		}
	}
	// Direct path.Match
	if matched, err := path.Match(pattern, name); err == nil && matched {
		return true
	}
	// Extension match like "*.jpg" against the base name or full path
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:]
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	base := path.Base(name)
	if matched, err := path.Match(pattern, base); err == nil && matched {
		return true
	}
	// Wildcard matching supporting multiple asterisks (e.g. backup-*-2026-*.sql or *error*.log)
	if strings.Contains(pattern, "*") {
		if matchWildcards(pattern, name) || matchWildcards(pattern, base) {
			return true
		}
	}
	return false
}

func matchWildcards(pattern, target string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == target
	}
	if !strings.HasPrefix(target, parts[0]) {
		return false
	}
	target = target[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(target, parts[i])
		if idx < 0 {
			return false
		}
		target = target[idx+len(parts[i]):]
	}
	return strings.HasSuffix(target, parts[len(parts)-1])
}

// ShouldSync evaluates if objectKey should be synced according to the filter rules.
// Exclude patterns take precedence over include patterns.
func (f *Filter) ShouldSync(objectKey string) bool {
	if f == nil {
		return true
	}
	for _, pattern := range f.ExcludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" && MatchGlob(pattern, objectKey) {
			return false
		}
	}
	hasInclude := false
	for _, pattern := range f.IncludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" {
			hasInclude = true
			if MatchGlob(pattern, objectKey) {
				return true
			}
		}
	}
	if !hasInclude {
		return true
	}
	return false
}
