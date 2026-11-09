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
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type RolePermissionListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRolePermissionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RolePermissionListLogic {
	return &RolePermissionListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RolePermissionListLogic) RolePermissionList(req *types.RoleDetailReq) (resp *types.RolePermissionIDsResp, err error) {
	// 1. 参数验证
	if req.ID <= 0 {
		return nil, common.NewBizError(common.ErrUserIDFormat)
	}

	// 2. 检查角色是否存在
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

	// 3. 查询角色的权限ID列表
	rolePermissions, err := l.svcCtx.EntClient.AdminRolePermission.
		Query().
		Where(adminrolepermission.RoleID(req.ID)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询角色权限失败，角色ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleListFail)
	}

	// 提取权限ID数组
	permissionIDs := make([]int64, len(rolePermissions))
	for i, rp := range rolePermissions {
		permissionIDs[i] = rp.PermissionID
	}

	l.Logger.Infof("查询角色权限成功，角色ID: %d, 角色名称: %s, 权限数量: %d",
		req.ID, role.Name, len(permissionIDs))

	// 4. 返回权限ID列表
	return &types.RolePermissionIDsResp{
		PermissionIDs: permissionIDs,
	}, nil
}
