// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/ent/adminuserrole"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminRoleRemoveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRoleRemoveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRoleRemoveLogic {
	return &AdminRoleRemoveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminRoleRemoveLogic) AdminRoleRemove(req *types.AdminRoleRemoveReq) (resp *types.CommonResp, err error) {
	// 1. 参数验证
	if req.ID <= 0 {
		return nil, common.NewBizError(common.ErrUserIDFormat)
	}
	if req.RoleID <= 0 {
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

	// 3. 删除角色关联
	deleted, err := l.svcCtx.EntClient.AdminUserRole.
		Delete().
		Where(
			adminuserrole.UserID(req.ID),
			adminuserrole.RoleID(req.RoleID),
		).
		Exec(l.ctx)
	if err != nil {
		l.Logger.Errorf("移除管理员角色失败，管理员ID: %d, 角色ID: %d, 错误: %v", req.ID, req.RoleID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if deleted == 0 {
		l.Logger.Errorf("管理员未绑定该角色，管理员ID: %d, 角色ID: %d", req.ID, req.RoleID)
		return nil, common.NewBizErrorWithMsg(common.ErrRoleNotFound, "该管理员未绑定此角色")
	}

	l.Logger.Infof("移除管理员角色成功，管理员ID: %d, 角色ID: %d", req.ID, req.RoleID)

	// 4. 返回成功响应
	return &types.CommonResp{
		ID:      req.ID,
		Message: "移除管理员角色成功",
	}, nil
}
