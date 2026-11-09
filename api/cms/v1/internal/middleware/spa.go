package middleware

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PageData 服务端注入到 SPA index.html 的页面元信息。
// Redirect 非空时优先返回 301，用于文章 slug 变更后的旧链接跳转。
type PageData struct {
	Title       string
	Description string
	Keywords    string
	Lang        string
	Redirect    string
}

// PageResolver 按文章 slug 解析页面元信息；ok=false 表示按普通 SPA 页面处理。
type PageResolver func(slug string) (PageData, bool)

// SiteValuesFunc 返回站点配置的 key -> 明文值快照。
// 由 main 包延迟注入，避免 middleware 反向依赖 svc 包形成循环引用。
type SiteValuesFunc func(ctx context.Context) map[string]string

type spaOptions struct {
	resolver   PageResolver
	siteValues SiteValuesFunc
}

// SPAOption 是 SPAHandler 的可选配置
type SPAOption func(*spaOptions)

// WithPageResolver 为 /articles/:slug 开启服务端 meta 注入与历史 slug 301。
// 不传时 SPAHandler 行为与纯静态托管完全一致（http.ServeFile）。
func WithPageResolver(fn PageResolver) SPAOption {
	return func(o *spaOptions) { o.resolver = fn }
}

// WithSiteValues 让站点名称、SEO 默认值、标题模板与语言作用到首屏 HTML。
// 未注入时不会改写 index.html，保持原有纯静态托管行为。
func WithSiteValues(fn SiteValuesFunc) SPAOption {
	return func(o *spaOptions) { o.siteValues = fn }
}

var titleTagRe = regexp.MustCompile(`(?is)<title[^>]*>.*?</title>`)
var langAttrRe = regexp.MustCompile(`(?is)(<html[^>]*\blang\s*=\s*")[^"]*(")`)
var langValueRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// SPAHandler 在配置了 StaticDir 时由本服务直接托管前端 SPA，作为自定义 404(NotFound) 处理器使用：
//   - 跳过 API 前缀（/admin、/api）与静态资源前缀（/uploads），交由 go-zero 路由处理；
//   - 其余 GET 请求：若路径对应真实文件（如 /assets/index-xxx.js）则直接返回，
//     否则回退到 index.html，以支持前端 history 路由（如 /system/roles 刷新不 404）。
//   - 配置了 PageResolver 时，/articles/:slug 会由服务端注入 SEO meta（爬虫可抓取），
//     历史 slug 则返回 301 到新地址。
//   - 配置了 SiteValues 时，所有回退页面都会注入站点级 title/description/keywords/lang，
//     文章页标题还会套用 seo_title_template。
//
// 必须以 rest.WithNotFoundHandler 注册，而不是 server.Use：
// go-zero 的 server.Use 注册的全局中间件不会作用于 404（未匹配）路由，
// 而 notFoundHandler 在 MustNewServer 时即绑定中间件链，早于 server.Use 调用，
// 因此用 server.Use 兜底的 SPA 中间件对 /、/admin/users 这类未匹配路径不会生效。
func SPAHandler(staticDir string, opts ...SPAOption) http.Handler {
	cfg := &spaOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 未配置产物目录、或非 GET 请求：交回 go-zero 默认 404
		if staticDir == "" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		path := r.URL.Path
		if strings.HasPrefix(path, "/admin") ||
			strings.HasPrefix(path, "/api") ||
			strings.HasPrefix(path, "/uploads") {
			http.NotFound(w, r)
			return
		}

		// 尝试直接返回真实文件（去掉查询串并防目录穿越）
		clean := strings.TrimPrefix(path, "/")
		if clean != "" {
			filePath := filepath.Join(staticDir, filepath.Clean(clean))
			if isInside(staticDir, filePath) {
				if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
					http.ServeFile(w, r, filePath)
					return
				}
			}
		}

		// SPA history 路由回退到 index.html
		index := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			var values map[string]string
			if cfg.siteValues != nil {
				values = cfg.siteValues(r.Context())
			}
			if cfg.resolver != nil {
				if slug, ok := articleSlugFromPath(path); ok {
					if data, found := cfg.resolver(slug); found {
						if data.Redirect != "" {
							http.Redirect(w, r, "/articles/"+url.PathEscape(data.Redirect), http.StatusMovedPermanently)
							return
						}
						serveInjectedIndex(w, r, index, withSiteTemplate(data, values))
						return
					}
				}
			}
			// 普通 SPA 页面也要带上站点级 meta，避免首页/后台 title 永远是构建时的固定值
			if d := defaultPageData(values); d.Title != "" || d.Description != "" || d.Keywords != "" || d.Lang != "" {
				serveInjectedIndex(w, r, index, d)
				return
			}
			http.ServeFile(w, r, index)
			return
		}

		// 既无对应文件、也无 index.html：返回 404
		http.NotFound(w, r)
	})
}

