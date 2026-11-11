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
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminroledept"
	entschema "github.com/yuwen002/meteor-cms/ent/schema"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/datascope"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleUpdateLogic {
	return &RoleUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleUpdateLogic) RoleUpdate(req *types.RoleUpdateReq) (resp *types.CommonResp, err error) {
	// 1. 检查角色是否存在
	role, err := l.svcCtx.EntClient.AdminRole.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrRoleNotFound)
		}
		l.Logger.Errorf("查询角色失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 2. 检查是否是系统角色（系统角色不允许修改）
	if role.IsSystem {
		return nil, common.NewBizError(common.ErrRoleIsSystemRole)
	}

	// 3. 检查角色编码是否重复（如果提供了Code）
	if req.Code != nil && *req.Code != role.Code {
		// 验证code不能为空字符串
		if strings.TrimSpace(*req.Code) == "" {
			return nil, common.NewBizError(common.ErrRoleCodeCannotBeEmpty)
		}

		exists, err := l.svcCtx.EntClient.AdminRole.
			Query().
			Where(
				adminrole.Code(*req.Code),
				adminrole.IDNEQ(req.ID),
			).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("检查角色编码失败: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if exists {
			return nil, common.NewBizError(common.ErrRoleCodeExists)
		}
	}

	// 4. 验证字段（如果提供了值）
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, common.NewBizError(common.ErrRoleNameCannotBeEmpty)
		}
	}
	if req.DataScope != nil && !datascope.ValidScope(*req.DataScope) {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "data_scope 必须为 1-5")
	}

	// 离开「自定义部门」时同步清理历史部门清单，避免日后改回 5 时继承旧范围。
	clearDepts := req.DataScope != nil &&
		*req.DataScope != datascope.ScopeCustom && role.DataScope == datascope.ScopeCustom

	// 5. 更新与关联清理放在同一事务，避免只改了 data_scope 却留下旧部门清单
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

	update := tx.AdminRole.
		UpdateOneID(req.ID).
		SetUpdatedAt(time.Now())

	// 6. 更新字段（只更新非nil的字段）
	if req.Name != nil {
		update.SetName(*req.Name)
	}
	if req.Code != nil {
		update.SetCode(*req.Code)
	}
	if req.Desc != nil {
		update.SetDesc(*req.Desc)
	}
	if req.DataScope != nil {
		update.SetDataScope(*req.DataScope)
	}
	if req.IsActive != nil {
		update.SetIsActive(*req.IsActive)
	}
	if req.Sort != nil {
		update.SetSort(*req.Sort)
	}

	if _, err = update.Save(l.ctx); err != nil {
		l.Logger.Errorf("更新角色失败: %v", err)
		return nil, common.NewBizError(common.ErrRoleUpdateFail)
	}

	if clearDepts {
		if _, err = tx.AdminRoleDept.
			Delete().
			Where(adminroledept.RoleID(req.ID)).
			Exec(entschema.SkipSoftDelete(l.ctx)); err != nil {
			l.Logger.Errorf("清理角色部门范围失败: %v", err)
			return nil, common.NewBizError(common.ErrRoleUpdateFail)
		}
	}

	if err = tx.Commit(); err != nil {
		l.Logger.Errorf("提交事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 角色启用状态变更会改变其下所有用户的有效权限
	l.svcCtx.PermissionChecker.InvalidateAll()

	// 7. 返回成功响应
	return &types.CommonResp{
		ID:      req.ID,
		Message: "更新角色成功",
	}, nil
}
