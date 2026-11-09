package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/loginlog"
	"github.com/yuwen002/meteor-cms/ent/operationlog"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/core/logx"
)

// maxLogExportRows 单次导出的上限，避免一次拉爆内存或产生超大响应
const maxLogExportRows = 10000

func operationLogItem(v *ent.OperationLog) types.OperationLogItem {
	return types.OperationLogItem{
		ID:         v.ID,
		UserID:     v.UserID,
		Username:   v.Username,
		Module:     v.Module,
		Action:     v.Action,
		Method:     v.Method,
		Path:       v.Path,
		IP:         v.IP,
		UserAgent:  v.UserAgent,
		Status:     v.Status,
		ErrorMsg:   v.ErrorMsg,
		DurationMs: v.DurationMs,
		Params:     v.Params,
		Query:      v.Query,
		RequestID:  v.RequestID,
		BizCode:    v.BizCode,
		HTTPStatus: v.HTTPStatus,
		CreatedAt:  v.CreatedAt.Format(time.RFC3339),
	}
}

func loginLogItem(v *ent.LoginLog) types.LoginLogItem {
	return types.LoginLogItem{
		ID:         v.ID,
		UserID:     v.UserID,
		Username:   v.Username,
		IP:         v.IP,
		UserAgent:  v.UserAgent,
		Status:     v.Status,
		FailReason: v.FailReason,
		RequestID:  v.RequestID,
		LoginAt:    v.LoginAt.Format(time.RFC3339),
	}
}

// scopedOperationLogQuery 每次返回全新 query，避免复用时谓词累积。
func scopedOperationLogQuery(ctx context.Context, svcCtx *svc.ServiceContext) (*ent.OperationLogQuery, error) {
	uid, scoped, err := logScopeUserID(ctx, svcCtx.EntClient)
	if err != nil {
		return nil, err
	}
	q := svcCtx.EntClient.OperationLog.Query()
	if scoped {
		q = q.Where(operationlog.UserIDEQ(uid))
	}
	return q, nil
}

func scopedLoginLogQuery(ctx context.Context, svcCtx *svc.ServiceContext) (*ent.LoginLogQuery, error) {
	uid, scoped, err := logScopeUserID(ctx, svcCtx.EntClient)
	if err != nil {
		return nil, err
	}
	q := svcCtx.EntClient.LoginLog.Query()
	if scoped {
		q = q.Where(loginlog.UserIDEQ(uid))
	}
	return q, nil
}

func applyOperationLogFilters(q *ent.OperationLogQuery, status *int, start, end string) *ent.OperationLogQuery {
	if status != nil {
		q = q.Where(operationlog.StatusEQ(*status))
	}
	if s, ok := parseLogTime(start); ok {
		q = q.Where(operationlog.CreatedAtGTE(s))
	}
	if e, ok := parseLogEndTime(end); ok {
		q = q.Where(operationlog.CreatedAtLTE(e))
	}
	return q
}

func applyLoginLogFilters(q *ent.LoginLogQuery, status *int, start, end string) *ent.LoginLogQuery {
	if status != nil {
		q = q.Where(loginlog.StatusEQ(*status))
	}
	if s, ok := parseLogTime(start); ok {
		q = q.Where(loginlog.LoginAtGTE(s))
	}
	if e, ok := parseLogEndTime(end); ok {
		q = q.Where(loginlog.LoginAtLTE(e))
	}
	return q
}

