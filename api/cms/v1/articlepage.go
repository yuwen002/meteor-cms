package main

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/middleware"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/slugredirect"
	"github.com/zeromicro/go-zero/core/logx"
)

// newArticlePageResolver 给 SPAHandler 提供 /articles/:slug 的服务端页面元信息。
// 纯 SPA 只能在客户端改 document.title，爬虫抓不到 meta；这里在回退 index.html 时
// 由服务端注入 title/description/keywords/og 标签，并对历史 slug 返回 301。
//
// 查询失败一律降级为普通 SPA 页面（ok=false），绝不因为 SEO 查询异常阻断用户访问。
func newArticlePageResolver(client *ent.Client) middleware.PageResolver {
	return func(slug string) (middleware.PageData, bool) {
		if client == nil {
			return middleware.PageData{}, false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		now := time.Now()

		a, err := client.Article.Query().
			Where(article.StatusEQ(1), article.PublishedAtLTE(now), article.SlugEQ(slug)).
			Only(ctx)
		if err == nil {
			return middleware.PageData{
				Title:       pageFirstNonEmpty(a.SeoTitle, a.Title),
				Description: pageFirstNonEmpty(a.SeoDescription, a.Summary),
				Keywords:    a.SeoKeywords,
			}, true
		}
		if !ent.IsNotFound(err) {
			logx.Errorf("resolve article page meta failed: %v", err)
			return middleware.PageData{}, false
		}

		// 当前 slug 没有文章：查历史别名。只有目标文章仍然公开可见时才 301，
		// 避免把已下线 / 已删除文章的旧链接重定向到一个空白页。
		redirect, err := client.SlugRedirect.Query().
			Where(slugredirect.EntityTypeEQ("article"), slugredirect.OldSlugEQ(slug)).
			Only(ctx)
		if err != nil {
			return middleware.PageData{}, false
		}
		target, err := client.Article.Query().
			Where(article.IDEQ(redirect.EntityID), article.StatusEQ(1), article.PublishedAtLTE(now)).
			Only(ctx)
		if err != nil {
			return middleware.PageData{}, false
		}
		return middleware.PageData{Redirect: target.Slug}, true
	}
}

func pageFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
