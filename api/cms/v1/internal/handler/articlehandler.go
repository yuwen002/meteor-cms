package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// bindAndWrite 统一完成参数解析与响应回写，供文章模块的 handler 复用。
// writeBizResult 会把 *common.BizError 展开为业务错误码，其余按服务端错误处理。
func bindAndWrite[T any](w http.ResponseWriter, r *http.Request, fn func(*T) (interface{}, error)) {
	var req T
	if err := httpx.Parse(r, &req); err != nil {
		common.Fail(w, common.ErrInvalidParams, err.Error())
		return
	}
	resp, err := fn(&req)
	writeBizResult(w, resp, err)
}

func articleListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleListReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleList(req)
		})
	}
}

func articleDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleIDReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleDetail(req)
		})
	}
}

func articleCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleCreateReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleCreate(req)
		})
	}
}

func articleUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleUpdateReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleUpdate(req)
		})
	}
}

func articleDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleIDReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleDelete(req)
		})
	}
}

func articlePublishHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticlePublishReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticlePublish(req)
		})
	}
}

func articleWithdrawHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleIDReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleWithdraw(req)
		})
	}
}

func articleToggleTopHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleIDReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleToggleTop(req)
		})
	}
}
