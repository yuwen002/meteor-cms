package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
)

// 公开阅读接口：只挂 ClientInfoMiddleware，不经过 JWT / 权限 / 操作日志中间件。
// 路径统一用 /api 前缀，与 SPAHandler 的排除规则保持一致。

func publicArticleListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.PublicArticleListReq) (interface{}, error) {
			return logic.NewPublicArticleLogic(r.Context(), svcCtx).PublicArticleList(req)
		})
	}
}

func publicArticleDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.PublicArticleDetailReq) (interface{}, error) {
			return logic.NewPublicArticleLogic(r.Context(), svcCtx).PublicArticleDetail(req)
		})
	}
}

func publicTaxonomyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.NewPublicArticleLogic(r.Context(), svcCtx).PublicTaxonomy()
		writeBizResult(w, resp, err)
	}
}
