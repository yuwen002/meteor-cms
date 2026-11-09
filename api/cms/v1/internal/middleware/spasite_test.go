package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const siteIndexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8" /><title>Build Title</title></head>
<body><div id="app"></div></body>
</html>`

func newSiteStaticDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(siteIndexHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func serveSPAWithSite(dir, path string, values map[string]string, resolver PageResolver) *httptest.ResponseRecorder {
	opts := []SPAOption{WithSiteValues(func(context.Context) map[string]string { return values })}
	if resolver != nil {
		opts = append(opts, WithPageResolver(resolver))
	}
	rec := httptest.NewRecorder()
	SPAHandler(dir, opts...).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// TestSPAHandler_SiteMeta 普通 SPA 页面也要带上站点级 title/description/keywords/lang。
func TestSPAHandler_SiteMeta(t *testing.T) {
	dir := newSiteStaticDir(t)
	rec := serveSPAWithSite(dir, "/dashboard", map[string]string{
		"site_name":        "我的站点",
		"site_description": "站点描述",
		"site_keywords":    "go,cms",
		"site_language":    "en-US",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /dashboard = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>我的站点</title>",
		`<meta name="description" content="站点描述" />`,
		`<meta name="keywords" content="go,cms" />`,
		`<html lang="en-US">`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("首屏缺少 %q, body=%q", want, body)
		}
	}
	if strings.Contains(body, "Build Title") {
		t.Fatalf("构建时的固定 title 未被替换, body=%q", body)
	}
}

// TestSPAHandler_WithoutSiteValues 未注入站点配置时必须保持纯静态托管行为。
func TestSPAHandler_WithoutSiteValues(t *testing.T) {
	dir := newSiteStaticDir(t)
	rec := httptest.NewRecorder()
	SPAHandler(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if rec.Body.String() != siteIndexHTML {
		t.Fatalf("未注入配置时 index.html 被改写: %q", rec.Body.String())
	}
}

// TestSPAHandler_ArticleTitleTemplate 文章页标题套模板，空描述/关键词回退到站点默认。
func TestSPAHandler_ArticleTitleTemplate(t *testing.T) {
	dir := newSiteStaticDir(t)
	resolver := func(slug string) (PageData, bool) {
		return PageData{Title: "文章标题"}, true
	}
	rec := serveSPAWithSite(dir, "/articles/hello", map[string]string{
		"site_name":               "我的站点",
		"seo_title_template":      "{title} | {site_name}",
		"seo_default_description": "默认描述",
		"site_keywords":           "默认关键词",
	}, resolver)
	body := rec.Body.String()
	for _, want := range []string{
		"<title>文章标题 | 我的站点</title>",
		`<meta name="description" content="默认描述" />`,
		`<meta name="keywords" content="默认关键词" />`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("文章页缺少 %q, body=%q", want, body)
		}
	}
}

func TestWithSiteTemplate_NoTitlePlaceholder(t *testing.T) {
	d := withSiteTemplate(PageData{Title: "文章标题"}, map[string]string{
		"site_name":          "我的站点",
		"seo_title_template": "{site_name}",
	})
	if d.Title != "文章标题" {
		t.Fatalf("模板缺少 {title} 时不应丢失文章标题, got %q", d.Title)
	}
}

func TestInjectPageMeta_LangSanitized(t *testing.T) {
	got := injectPageMeta(siteIndexHTML, PageData{Lang: `en" onload="alert(1)`})
	if strings.Contains(got, "onload=") || strings.Contains(got, `"alert`) {
		t.Fatalf("lang 配置被注入到标签属性: %q", got)
	}
	if !strings.Contains(got, `<html lang="`) {
		t.Fatalf("lang 属性丢失: %q", got)
	}
}

func TestInjectPageMeta_EscapesTitle(t *testing.T) {
	got := injectPageMeta(siteIndexHTML, PageData{Title: `<script>alert(1)</script>`})
	if strings.Contains(got, "<script>") {
		t.Fatalf("title 未转义: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("title 转义结果异常: %q", got)
	}
}
