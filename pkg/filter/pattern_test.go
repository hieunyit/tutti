// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"", "anything.txt", true},
		{"*", "anything.txt", true},
		{"**", "path/to/anything.txt", true},
		{"data/*", "data/file.txt", true},
		{"data/*", "data/sub/file.txt", true},
		{"data/*", "other/file.txt", false},
		{"uploads/", "uploads/file.txt", true},
		{"uploads/", "other/file.txt", false},
		{"*.jpg", "image.jpg", true},
		{"*.jpg", "path/to/image.jpg", true},
		{"*.jpg", "image.png", false},
		{"*.tar.gz", "archive.tar.gz", true},
		{"*.tar.gz", "path/to/archive.tar.gz", true},
		{"*.tar.gz", "archive.zip", false},
		{"backup-*-2026.sql", "backup-prod-2026.sql", true},
		{"backup-*-2026.sql", "backup-test-2025.sql", false},
		{"backup-*-*-2026.sql", "backup-prod-asia-2026.sql", true},
		{"backup-*-*-2026.sql", "backup-prod-2026.sql", false},
		{"*error*.log", "app-error-dump.log", true},
		{"*error*.log", "path/to/app-error-dump.log", true},
		{"*error*.log", "app-info.log", false},
	}

	for _, tt := range tests {
		got := MatchGlob(tt.pattern, tt.name)
		assert.Equal(t, tt.want, got, "pattern: %s, name: %s", tt.pattern, tt.name)
	}
}

func TestFilter_ShouldSync(t *testing.T) {
	t.Run("nil filter syncs everything", func(t *testing.T) {
		var f *Filter
		assert.True(t, f.ShouldSync("any/path/file.txt"))
	})

	t.Run("empty filter syncs everything", func(t *testing.T) {
		f := New(nil, nil)
		assert.True(t, f.ShouldSync("any/path/file.txt"))
	})

	t.Run("empty strings in include patterns syncs everything", func(t *testing.T) {
		f := New([]string{"", "   "}, []string{"", "   "})
		assert.True(t, f.ShouldSync("any/path/file.txt"))
	})

	t.Run("exclude only", func(t *testing.T) {
		f := New(nil, []string{"temp/*", "*.tmp", "logs/*"})
		assert.False(t, f.ShouldSync("temp/file.txt"))
		assert.False(t, f.ShouldSync("data/test.tmp"))
		assert.False(t, f.ShouldSync("logs/app.log"))
		assert.True(t, f.ShouldSync("data/production.db"))
	})

	t.Run("include only", func(t *testing.T) {
		f := New([]string{"data/*", "*.jpg"}, nil)
		assert.True(t, f.ShouldSync("data/doc.pdf"))
		assert.True(t, f.ShouldSync("images/photo.jpg"))
		assert.False(t, f.ShouldSync("images/photo.png"))
		assert.False(t, f.ShouldSync("other/random.txt"))
	})

	t.Run("exclude overrides include", func(t *testing.T) {
		f := New([]string{"data/*", "*.jpg"}, []string{"data/temp/*", "*.tmp"})
		assert.True(t, f.ShouldSync("data/valid.jpg"))
		assert.False(t, f.ShouldSync("data/temp/test.jpg")) // matched exclude
		assert.False(t, f.ShouldSync("data/valid.tmp"))    // matched exclude
	})
}
