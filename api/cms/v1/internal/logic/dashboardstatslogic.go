package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/loginlog"
	"github.com/yuwen002/meteor-cms/ent/operationlog"

	"github.com/zeromicro/go-zero/core/logx"
)

type DashboardStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDashboardStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DashboardStatsLogic {
	return &DashboardStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DashboardStats 聚合后台常用统计指标。
// 各指标相互独立：单项查询失败仅记录日志，不影响其他指标返回。
func (l *DashboardStatsLogic) DashboardStats() (resp *types.DashboardStatsResp, err error) {
	resp = &types.DashboardStatsResp{}
	client := l.svcCtx.EntClient

	// 资源计数
	if n, e := client.AdminUser.Query().Count(l.ctx); e == nil {
		resp.AdminCount = int64(n)
	} else {
		logx.Errorf("统计管理员数失败: %v", e)
	}
	if n, e := client.Department.Query().Count(l.ctx); e == nil {
		resp.DeptCount = int64(n)
	} else {
		logx.Errorf("统计部门数失败: %v", e)
	}
	if n, e := client.AdminRole.Query().Count(l.ctx); e == nil {
		resp.RoleCount = int64(n)
	} else {
		logx.Errorf("统计角色数失败: %v", e)
	}
	if n, e := client.AdminPermission.Query().Count(l.ctx); e == nil {
		resp.PermissionCount = int64(n)
	} else {
		logx.Errorf("统计权限数失败: %v", e)
	}

	// 今日登录成功 / 失败次数
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if n, e := client.LoginLog.Query().
		Where(loginlog.StatusEQ(0), loginlog.LoginAtGTE(today)).
		Count(l.ctx); e == nil {
		resp.TodayLoginCount = int64(n)
	} else {
		logx.Errorf("统计今日登录失败: %v", e)
	}
	if n, e := client.LoginLog.Query().
		Where(loginlog.StatusEQ(1), loginlog.LoginAtGTE(today)).
		Count(l.ctx); e == nil {
		resp.TodayFailLogin = int64(n)
	} else {
		logx.Errorf("统计今日失败登录失败: %v", e)
	}

	// 近 7 天操作日志数
	weekAgo := now.AddDate(0, 0, -7)
	if n, e := client.OperationLog.Query().
		Where(operationlog.CreatedAtGTE(weekAgo)).
		Count(l.ctx); e == nil {
		resp.WeekOpCount = int64(n)
	} else {
		logx.Errorf("统计近 7 天操作数失败: %v", e)
	}

	return resp, nil
}
