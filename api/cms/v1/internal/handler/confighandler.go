// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func configListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewConfigLogic(r.Context(), svcCtx)
		resp, err := l.ConfigList(&req)
		writeBizResult(w, resp, err)
	}
}

func configCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewConfigLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.ConfigCreate(&req))
	}
}

func configUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewConfigLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.ConfigUpdate(&req))
	}
}

func configDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConfigDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewConfigLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.ConfigDelete(&req))
	}
}
