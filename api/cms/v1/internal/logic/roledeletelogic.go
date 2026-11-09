// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminrolepermission"
	"github.com/yuwen002/meteor-cms/ent/adminuserrole"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDeleteLogic {
	return &RoleDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleDeleteLogic) RoleDelete(req *types.RoleDeleteReq) (resp *types.CommonResp, err error) {
	// 1. 参数验证
	if req.ID <= 0 {
		return nil, common.NewBizError(common.ErrUserIDFormat)
	}

	// 2. 查询角色是否存在
	role, err := l.svcCtx.EntClient.AdminRole.
		Query().
		Where(adminrole.ID(req.ID)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			l.Logger.Errorf("角色不存在，ID: %d", req.ID)
			return nil, common.NewBizError(common.ErrRoleNotFound)
		}
		l.Logger.Errorf("查询角色失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleListFail)
	}

	// 3. 检查是否为系统内置角色
	if role.IsSystem {
		l.Logger.Errorf("尝试删除系统内置角色，ID: %d", req.ID)
		return nil, common.NewBizError(common.ErrRoleDeleteFail)
	}

	// 4. 检查是否有管理员正在使用该角色
	userCount, err := l.svcCtx.EntClient.AdminUserRole.
		Query().
		Where(adminuserrole.RoleID(req.ID)).
		Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询角色关联用户失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if userCount > 0 {
		l.Logger.Errorf("角色正在被使用，无法删除，ID: %d, 用户数量: %d", req.ID, userCount)
		return nil, common.NewBizError(common.ErrRoleDeleteFail)
	}

	// 5. 删除角色权限关联
	permissions, err := l.svcCtx.EntClient.AdminRolePermission.
		Query().
		Where(adminrolepermission.RoleID(req.ID)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询角色权限关联失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	if len(permissions) > 0 {
		_, err = l.svcCtx.EntClient.AdminRolePermission.
			Delete().
			Where(adminrolepermission.RoleID(req.ID)).
			Exec(l.ctx)
		if err != nil {
			l.Logger.Errorf("删除角色权限关联失败，ID: %d, 错误: %v", req.ID, err)
			return nil, common.NewBizError(common.ErrRoleDeleteFail)
		}
	}

	// 5.1. 删除用户角色关联
	userRoles, err := l.svcCtx.EntClient.AdminUserRole.
		Query().
		Where(adminuserrole.RoleID(req.ID)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询用户角色关联失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	if len(userRoles) > 0 {
		_, err = l.svcCtx.EntClient.AdminUserRole.
			Delete().
			Where(adminuserrole.RoleID(req.ID)).
			Exec(l.ctx)
		if err != nil {
			l.Logger.Errorf("删除用户角色关联失败，ID: %d, 错误: %v", req.ID, err)
			return nil, common.NewBizError(common.ErrRoleDeleteFail)
		}
	}

	// 6. 删除角色
	err = l.svcCtx.EntClient.AdminRole.
		DeleteOneID(req.ID).
		Exec(l.ctx)
	if err != nil {
		l.Logger.Errorf("删除角色失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleDeleteFail)
	}

	// 角色停用/删除会影响其下所有用户的权限集合
	l.svcCtx.PermissionChecker.InvalidateAll()

	l.Logger.Infof("角色删除成功，ID: %d, 名称: %s", req.ID, role.Name)

	// 7. 返回成功响应
	return &types.CommonResp{
		ID:      req.ID,
		Message: "角色删除成功",
	}, nil
}
