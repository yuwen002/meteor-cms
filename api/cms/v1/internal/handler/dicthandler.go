// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package handler

import (
	"errors"
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// writeBizResult 统一写回：业务错误用其 code/msg，其余按服务端错误处理
func writeBizResult(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		var bizErr *common.BizError
		if errors.As(err, &bizErr) {
			common.Fail(w, bizErr.Code, bizErr.Msg)
		} else {
			common.Fail(w, common.ErrInternalServer, err.Error())
		}
		return
	}
	common.Ok(w, data)
}

func dictListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictLogic(r.Context(), svcCtx)
		resp, err := l.DictList(&req)
		writeBizResult(w, resp, err)
	}
}

func dictCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.DictCreate(&req))
	}
}

func dictUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.DictUpdate(&req))
	}
}

func dictDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.DictDelete(&req))
	}
}

// ---------- 字典项 ----------

func dictItemListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictItemListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictItemLogic(r.Context(), svcCtx)
		resp, err := l.DictItemList(&req)
		writeBizResult(w, resp, err)
	}
}

func dictItemCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictItemCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictItemLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.DictItemCreate(&req))
	}
}

func dictItemUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictItemUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictItemLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.DictItemUpdate(&req))
	}
}

func dictItemDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DictItemDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		l := logic.NewDictItemLogic(r.Context(), svcCtx)
		writeBizResult(w, nil, l.DictItemDelete(&req))
	}
}
