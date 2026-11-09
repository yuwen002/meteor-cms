package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSiteBaseURL(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "fallback.example.com"
	cases := []struct {
		name     string
		values   map[string]string
		fallback string
		want     string
	}{
		{"配置优先", map[string]string{"site_url": "https://site.example.com/"}, "https://yaml.example.com", "https://site.example.com"},
		{"配置文件兜底", map[string]string{}, "https://yaml.example.com/", "https://yaml.example.com"},
		{"请求 Host 兜底", map[string]string{}, "", "http://fallback.example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := siteBaseURL(r, c.values, c.fallback); got != c.want {
				t.Fatalf("siteBaseURL() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSiteBool(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "off", "FALSE"} {
		if siteBool(map[string]string{"k": v}, "k", true) {
			t.Fatalf("siteBool(%q) 应为 false", v)
		}
	}
	for _, v := range []string{"1", "true", "yes", "on", "TRUE"} {
		if !siteBool(map[string]string{"k": v}, "k", false) {
			t.Fatalf("siteBool(%q) 应为 true", v)
		}
	}
	if !siteBool(map[string]string{}, "k", true) || siteBool(map[string]string{"k": "bad"}, "k", false) {
		t.Fatal("缺省/非法值应使用 fallback")
	}
}

func TestRobotsText(t *testing.T) {
	got := robotsText(map[string]string{}, "https://site.example.com/")
	if !strings.Contains(got, "Disallow: /admin\n") {
		t.Fatalf("默认应禁止 /admin, got %q", got)
	}
	if !strings.Contains(got, "Sitemap: https://site.example.com/sitemap.xml\n") {
		t.Fatalf("默认应声明 sitemap, got %q", got)
	}

	got = robotsText(map[string]string{
		"seo_robots_disallow": "/admin, /tmp\n/private",
		"seo_robots_sitemap":  "false",
	}, "https://site.example.com")
	for _, want := range []string{"Disallow: /admin\n", "Disallow: /tmp\n", "Disallow: /private\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("缺少 %q, got %q", want, got)
		}
	}
	if strings.Contains(got, "Sitemap:") {
		t.Fatalf("关闭后不应声明 sitemap, got %q", got)
	}
}

// TestRobotsText_RejectsDirectiveInjection 配置值不能变成任意 robots 指令。
func TestRobotsText_RejectsDirectiveInjection(t *testing.T) {
	got := robotsText(map[string]string{
		"seo_robots_disallow": "/admin\nSitemap: http://evil.example",
	}, "https://site.example.com")
	if strings.Contains(got, "evil.example") {
		t.Fatalf("配置注入了额外的 robots 指令: %q", got)
	}
	if !strings.Contains(got, "Disallow: /admin\n") {
		t.Fatalf("合法路径被误丢弃: %q", got)
	}
}
