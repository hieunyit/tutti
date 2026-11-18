// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package filter

import (
	"fmt"
	"testing"
)

func BenchmarkPatternFilter_ShouldSync(b *testing.B) {
	include := []string{"images/*", "data/**", "*.json", "logs/2026/04/*.log"}
	exclude := []string{"images/thumbnails/*", "*.tmp", "data/private/**"}
	f := New(include, exclude)

	keys := []string{
		"images/avatar.jpg",
		"images/thumbnails/thumb1.jpg",
		"data/analytics/q1.csv",
		"data/private/secrets.env",
		"configs/settings.json",
		"logs/2026/04/app-01.log",
		"logs/2026/05/app-01.log",
		"temp/cache.tmp",
		"unrelated/path/to/archive.zip",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		_ = f.ShouldSync(key)
	}
}

func BenchmarkMatchGlob_Prefix(b *testing.B) {
	pattern := "data/partition-12345/**"
	key := "data/partition-12345/subfolder/record-999.parquet"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = MatchGlob(pattern, key)
	}
}

func BenchmarkMatchGlob_WildcardExtension(b *testing.B) {
	pattern := "*.parquet"
	key := "data/warehouse/tables/customers/part-0001.parquet"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = MatchGlob(pattern, key)
	}
}

func BenchmarkPatternFilter_Parallel(b *testing.B) {
	include := []string{"uploads/*", "*.png", "*.jpg", "reports/**"}
	exclude := []string{"*.tmp", "uploads/trash/*"}
	f := New(include, exclude)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("uploads/user-%d/document-%d.jpg", i%1000, i)
			_ = f.ShouldSync(key)
			i++
		}
	})
}
