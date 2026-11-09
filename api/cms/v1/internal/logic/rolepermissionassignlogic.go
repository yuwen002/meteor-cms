// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminrolepermission"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type RolePermissionAssignLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRolePermissionAssignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RolePermissionAssignLogic {
	return &RolePermissionAssignLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RolePermissionAssignLogic) RolePermissionAssign(req *types.RolePermissionAssignReq) (resp *types.CommonResp, err error) {
	// 1. 参数验证
	if req.RoleID <= 0 {
		return nil, common.NewBizError(common.ErrUserIDFormat)
	}
	if len(req.PermissionIDs) == 0 {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}

	// 权限ID去重且必须为正数
	permissionIDs := make([]int64, 0, len(req.PermissionIDs))
	seen := make(map[int64]struct{}, len(req.PermissionIDs))
	for _, permissionID := range req.PermissionIDs {
		if permissionID <= 0 {
			return nil, common.NewBizError(common.ErrInvalidParams)
		}
		if _, ok := seen[permissionID]; ok {
			continue
		}
		seen[permissionID] = struct{}{}
		permissionIDs = append(permissionIDs, permissionID)
	}

	// 2. 检查角色是否存在
	role, err := l.svcCtx.EntClient.AdminRole.
		Query().
		Where(adminrole.ID(req.RoleID)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			l.Logger.Errorf("角色不存在，ID: %d", req.RoleID)
			return nil, common.NewBizError(common.ErrRoleNotFound)
		}
		l.Logger.Errorf("查询角色失败，ID: %d, 错误: %v", req.RoleID, err)
		return nil, common.NewBizError(common.ErrRoleListFail)
	}

	// 3. 检查是否为系统内置角色
	if role.IsSystem {
		l.Logger.Errorf("尝试修改系统内置角色权限，ID: %d", req.RoleID)
		return nil, common.NewBizError(common.ErrRoleIsSystemRole)
	}

	// 4. 验证权限是否存在
	permissionCount, err := l.svcCtx.EntClient.AdminPermission.
		Query().
		Where(adminpermission.IDIn(permissionIDs...)).
		Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询权限失败，错误: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if permissionCount != len(permissionIDs) {
		l.Logger.Errorf("部分权限不存在，请求权限数量: %d, 实际存在: %d", len(permissionIDs), permissionCount)
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionNotFound, "部分权限不存在或已被删除")
	}

	// 5. 开启事务
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		l.Logger.Errorf("开启事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				l.Logger.Errorf("事务回滚失败: %v", rollbackErr)
			}
		}
	}()

	// 6. 删除现有的角色权限关联
	_, err = tx.AdminRolePermission.
		Delete().
		Where(adminrolepermission.RoleID(req.RoleID)).
		Exec(l.ctx)
	if err != nil {
		l.Logger.Errorf("删除现有角色权限关联失败，角色ID: %d, 错误: %v", req.RoleID, err)
		return nil, common.NewBizError(common.ErrRoleUpdateFail)
	}

	// 7. 批量创建新的角色权限关联
	bulk := make([]*ent.AdminRolePermissionCreate, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		bulk = append(bulk, tx.AdminRolePermission.
			Create().
			SetRoleID(req.RoleID).
			SetPermissionID(permissionID))
	}

	if len(bulk) > 0 {
		_, err = tx.AdminRolePermission.CreateBulk(bulk...).Save(l.ctx)
		if err != nil {
			l.Logger.Errorf("批量创建角色权限关联失败，角色ID: %d, 错误: %v", req.RoleID, err)
			return nil, common.NewBizError(common.ErrRoleUpdateFail)
		}
	}

	// 8. 提交事务
	if err = tx.Commit(); err != nil {
		l.Logger.Errorf("提交事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 角色权限变更影响该角色下所有用户，立即让全局权限快照失效
	l.svcCtx.PermissionChecker.InvalidateAll()

	l.Logger.Infof("角色权限分配成功，角色ID: %d, 角色名称: %s, 权限数量: %d",
		req.RoleID, role.Name, len(req.PermissionIDs))

	// 9. 返回成功响应
	return &types.CommonResp{
		ID:      req.RoleID,
		Message: "角色权限分配成功",
	}, nil
}
