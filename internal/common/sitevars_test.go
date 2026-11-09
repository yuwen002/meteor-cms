package common

import (
	"testing"
	"time"
)

func TestExpandSiteText(t *testing.T) {
	now := time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		name     string
		text     string
		siteName string
		want     string
	}{
		{"年份模板", "© {year} Meteor CMS", "Meteor CMS", "© 2027 Meteor CMS"},
		{"站名占位符", "© {year} {site_name} · {site}", "我的博客", "© 2027 我的博客 · 我的博客"},
		{"无占位符原样输出", "© 2026 自定义版权", "我的博客", "© 2026 自定义版权"},
		{"未知占位符保留", "© {year} {unknown}", "我的博客", "© 2027 {unknown}"},
		{"空值", "", "我的博客", ""},
		{"站名含占位符不递归展开", "{site_name}", "{year}", "{year}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExpandSiteText(tc.text, now, tc.siteName); got != tc.want {
				t.Fatalf("ExpandSiteText(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func BenchmarkExpandSiteText(b *testing.B) {
	text := "© {year} {site_name} · 保留原样 {unknown}"
	for i := 0; i < b.N; i++ {
		_ = ExpandSiteText(text, time.Now(), "Meteor CMS")
	}
}
