package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/middleware"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

// RegisterSiteExtRoutes 注册站点级扩展模块（友链 / 评论 / 订阅）的路由。
// 后台管理接口走完整鉴权 + 审计链；公开接口只读或写入受功能开关约束，
// 不挂 JWT/权限中间件，与 /api/site 保持一致。
func RegisterSiteExtRoutes(server *rest.Server, svcCtx *svc.ServiceContext) {
	protected := []rest.Middleware{
		svcCtx.ClientInfoMiddleware,
		svcCtx.JwtMiddleware,
		svcCtx.PermissionMiddleware,
		svcCtx.OperationLogMiddleware,
	}
	server.AddRoutes(rest.WithMiddlewares(protected, []rest.Route{
		// 友链
		{Method: http.MethodGet, Path: "/admin/links", Handler: adminLinkListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/admin/links", Handler: adminLinkCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/admin/links/:id", Handler: adminLinkUpdateHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/admin/links/:id", Handler: adminLinkDeleteHandler(svcCtx)},
		// 评论审核
		{Method: http.MethodGet, Path: "/admin/comments", Handler: adminCommentListHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/admin/comments/:id/status", Handler: adminCommentStatusHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/admin/comments/:id", Handler: adminCommentDeleteHandler(svcCtx)},
		// 邮件订阅
		{Method: http.MethodGet, Path: "/admin/subscribers", Handler: adminSubscriberListHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/admin/subscribers/:id", Handler: adminSubscriberDeleteHandler(svcCtx)},
	}...))

	server.AddRoutes(rest.WithMiddlewares(
		[]rest.Middleware{svcCtx.ClientInfoMiddleware},
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/links", Handler: publicLinksHandler(svcCtx)},
			{Method: http.MethodGet, Path: "/api/comments/:slug", Handler: publicCommentsHandler(svcCtx)},
			{Method: http.MethodPost, Path: "/api/subscribe", Handler: publicSubscribeHandler(svcCtx)},
		}...,
	))

	// 评论提交是唯一允许匿名写入的公开接口，单独施加比全局 /api/ 限流更严格的配额。
	commentLimit := middleware.NewCommentRateLimitMiddleware(
		svcCtx.Redis,
		svcCtx.Config.Public.CommentRate.Max,
		svcCtx.Config.Public.CommentRate.Window,
	)
	server.AddRoutes(rest.WithMiddlewares(
		[]rest.Middleware{svcCtx.ClientInfoMiddleware, commentLimit},
		[]rest.Route{
			{Method: http.MethodPost, Path: "/api/comments", Handler: publicCommentCreateHandler(svcCtx)},
		}...,
	))
}
