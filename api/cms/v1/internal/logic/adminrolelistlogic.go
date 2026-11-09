// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminRoleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRoleListLogic {
	return &AdminRoleListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminRoleListLogic) AdminRoleList(req *types.AdminDetailReq) (resp *types.AdminRoleListResp, err error) {
	// 1. 参数验证
	if req.ID <= 0 {
		return nil, common.NewBizError(common.ErrUserIDFormat)
	}

	// 2. 查询管理员及其拥有的角色（角色表存在软删除，默认只返回未删除数据）
	user, err := l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.IDEQ(req.ID)).
		WithRoles(func(q *ent.AdminRoleQuery) {
			q.Order(ent.Asc(adminrole.FieldSort), ent.Desc(adminrole.FieldID))
		}).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			l.Logger.Errorf("管理员不存在，ID: %d", req.ID)
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询管理员角色失败，管理员ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrRoleListFail)
	}

	// 3. 构建角色列表
	roles := make([]types.AdminRoleItem, 0, len(user.Edges.Roles))
	for _, role := range user.Edges.Roles {
		roles = append(roles, types.AdminRoleItem{
			ID:        role.ID,
			Name:      role.Name,
			Code:      role.Code,
			Desc:      role.Desc,
			DataScope: role.DataScope,
			IsSystem:  role.IsSystem,
			IsActive:  role.IsActive,
			Sort:      role.Sort,
			CreatedAt: role.CreatedAt.Format(time.RFC3339),
		})
	}

	l.Logger.Infof("查询管理员角色成功，管理员ID: %d, 角色数量: %d", req.ID, len(roles))

	// 4. 返回角色列表
	return &types.AdminRoleListResp{
		Roles: roles,
	}, nil
}
