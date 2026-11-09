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
	"github.com/yuwen002/meteor-cms/ent/department"
	entschema "github.com/yuwen002/meteor-cms/ent/schema"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/datascope"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleDeptAssignLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleDeptAssignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDeptAssignLogic {
	return &RoleDeptAssignLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleDeptAssignLogic) RoleDeptAssign(req *types.RoleDeptReq) (resp *types.CommonResp, err error) {
	// 1. 角色必须存在
	role, err := l.svcCtx.EntClient.AdminRole.
		Query().
		Where(adminrole.ID(req.ID)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrRoleNotFound)
		}
		l.Logger.Errorf("查询角色失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 2. 系统内置角色不可调整数据范围
	if role.IsSystem {
		return nil, common.NewBizError(common.ErrRoleIsSystemRole)
	}

	// 3. 只有 data_scope=5（自定义部门）的角色才维护部门清单
	if role.DataScope != datascope.ScopeCustom {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams,
			"仅 data_scope=5（自定义部门）的角色支持设置部门范围")
	}

	// 4. 部门ID去重且必须为正数
	deptIDs := make([]int64, 0, len(req.DeptIDs))
	seen := make(map[int64]struct{}, len(req.DeptIDs))
	for _, id := range req.DeptIDs {
		if id <= 0 {
			return nil, common.NewBizError(common.ErrInvalidParams)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		deptIDs = append(deptIDs, id)
	}

	// 5. 部门必须存在且处于启用状态
	if len(deptIDs) > 0 {
		count, err := l.svcCtx.EntClient.Department.
			Query().
			Where(
				department.IDIn(deptIDs...),
				department.IsActive(true),
			).
			Count(l.ctx)
		if err != nil {
			l.Logger.Errorf("查询部门失败，错误: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if count != len(deptIDs) {
			return nil, common.NewBizErrorWithMsg(common.ErrDepartmentNotFound, "部分部门不存在或未启用")
		}
	}

	// 6. 事务：整表重建该角色的部门关联。
	// 关联表带软删除，这里用 SkipSoftDelete 真删，避免历史软删除行继续占用 (role_id, dept_id) 唯一索引。
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

	if _, err = tx.AdminRoleDept.
		Delete().
		Where(adminroledept.RoleID(req.ID)).
		Exec(entschema.SkipSoftDelete(l.ctx)); err != nil {
		l.Logger.Errorf("清理角色部门关联失败，角色ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	for _, deptID := range deptIDs {
		if _, err = tx.AdminRoleDept.
			Create().
			SetRoleID(req.ID).
			SetDeptID(deptID).
			Save(l.ctx); err != nil {
			l.Logger.Errorf("创建角色部门关联失败，角色ID: %d, 部门ID: %d, 错误: %v", req.ID, deptID, err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
	}

	if err = tx.Commit(); err != nil {
		l.Logger.Errorf("提交事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	return &types.CommonResp{
		ID:      req.ID,
		Message: "角色部门范围设置成功",
	}, nil
}
