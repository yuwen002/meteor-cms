package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
)

func tagListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.TagListReq) (interface{}, error) {
			return logic.NewTagLogic(r.Context(), svcCtx).TagList(req)
		})
	}
}

func tagCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.TagCreateReq) (interface{}, error) {
			return logic.NewTagLogic(r.Context(), svcCtx).TagCreate(req)
		})
	}
}

func tagUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.TagUpdateReq) (interface{}, error) {
			return nil, logic.NewTagLogic(r.Context(), svcCtx).TagUpdate(req)
		})
	}
}

func tagDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.TagIDReq) (interface{}, error) {
			return nil, logic.NewTagLogic(r.Context(), svcCtx).TagDelete(req)
		})
	}
}