func siteValue(values map[string]string, key, fallback string) string {
	if v, ok := values[key]; ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// defaultPageData 生成非文章页的站点级默认 meta。
func defaultPageData(values map[string]string) PageData {
	return PageData{
		Title: siteValue(values, "site_name", ""),
		Description: firstNonEmpty(
			siteValue(values, "seo_default_description", ""),
			siteValue(values, "site_description", ""),
		),
		Keywords: siteValue(values, "site_keywords", ""),
		Lang:     siteValue(values, "site_language", ""),
	}
}

// withSiteTemplate 合并文章页 meta 与站点配置：标题模板、默认描述/关键词、html lang。
func withSiteTemplate(data PageData, values map[string]string) PageData {
	siteName := siteValue(values, "site_name", "Meteor CMS")
	title := firstNonEmpty(data.Title, siteName)
	tpl := siteValue(values, "seo_title_template", "{title} - {site_name}")
	if strings.Contains(tpl, "{title}") {
		data.Title = strings.NewReplacer(
			"{title}", title,
			"{site_name}", siteName,
			"{site}", siteName,
		).Replace(tpl)
	} else {
		// 模板忘了写 {title} 时不能让所有文章页共用同一个标题
		data.Title = title
	}
	data.Description = firstNonEmpty(
		data.Description,
		siteValue(values, "seo_default_description", ""),
		siteValue(values, "site_description", ""),
	)
	data.Keywords = firstNonEmpty(data.Keywords, siteValue(values, "site_keywords", ""))
	data.Lang = siteValue(values, "site_language", "")
	return data
}

func articleSlugFromPath(p string) (string, bool) {
	const prefix = "/articles/"
	if !strings.HasPrefix(p, prefix) {
		return "", false
	}
	slug := strings.TrimPrefix(p, prefix)
	if slug == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return slug, true
}

func serveInjectedIndex(w http.ResponseWriter, r *http.Request, index string, data PageData) {
	body, err := os.ReadFile(index)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(injectPageMeta(string(body), data)))
}

// injectPageMeta 所有动态值都经 html.EscapeString，避免文章标题/SEO 字段注入标签。
func injectPageMeta(page string, d PageData) string {
	if d.Lang != "" {
		lang := langValueRe.ReplaceAllString(d.Lang, "")
		if lang != "" && langAttrRe.MatchString(page) {
			page = langAttrRe.ReplaceAllString(page, "${1}"+lang+"${2}")
		}
	}
	tags := make([]string, 0, 5)
	if d.Title != "" {
		esc := html.EscapeString(d.Title)
		titleTag := "<title>" + esc + "</title>"
		if titleTagRe.MatchString(page) {
			page = titleTagRe.ReplaceAllLiteralString(page, titleTag)
		} else {
			tags = append(tags, titleTag)
		}
		tags = append(tags, `<meta property="og:title" content="`+esc+`" />`)
	}
	if d.Description != "" {
		esc := html.EscapeString(d.Description)
		tags = append(tags, `<meta name="description" content="`+esc+`" />`)
		tags = append(tags, `<meta property="og:description" content="`+esc+`" />`)
	}
	if d.Keywords != "" {
		tags = append(tags, `<meta name="keywords" content="`+html.EscapeString(d.Keywords)+`" />`)
	}
	if len(tags) == 0 {
		return page
	}
	injected := strings.Join(tags, "\n")
	if idx := strings.Index(strings.ToLower(page), "</head>"); idx >= 0 {
		return page[:idx] + injected + "\n" + page[idx:]
	}
	return injected + "\n" + page
}

// isInside 判断 target 是否位于 root 目录内，避免 ../ 穿越读取任意文件。
func isInside(root, target string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rootWithSep := absRoot + string(os.PathSeparator)
	return absTarget == absRoot || strings.HasPrefix(absTarget, rootWithSep)
}
