package handler

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterLogRoutes 注册日志模块的详情 / 导出 / 统计接口。
//
// 独立于 goctl 生成的 routes.go，避免重跑 gen-api 时丢失。中间件与受保护分组一致。
// shouldSkipAudit 只跳过两个列表查询，因此这里的新接口都会被正常审计。
func RegisterLogRoutes(server *rest.Server, serverCtx *svc.ServiceContext) {
	protected := []rest.Middleware{
		serverCtx.ClientInfoMiddleware,
		serverCtx.JwtMiddleware,
		serverCtx.OperationLogMiddleware,
		serverCtx.PermissionMiddleware,
	}

	server.AddRoutes(rest.WithMiddlewares(protected, []rest.Route{
		{Method: http.MethodGet, Path: "/admin/logs/operations/:id", Handler: operationLogDetailHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/admin/logs/logins/:id", Handler: loginLogDetailHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/admin/logs/export", Handler: logExportHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/admin/logs/stats", Handler: logStatsHandler(serverCtx)},
	}...))
}

func writeLogBizError(w http.ResponseWriter, err error) {
	var bizErr *common.BizError
	if errors.As(err, &bizErr) {
		common.Fail(w, bizErr.Code, bizErr.Msg)
		return
	}
	common.Fail(w, common.ErrInternalServer, err.Error())
}

func operationLogDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LogIDReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		resp, err := logic.OperationLogDetail(r.Context(), svcCtx, req.ID)
		if err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func loginLogDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LogIDReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		resp, err := logic.LoginLogDetail(r.Context(), svcCtx, req.ID)
		if err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func logStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LogStatsReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		resp, err := logic.OperationLogStats(r.Context(), svcCtx, req.Days)
		if err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func logExportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LogExportReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if req.Type == "" {
			req.Type = "operation"
		}

		var header []string
		var rows [][]string
		switch req.Type {
		case "operation":
			items, err := logic.OperationLogExportRows(r.Context(), svcCtx, req)
			if err != nil {
				writeLogBizError(w, err)
				return
			}
			header = []string{
				"ID", "时间", "操作人", "模块", "操作", "方法", "路径", "结果",
				"业务码", "HTTP状态", "IP", "链路ID", "耗时(ms)", "参数快照", "Query快照", "失败原因",
			}
			for _, v := range items {
				rows = append(rows, []string{
					strconv.FormatInt(v.ID, 10), v.CreatedAt, v.Username, v.Module, v.Action,
					v.Method, v.Path, statusText(v.Status), strconv.Itoa(v.BizCode),
					strconv.Itoa(v.HTTPStatus), v.IP, v.RequestID,
					strconv.FormatInt(v.DurationMs, 10), v.Params, v.Query, v.ErrorMsg,
				})
			}
		case "login":
			items, err := logic.LoginLogExportRows(r.Context(), svcCtx, req)
			if err != nil {
				writeLogBizError(w, err)
				return
			}
			header = []string{"ID", "登录时间", "账号", "IP", "结果", "失败原因", "链路ID", "客户端"}
			for _, v := range items {
				rows = append(rows, []string{
					strconv.FormatInt(v.ID, 10), v.LoginAt, v.Username, v.IP,
					statusText(v.Status), v.FailReason, v.RequestID, v.UserAgent,
				})
			}
		default:
			common.Fail(w, common.ErrInvalidParams, "type 仅支持 operation 或 login")
			return
		}

		filename := fmt.Sprintf("%s-logs-%s.csv", req.Type, time.Now().Format("20060102"))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		// Excel 打开中文 CSV 需要 UTF-8 BOM
		_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

		cw := csv.NewWriter(w)
		_ = cw.Write(header)
		for i := range rows {
			for j := range rows[i] {
				rows[i][j] = csvSafeCell(rows[i][j])
			}
		}
		_ = cw.WriteAll(rows)
		cw.Flush()
	}
}

func statusText(status int) string {
	if status == 0 {
		return "成功"
	}
	return "失败"
}

// csvSafeCell 防止日志内容被 Excel 当成公式执行（CSV 注入）。
func csvSafeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		return "'" + s
	}
	return s
}