// OperationLogDetail 按 ID 查看操作日志，数据范围与列表一致。
func OperationLogDetail(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.OperationLogItem, error) {
	q, err := scopedOperationLogQuery(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	v, err := q.Where(operationlog.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// 不区分「不存在」与「无权查看」，避免探测他人日志是否存在
			return nil, common.NewBizError(common.ErrForbidden)
		}
		logx.Errorf("查询操作日志详情失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	item := operationLogItem(v)
	return &item, nil
}

// LoginLogDetail 按 ID 查看登录日志，数据范围与列表一致。
func LoginLogDetail(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.LoginLogItem, error) {
	q, err := scopedLoginLogQuery(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	v, err := q.Where(loginlog.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrForbidden)
		}
		logx.Errorf("查询登录日志详情失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	item := loginLogItem(v)
	return &item, nil
}

// OperationLogExportRows 返回导出用的操作日志行（已应用数据范围与筛选）。
func OperationLogExportRows(ctx context.Context, svcCtx *svc.ServiceContext, req types.LogExportReq) ([]types.OperationLogItem, error) {
	q, err := scopedOperationLogQuery(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	rows, err := applyOperationLogFilters(q, req.Status, req.StartTime, req.EndTime).
		Order(ent.Desc(operationlog.FieldCreatedAt)).
		Limit(maxLogExportRows).
		All(ctx)
	if err != nil {
		logx.Errorf("导出操作日志查询失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	out := make([]types.OperationLogItem, 0, len(rows))
	for _, v := range rows {
		out = append(out, operationLogItem(v))
	}
	return out, nil
}

// LoginLogExportRows 返回导出用的登录日志行。
func LoginLogExportRows(ctx context.Context, svcCtx *svc.ServiceContext, req types.LogExportReq) ([]types.LoginLogItem, error) {
	q, err := scopedLoginLogQuery(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	rows, err := applyLoginLogFilters(q, req.Status, req.StartTime, req.EndTime).
		Order(ent.Desc(loginlog.FieldLoginAt)).
		Limit(maxLogExportRows).
		All(ctx)
	if err != nil {
		logx.Errorf("导出登录日志查询失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	out := make([]types.LoginLogItem, 0, len(rows))
	for _, v := range rows {
		out = append(out, loginLogItem(v))
	}
	return out, nil
}

// OperationLogStats 按天 / 模块 / 业务码聚合操作日志，供后台趋势排查。
func OperationLogStats(ctx context.Context, svcCtx *svc.ServiceContext, days int) (*types.LogStatsResp, error) {
	if days < 1 {
		days = 7
	}
	if days > 30 {
		days = 30
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := today.AddDate(0, 0, -(days - 1))

	resp := &types.LogStatsResp{
		Days:      days,
		ByDay:     make([]types.LogDayStat, 0, days),
		ByModule:  make([]types.LogModuleStat, 0),
		ByBizCode: make([]types.LogBizCodeStat, 0),
	}

	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		next := day.AddDate(0, 0, 1)
		q, err := scopedOperationLogQuery(ctx, svcCtx)
		if err != nil {
			return nil, err
		}
		total, err := q.Where(operationlog.CreatedAtGTE(day), operationlog.CreatedAtLT(next)).Count(ctx)
		if err != nil {
			logx.Errorf("统计操作日志按天总数失败: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		failQ, err := scopedOperationLogQuery(ctx, svcCtx)
		if err != nil {
			return nil, err
		}
		failed, err := failQ.Where(
			operationlog.CreatedAtGTE(day),
			operationlog.CreatedAtLT(next),
			operationlog.StatusEQ(1),
		).Count(ctx)
		if err != nil {
			logx.Errorf("统计操作日志按天失败数失败: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		resp.ByDay = append(resp.ByDay, types.LogDayStat{
			Date: day.Format("2006-01-02"), Total: int64(total), Failed: int64(failed),
		})
		resp.Total += int64(total)
		resp.Failed += int64(failed)
	}
	resp.Success = resp.Total - resp.Failed

	moduleQ, err := scopedOperationLogQuery(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	var moduleRows []struct {
		Module string `json:"module"`
		Count  int    `json:"count"`
	}
	if err := moduleQ.Where(operationlog.CreatedAtGTE(start)).
		GroupBy(operationlog.FieldModule).Aggregate(ent.Count()).Scan(ctx, &moduleRows); err != nil {
		logx.Errorf("按模块聚合操作日志失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	for _, row := range moduleRows {
		resp.ByModule = append(resp.ByModule, types.LogModuleStat{Module: row.Module, Total: int64(row.Count)})
	}

	codeQ, err := scopedOperationLogQuery(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	var codeRows []struct {
		BizCode int `json:"biz_code"`
		Count   int `json:"count"`
	}
	if err := codeQ.Where(operationlog.CreatedAtGTE(start), operationlog.BizCodeNEQ(0)).
		GroupBy(operationlog.FieldBizCode).Aggregate(ent.Count()).Scan(ctx, &codeRows); err != nil {
		logx.Errorf("按业务码聚合操作日志失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	for _, row := range codeRows {
		resp.ByBizCode = append(resp.ByBizCode, types.LogBizCodeStat{BizCode: row.BizCode, Total: int64(row.Count)})
	}
	return resp, nil
}
