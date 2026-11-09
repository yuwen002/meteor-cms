// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/ent/adminuserrole"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminRoleAssignLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRoleAssignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRoleAssignLogic {
	return &AdminRoleAssignLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminRoleAssignLogic) AdminRoleAssign(req *types.AdminRoleAssignReq) (resp *types.CommonResp, err error) {
	// 1. 参数验证（角色ID去重，避免唯一索引冲突）
	if req.ID <= 0 {
		return nil, common.NewBizError(common.ErrUserIDFormat)
	}
	roleIDs := make([]int64, 0, len(req.RoleIDs))
	seen := make(map[int64]struct{}, len(req.RoleIDs))
	for _, roleID := range req.RoleIDs {
		if roleID <= 0 {
			return nil, common.NewBizError(common.ErrInvalidParams)
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		roleIDs = append(roleIDs, roleID)
	}
	if len(roleIDs) == 0 {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}

	// 2. 检查管理员是否存在
	_, err = l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.IDEQ(req.ID)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			l.Logger.Errorf("管理员不存在，ID: %d", req.ID)
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询管理员失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 3. 校验角色是否全部存在（软删除的角色视为不存在）
	roleCount, err := l.svcCtx.EntClient.AdminRole.
		Query().
		Where(adminrole.IDIn(roleIDs...)).
		Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询角色失败，错误: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if roleCount != len(roleIDs) {
		l.Logger.Errorf("部分角色不存在，请求角色数量: %d, 实际存在: %d", len(roleIDs), roleCount)
		return nil, common.NewBizErrorWithMsg(common.ErrRoleNotFound, "部分角色不存在或已被删除")
	}

	// 4. 开启事务
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

	// 5. 删除管理员现有的角色关联（覆盖式分配）
	_, err = tx.AdminUserRole.
		Delete().
		Where(adminuserrole.UserID(req.ID)).
		Exec(l.ctx)
	if err != nil {
		l.Logger.Errorf("删除现有管理员角色关联失败，管理员ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 6. 批量创建新的角色关联
	bulk := make([]*ent.AdminUserRoleCreate, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		bulk = append(bulk, tx.AdminUserRole.
			Create().
			SetUserID(req.ID).
			SetRoleID(roleID))
	}
	if _, err = tx.AdminUserRole.CreateBulk(bulk...).Save(l.ctx); err != nil {
		l.Logger.Errorf("批量创建管理员角色关联失败，管理员ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 7. 提交事务
	if err = tx.Commit(); err != nil {
		l.Logger.Errorf("提交事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 用户角色变更后只让该用户的权限快照失效
	l.svcCtx.PermissionChecker.InvalidateUser(req.ID)

	l.Logger.Infof("管理员角色分配成功，管理员ID: %d, 角色ID: %v", req.ID, roleIDs)

	// 8. 返回成功响应
	return &types.CommonResp{
		ID:      req.ID,
		Message: "分配管理员角色成功",
	}, nil
}
