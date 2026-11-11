// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminroledept"
	"github.com/yuwen002/meteor-cms/ent/adminrolepermission"
	"github.com/yuwen002/meteor-cms/ent/adminuserrole"
	entschema "github.com/yuwen002/meteor-cms/ent/schema"
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
		return nil, common.NewBizError(common.ErrRoleIsInUse)
	}

	// 5. 事务清理全部关联并删除角色，避免中途失败留下「权限已清、角色仍在」的半删除状态
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		l.Logger.Errorf("开启事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				l.Logger.Errorf("事务回滚失败: %v", rbErr)
			}
		}
	}()

	if _, err = tx.AdminRolePermission.
		Delete().
		Where(adminrolepermission.RoleID(req.ID)).
		Exec(l.ctx); err != nil {
		l.Logger.Errorf("删除角色权限关联失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleDeleteFail)
	}

	if _, err = tx.AdminUserRole.
		Delete().
		Where(adminuserrole.RoleID(req.ID)).
		Exec(l.ctx); err != nil {
		l.Logger.Errorf("删除用户角色关联失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleDeleteFail)
	}

	// 自定义部门关联带软删除，必须真删，否则历史行继续占用 (role_id, dept_id) 唯一索引
	if _, err = tx.AdminRoleDept.
		Delete().
		Where(adminroledept.RoleID(req.ID)).
		Exec(entschema.SkipSoftDelete(l.ctx)); err != nil {
		l.Logger.Errorf("删除角色部门关联失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleDeleteFail)
	}

	// 角色已无任何关联，物理删除可释放 name/code 唯一索引，允许同名重建
	if err = tx.AdminRole.
		DeleteOneID(req.ID).
		Exec(entschema.SkipSoftDelete(l.ctx)); err != nil {
		l.Logger.Errorf("删除角色失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleDeleteFail)
	}

	if err = tx.Commit(); err != nil {
		l.Logger.Errorf("提交事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
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
