package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/permcheck"
)

// grantAllowed 是授权边界的纯判定，便于脱离数据库做回归测试：
// 超管放行；非超管必须持有中间件注入的权限快照，且待授权限集合是自身权限的子集。
// hasSnapshot=false 表示请求未经过权限校验（例如测试直连或异常链路），一律拒绝。
func grantAllowed(isSuper, hasSnapshot bool, own, requested []string) bool {
	if isSuper {
		return true
	}
	if !hasSnapshot {
		return false
	}
	return permcheck.ContainsAll(own, requested)
}

// ensurePermissionGrantAllowed 校验待授予的权限：必须存在且启用，且不得超出调用者的权限集合。
// 空列表表示「清空该角色的全部权限」，属于收权操作，无需提权判定。
func ensurePermissionGrantAllowed(ctx context.Context, client *ent.Client, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	perms, err := client.AdminPermission.
		Query().
		Where(adminpermission.IDIn(ids...), adminpermission.IsActive(true)).
		All(ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if len(perms) != len(ids) {
		return common.NewBizErrorWithMsg(common.ErrPermissionNotFound, "部分权限不存在或已停用")
	}

	isSuper, err := currentUserIsSuper(ctx, client)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	own, hasSnapshot := permcheck.PermissionsFrom(ctx)
	requested := make([]string, 0, len(perms))
	for _, p := range perms {
		requested = append(requested, p.Permission)
	}
	if !grantAllowed(isSuper, hasSnapshot, own, requested) {
		return common.NewBizError(common.ErrRolePermissionDenied)
	}
	return nil
}

// ensureRoleGrantAllowed 校验待分配的角色：必须存在，且对非超管不携带超出自身权限集合的权限。
// 内置系统角色（含 SUPER_ADMIN）与停用角色只允许超级管理员下发，避免通过分配角色提权。
func ensureRoleGrantAllowed(ctx context.Context, client *ent.Client, roleIDs []int64) error {
	roles, err := client.AdminRole.
		Query().
		Where(adminrole.IDIn(roleIDs...)).
		All(ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if len(roles) != len(roleIDs) {
		return common.NewBizErrorWithMsg(common.ErrRoleNotFound, "部分角色不存在或已被删除")
	}

	isSuper, err := currentUserIsSuper(ctx, client)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if isSuper {
		return nil
	}

	own, hasSnapshot := permcheck.PermissionsFrom(ctx)
	if !hasSnapshot {
		return common.NewBizError(common.ErrPermissionDenied)
	}
	for _, role := range roles {
		if role.IsSystem || !role.IsActive {
			return common.NewBizError(common.ErrRolePermissionDenied)
		}
	}
	perms, err := client.AdminRole.
		Query().
		Where(adminrole.IDIn(roleIDs...)).
		QueryPermissions().
		Where(adminpermission.IsActive(true)).
		All(ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	requested := make([]string, 0, len(perms))
	for _, p := range perms {
		requested = append(requested, p.Permission)
	}
	if !permcheck.ContainsAll(own, requested) {
		return common.NewBizError(common.ErrRolePermissionDenied)
	}
	return nil
}

// ensureSuperAdmin 校验只有超级管理员才能执行该操作；判定走 DB 真值而非 token claim。
func ensureSuperAdmin(ctx context.Context, client *ent.Client) error {
	isSuper, err := currentUserIsSuper(ctx, client)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if !isSuper {
		return common.NewBizError(common.ErrPermissionDenied)
	}
	return nil
}
