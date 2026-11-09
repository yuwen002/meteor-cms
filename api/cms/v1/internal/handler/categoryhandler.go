package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
)

func categoryListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.CategoryListReq) (interface{}, error) {
			return logic.NewCategoryLogic(r.Context(), svcCtx).CategoryList(req)
		})
	}
}

func categoryCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.CategoryCreateReq) (interface{}, error) {
			return logic.NewCategoryLogic(r.Context(), svcCtx).CategoryCreate(req)
		})
	}
}

func categoryUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.CategoryUpdateReq) (interface{}, error) {
			return nil, logic.NewCategoryLogic(r.Context(), svcCtx).CategoryUpdate(req)
		})
	}
}

func categoryDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bindAndWrite(w, r, func(req *types.CategoryIDReq) (interface{}, error) {
			return nil, logic.NewCategoryLogic(r.Context(), svcCtx).CategoryDelete(req)
		})
	}
}
