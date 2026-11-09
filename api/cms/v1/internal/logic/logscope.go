package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/permcheck"
	"github.com/yuwen002/meteor-cms/internal/utils"
)

// LogViewAllPermission 允许查看全量审计日志的权限码。
// 有了它才能设置「审计员」岗位，而不必把普通管理员直接提成超管。
const LogViewAllPermission = "system:log:view_all"

// canViewAllLogs 判断当前用户能否查看全量日志。
// 超管始终可以；其余用户须持有 view_all 权限（复用权限中间件注入的同一份快照）。
func canViewAllLogs(ctx context.Context, client *ent.Client) (bool, error) {
	isSuper, err := currentUserIsSuper(ctx, client)
	if err != nil {
		return false, err
	}
	if isSuper {
		return true, nil
	}
	if perms, ok := permcheck.PermissionsFrom(ctx); ok {
		return permcheck.Has(perms, LogViewAllPermission), nil
	}
	return false, nil
}

// logScopeUserID 返回非全量可见用户需要叠加的 author 过滤 ID；ok=false 表示不限制。
func logScopeUserID(ctx context.Context, client *ent.Client) (int64, bool, error) {
	all, err := canViewAllLogs(ctx, client)
	if err != nil {
		return 0, false, err
	}
	if all {
		return 0, false, nil
	}
	uid, ok := utils.CurrentUserID(ctx)
	if !ok || uid <= 0 {
		return 0, false, common.NewBizError(common.ErrUnauthorized)
	}
	return uid, true, nil
}
