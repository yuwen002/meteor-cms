package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
)

// ---------- 审核流 ----------

func articleSubmitHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleIDReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleSubmit(req)
		})
	}
}

func articleApproveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticlePublishReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleApprove(req)
		})
	}
}

func articleRejectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleRejectReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleReject(req)
		})
	}
}

// ---------- 修订历史 ----------

func articleRevisionListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleIDReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleRevisionList(req)
		})
	}
}

func articleRevisionDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleRevisionIDReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleRevisionDetail(req)
		})
	}
}

func articleRevisionRestoreHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleRevisionIDReq) (interface{}, error) {
			return nil, logic.NewArticleLogic(r.Context(), svcCtx).ArticleRevisionRestore(req)
		})
	}
}

// ---------- 回收站 ----------

func trashListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.TrashListReq) (interface{}, error) {
			return logic.NewTrashLogic(r.Context(), svcCtx).TrashList(req)
		})
	}
}

func trashRestoreHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.TrashEntityReq) (interface{}, error) {
			return logic.NewTrashLogic(r.Context(), svcCtx).TrashRestore(req)
		})
	}
}

func trashPurgeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.TrashEntityReq) (interface{}, error) {
			return nil, logic.NewTrashLogic(r.Context(), svcCtx).TrashPurge(req)
		})
	}
}

// ---------- 批量操作 ----------

func articleBatchOfflineHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleBatchReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleBatchOffline(req)
		})
	}
}

func articleBatchDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleBatchReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleBatchDelete(req)
		})
	}
}

// ---------- 审核记录 ----------

func articleReviewListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.ArticleIDReq) (interface{}, error) {
			return logic.NewArticleLogic(r.Context(), svcCtx).ArticleReviewList(req)
		})
	}
}
