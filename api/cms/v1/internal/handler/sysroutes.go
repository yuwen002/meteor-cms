package handler

import (
	"net/http"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterSystemExtraRoutes 注册配置增强接口。
// 独立于 goctl 生成的 routes.go；配置版本相关接口另用 /admin/config-versions 前缀，
// 避免与 /admin/configs/:id 产生路径参数歧义。
func RegisterSystemExtraRoutes(server *rest.Server, svcCtx *svc.ServiceContext) {
	protected := []rest.Middleware{
		svcCtx.ClientInfoMiddleware,
		svcCtx.JwtMiddleware,
		svcCtx.PermissionMiddleware,
		svcCtx.OperationLogMiddleware,
	}
	server.AddRoutes(rest.WithMiddlewares(protected, []rest.Route{
		{Method: http.MethodGet, Path: "/admin/configs/key/:key", Handler: configByKeyHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/admin/configs/:id", Handler: configDetailHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/admin/configs/batch", Handler: configBatchUpdateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/admin/config-versions/:id", Handler: configVersionsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/admin/config-versions/:id/rollback", Handler: configRollbackHandler(svcCtx)},
	}...))
}

// RegisterPublicSiteRoutes 注册面向前台的只读站点信息与字典接口。
// 只挂 ClientInfoMiddleware，不经过 JWT/权限中间件；字段白名单输出，绝不回传全量配置。
func RegisterPublicSiteRoutes(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.AddRoutes(rest.WithMiddlewares(
		[]rest.Middleware{svcCtx.ClientInfoMiddleware},
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/site", Handler: publicSiteHandler(svcCtx)},
			{Method: http.MethodGet, Path: "/api/dicts/:code", Handler: publicDictHandler(svcCtx)},
		}...,
	))
}

func configByKeyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigKeyReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		resp, err := logic.NewConfigLogic(r.Context(), svcCtx).ConfigByKey(req.Key)
		if err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func configDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		resp, err := logic.NewConfigLogic(r.Context(), svcCtx).ConfigDetail(req.ID)
		if err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func configBatchUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigBatchUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if err := logic.NewConfigLogic(r.Context(), svcCtx).ConfigBatchUpdate(&req); err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, types.CommonResp{Message: "配置已保存"})
	}
}

func configVersionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		resp, err := logic.NewConfigLogic(r.Context(), svcCtx).ConfigVersions(req.ID)
		if err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func configRollbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigRollbackReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if err := logic.NewConfigLogic(r.Context(), svcCtx).ConfigRollback(&req); err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, types.CommonResp{Message: "配置已回滚"})
	}
}

func publicSiteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values := svcCtx.Settings.All(r.Context())
		pick := func(key, fallback string) string {
			if v, ok := values[key]; ok && v != "" {
				return v
			}
			return fallback
		}
		// 后台保存的是原始模板（便于运营编辑），公开接口输出展开后的文本。
		name := pick("site_name", "Meteor CMS")
		common.Ok(w, types.SiteInfoResp{
			Name:           name,
			Logo:           pick("site_logo", ""),
			Description:    pick("site_description", ""),
			Keywords:       pick("site_keywords", ""),
			ICP:            pick("site_icp", ""),
			Footer:         common.ExpandSiteText(pick("site_footer", ""), time.Now(), name),
			Language:       pick("site_language", "zh-CN"),
			TitleTemplate:  pick("seo_title_template", "{title} - {site_name}"),
			Theme:          pick("theme_preset", "light"),
			PrimaryColor:   pick("theme_primary_color", "#409EFF"),
			CommentEnabled: siteBool(values, "feature_comment_enabled", false),
		})
	}
}

func publicDictHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PublicDictReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		items := svcCtx.Dicts.Items(r.Context(), req.Code)
		out := make([]types.PublicDictItem, 0, len(items))
		for _, it := range items {
			out = append(out, types.PublicDictItem{
				Label: it.Label, Value: it.Value,
				IsDefault: it.IsDefault, ParentID: it.ParentID, Sort: it.Sort,
			})
		}
		common.Ok(w, types.PublicDictResp{Code: req.Code, Items: out})
	}
}
