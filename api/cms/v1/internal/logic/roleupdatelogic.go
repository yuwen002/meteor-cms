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
	"github.com/yuwen002/meteor-cms/internal/common"

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

	update := l.svcCtx.EntClient.AdminRole.
		UpdateOneID(req.ID).
		SetUpdatedAt(time.Now())

	// 5. 更新字段（只更新非nil的字段）
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

	// 6. 执行更新
	_, err = update.Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("更新角色失败: %v", err)
		return nil, common.NewBizError(common.ErrRoleUpdateFail)
	}

	// 角色启用状态变更会改变其下所有用户的有效权限
	l.svcCtx.PermissionChecker.InvalidateAll()

	// 7. 返回成功响应
	return &types.CommonResp{
		ID:      req.ID,
		Message: "更新角色成功",
	}, nil
}
