package logic

import (
	"testing"
	"time"
)

func BenchmarkNormalizeSlug(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NormalizeSlug("  Foo_Meteor CMS 2026 发布说明 ")
	}
}

func BenchmarkGenerateSlug(b *testing.B) {
	now := time.Now()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateSlug("", "Meteor CMS 发布说明", now)
	}
}
