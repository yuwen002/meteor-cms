package handler

import (
	"net/http"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/contentbundle"
	"github.com/zeromicro/go-zero/core/logx"
)

// contentExportHandler 直接输出 JSON 文件，不走 { code, msg, data } 封装。
func contentExportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := contentbundle.Export(r.Context(), svcCtx.EntClient)
		if err != nil {
			logx.Errorf("导出内容包失败: %v", err)
			common.Fail(w, common.ErrInternalServer, "导出失败")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="cms-content-`+time.Now().Format("20060102")+`.json"`)
		w.Header().Set("Cache-Control", "no-store")
		if err := contentbundle.Write(w, b); err != nil {
			logx.Errorf("写出内容包失败: %v", err)
		}
	}
}

// contentImportHandler 请求体是内容包 JSON 本体，overwrite 通过 query 传入。
func contentImportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := contentbundle.Decode(r.Body)
		if err != nil {
			common.Fail(w, common.ErrInvalidParams, err.Error())
			return
		}
		v := r.URL.Query().Get("overwrite")
		overwrite := v == "1" || v == "true"
		res, err := contentbundle.Import(r.Context(), svcCtx.EntClient, b, overwrite)
		if err != nil {
			logx.Errorf("导入内容包失败: %v", err)
			common.Fail(w, common.ErrInternalServer, "导入失败，请检查内容包格式与 slug 冲突")
			return
		}
		// 内容包可能补进了配置与字典，写后立即失效读取缓存
		svcCtx.Settings.Invalidate()
		svcCtx.Dicts.Invalidate()
		common.Ok(w, res)
	}
}
