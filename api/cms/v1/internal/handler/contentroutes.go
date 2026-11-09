package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

// RegisterContentRoutes 注册文章发布模块的路由。
//
// 独立于 goctl 生成的 routes.go：该文件带有 "DO NOT EDIT" 头，却被项目手工
// 追加过文件管理、字典/配置等路由段；重跑 make gen-api 会冲掉这些追加段。
// 新模块单独注册，既不动生成文件，也保证以后重新生成不会丢失文章路由。
//
// 后台接口挂四个中间件（与 routes.go 中受保护分组一致）：
// ClientInfo + Jwt + Permission + OperationLog。
// 公开阅读接口只挂 ClientInfoMiddleware，不经过 JWT 与权限校验。
func RegisterContentRoutes(server *rest.Server, serverCtx *svc.ServiceContext) {
	protected := []rest.Middleware{
		serverCtx.ClientInfoMiddleware,
		serverCtx.JwtMiddleware,
		serverCtx.PermissionMiddleware,
		serverCtx.OperationLogMiddleware,
	}

	server.AddRoutes(rest.WithMiddlewares(protected, []rest.Route{
		// ---------- 文章 ----------
		{Method: http.MethodGet, Path: "/admin/articles", Handler: articleListHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles", Handler: articleCreateHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/admin/articles/:id", Handler: articleDetailHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/admin/articles/:id", Handler: articleUpdateHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/admin/articles/:id", Handler: articleDeleteHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/:id/publish", Handler: articlePublishHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/:id/withdraw", Handler: articleWithdrawHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/:id/toggle-top", Handler: articleToggleTopHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/batch-offline", Handler: articleBatchOfflineHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/batch-delete", Handler: articleBatchDeleteHandler(serverCtx)},

		// ---------- 分类 ----------
		{Method: http.MethodGet, Path: "/admin/categories", Handler: categoryListHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/categories", Handler: categoryCreateHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/admin/categories/:id", Handler: categoryUpdateHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/admin/categories/:id", Handler: categoryDeleteHandler(serverCtx)},

		// ---------- 标签 ----------
		{Method: http.MethodGet, Path: "/admin/tags", Handler: tagListHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/tags", Handler: tagCreateHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/admin/tags/:id", Handler: tagUpdateHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/admin/tags/:id", Handler: tagDeleteHandler(serverCtx)},

		// ---------- 审核流 ----------
		{Method: http.MethodPost, Path: "/admin/articles/:id/submit", Handler: articleSubmitHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/:id/approve", Handler: articleApproveHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/:id/reject", Handler: articleRejectHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/admin/articles/:id/reviews", Handler: articleReviewListHandler(serverCtx)},

		// ---------- 修订历史 ----------
		{Method: http.MethodGet, Path: "/admin/articles/:id/revisions", Handler: articleRevisionListHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/admin/articles/:id/revisions/:rid", Handler: articleRevisionDetailHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/articles/:id/revisions/:rid/restore", Handler: articleRevisionRestoreHandler(serverCtx)},

		// ---------- 回收站 ----------
		{Method: http.MethodGet, Path: "/admin/trash", Handler: trashListHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/trash/:type/:id/restore", Handler: trashRestoreHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/admin/trash/:type/:id/purge", Handler: trashPurgeHandler(serverCtx)},

		// ---------- 内容导入导出（JSON 快照，单事务幂等导入） ----------
		{Method: http.MethodGet, Path: "/admin/content/export", Handler: contentExportHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/admin/content/import", Handler: contentImportHandler(serverCtx)},

		// ---------- 运行监控（管理员可见，不对外暴露） ----------
		{Method: http.MethodGet, Path: "/admin/monitor", Handler: monitorHandler(serverCtx)},
	}...))

	// ---------- 公开阅读：无需登录 ----------
	// 只挂 ClientInfoMiddleware，不经过 JWT / 权限 / 操作日志中间件。
	// /api 前缀同时被 SPAHandler 排除，刷新前台页面不会误回退到 API 404。
	server.AddRoutes(rest.WithMiddlewares(
		[]rest.Middleware{serverCtx.ClientInfoMiddleware},
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/articles", Handler: publicArticleListHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/search", Handler: publicSearchHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/articles/:slug", Handler: publicArticleDetailHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/taxonomy", Handler: publicTaxonomyHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/api/rss", Handler: rssHandler(serverCtx)},
			// 探活接口不属于 /api/ 前缀，避免被公开接口限流误伤。
			{Method: http.MethodGet, Path: "/healthz", Handler: healthHandler(serverCtx)},
			// 就绪探针额外校验 schema 版本，供滚动发布时阻断未迁移实例。
			{Method: http.MethodGet, Path: "/readyz", Handler: readyHandler(serverCtx)},
			// sitemap/robots 是站点级约定路径，注册为真实路由以优先于 SPA 回退。
			{Method: http.MethodGet, Path: "/sitemap.xml", Handler: sitemapHandler(serverCtx)},
			{Method: http.MethodGet, Path: "/robots.txt", Handler: robotsHandler(serverCtx)},
		}...,
	))
}
