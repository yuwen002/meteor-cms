package handler

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/ent"
)

// 站点级公开输出：RSS 2.0 / sitemap.xml / robots.txt。
// 三者都是纯文本协议，不走统一 JSON 响应封装。
// 站点名称、语言、公开根地址与 robots 规则均来自 SysConfig（经 settings 服务缓存）。

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func siteFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// siteBaseURL 站点绝对根地址：配置 site_url 优先，其次 YAML WebBaseURL，最后按请求 Host 兜底。
func siteBaseURL(r *http.Request, values map[string]string, fallback string) string {
	if v := strings.TrimSpace(values["site_url"]); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(fallback); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://" + r.Host
}

func siteBool(values map[string]string, key string, fallback bool) bool {
	v, ok := values[key]
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}
	return fallback
}

// robotPath 去掉换行/控制字符，防止配置值注入额外的 robots 指令行。
func robotPath(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func rssHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, base, ok := loadFeed(w, r, svcCtx, 50)
		if !ok {
			return
		}
		values := svcCtx.Settings.All(r.Context())
		base = siteBaseURL(r, values, base)
		pick := func(key, fallback string) string {
			return siteFirstNonEmpty(values[key], fallback)
		}
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		b.WriteString(`<rss version="2.0"><channel>`)
		b.WriteString("<title>" + xmlEscape(pick("site_name", "Meteor CMS")) + "</title>")
		b.WriteString("<link>" + xmlEscape(base+"/articles") + "</link>")
		b.WriteString("<description>" + xmlEscape(pick("site_description", "最新文章")) + "</description>")
		b.WriteString("<language>" + xmlEscape(pick("site_language", "zh-CN")) + "</language>")
		for _, a := range rows {
			link := base + "/articles/" + a.Slug
			b.WriteString("<item>")
			b.WriteString("<title>" + xmlEscape(a.Title) + "</title>")
			b.WriteString("<link>" + xmlEscape(link) + "</link>")
			b.WriteString(`<guid isPermaLink="true">` + xmlEscape(link) + "</guid>")
			if a.PublishedAt != nil {
				b.WriteString("<pubDate>" + a.PublishedAt.UTC().Format(time.RFC1123Z) + "</pubDate>")
			}
			b.WriteString("<description>" + xmlEscape(siteFirstNonEmpty(a.Summary, a.Title)) + "</description>")
			b.WriteString("</item>")
		}
		b.WriteString("</channel></rss>")
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	}
}

func sitemapHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, base, ok := loadFeed(w, r, svcCtx, 200)
		if !ok {
			return
		}
		base = siteBaseURL(r, svcCtx.Settings.All(r.Context()), base)
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		b.WriteString("<url><loc>" + xmlEscape(base+"/articles") + "</loc><changefreq>daily</changefreq><priority>1.0</priority></url>")
		for _, a := range rows {
			b.WriteString("<url><loc>" + xmlEscape(base+"/articles/"+a.Slug) + "</loc>")
			t := a.UpdatedAt
			if a.PublishedAt != nil {
				t = *a.PublishedAt
			}
			b.WriteString("<lastmod>" + t.UTC().Format("2006-01-02") + "</lastmod>")
			b.WriteString("<changefreq>weekly</changefreq><priority>0.8</priority></url>")
		}
		b.WriteString("</urlset>")
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	}
}

func robotsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values := svcCtx.Settings.All(r.Context())
		base := siteBaseURL(r, values, "")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(robotsText(values, base)))
	}
}

// robotsText 按站点配置生成 robots.txt；配置值经控制字符过滤，不能注入额外指令行。
func robotsText(values map[string]string, base string) string {
	var b strings.Builder
	b.WriteString("User-agent: *\nAllow: /\n")
	disallow := values["seo_robots_disallow"]
	if strings.TrimSpace(disallow) == "" {
		disallow = "/admin"
	}
	for _, p := range strings.FieldsFunc(disallow, func(c rune) bool {
		return c == ',' || c == '\n' || c == '\r'
	}) {
		if p = robotPath(strings.TrimSpace(p)); strings.HasPrefix(p, "/") {
			b.WriteString("Disallow: " + p + "\n")
		}
	}
	if siteBool(values, "seo_robots_sitemap", true) {
		b.WriteString("Sitemap: " + strings.TrimRight(base, "/") + "/sitemap.xml\n")
	}
	return b.String()
}

func loadFeed(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext, limit int) ([]*ent.Article, string, bool) {
	rows, base, err := logic.NewPublicArticleLogic(r.Context(), svcCtx).PublicFeed(limit)
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return nil, "", false
	}
	if base == "" {
		base = "http://" + r.Host
	}
	return rows, base, true
}
