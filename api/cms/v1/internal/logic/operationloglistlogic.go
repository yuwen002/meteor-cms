// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/operationlog"
	"github.com/yuwen002/meteor-cms/ent/predicate"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type OperationLogListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOperationLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OperationLogListLogic {
	return &OperationLogListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OperationLogListLogic) OperationLogList(req *types.OperationLogListReq) (resp *types.OperationLogListResp, err error) {
	offset, pageSize := utils.NormalizePage(req.Page, req.PageSize, 20)
	page := offset/pageSize + 1

	query := l.svcCtx.EntClient.OperationLog.Query()

	conds := make([]predicate.OperationLog, 0, 8)
	// 数据权限：超管或持有 system:log:view_all 的审计员可看全量，其余人只能看自己；
	// user_id 仅对全量可见者生效，避免越权检索他人日志。
	uid, scoped, err := logScopeUserID(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("判定日志可见范围失败: %v", err)
		return nil, err
	}
	if scoped {
		conds = append(conds, operationlog.UserIDEQ(uid))
	} else if req.UserID > 0 {
		conds = append(conds, operationlog.UserIDEQ(req.UserID))
	}
	if req.Username != "" {
		conds = append(conds, operationlog.UsernameContainsFold(req.Username))
	}
	if req.Module != "" {
		conds = append(conds, operationlog.ModuleEQ(req.Module))
	}
	if req.Path != "" {
		conds = append(conds, operationlog.PathContainsFold(req.Path))
	}
	// Status 为指针：0（成功）与「未筛选」必须区分开
	if req.Status != nil {
		conds = append(conds, operationlog.StatusEQ(*req.Status))
	}
	if start, ok := parseLogTime(req.StartTime); ok {
		conds = append(conds, operationlog.CreatedAtGTE(start))
	}
	if end, ok := parseLogEndTime(req.EndTime); ok {
		conds = append(conds, operationlog.CreatedAtLTE(end))
	}
	if len(conds) > 0 {
		query = query.Where(operationlog.And(conds...))
	}

	total, err := query.Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询操作日志总数失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	list, err := query.
		Order(ent.Desc(operationlog.FieldCreatedAt)).
		Offset(offset).
		Limit(pageSize).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询操作日志失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	items := make([]types.OperationLogItem, 0, len(list))
	for _, item := range list {
		items = append(items, operationLogItem(item))
	}

	return &types.OperationLogListResp{
		Total:    int64(total),
		Page:     page,
		PageSize: pageSize,
		List:     items,
	}, nil
}

// parseLogTime 解析可选的起止时间，支持多种常见格式
func parseLogTime(input string) (time.Time, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, input, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseLogEndTime 解析结束时间。
// 纯日期（YYYY-MM-DD）按当天 23:59:59.999999999 处理，否则 end_time=2026-11-08
// 会被解析成当天零点，导致当天产生的日志全部查不到。
func parseLogEndTime(input string) (time.Time, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return time.Time{}, false
	}
	if t, err := time.ParseInLocation("2006-01-02", trimmed, time.Local); err == nil {
		return t.AddDate(0, 0, 1).Add(-time.Nanosecond), true
	}
	return parseLogTime(trimmed)
}
