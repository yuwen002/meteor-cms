// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"

	"github.com/zeromicro/go-zero/core/logx"
)

type ForceAdminLogoutByDeptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForceAdminLogoutByDeptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForceAdminLogoutByDeptLogic {
	return &ForceAdminLogoutByDeptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ForceAdminLogoutByDept 强制下线指定部门下的全部管理员（超管专用，与单用户强制下线同源）。
func (l *ForceAdminLogoutByDeptLogic) ForceAdminLogoutByDept(req *types.ForceLogoutByDeptReq) (resp *types.CommonResp, err error) {
	// 1. 仅超级管理员可执行
	isSuper, err := currentUserIsSuper(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("查询当前用户是否超管失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !isSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "只有超级管理员可以强制用户下线")
	}

	// 2. 查询该部门下全部管理员
	users, err := l.svcCtx.EntClient.AdminUser.Query().Where(adminuser.DeptIDEQ(req.DeptID)).All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询部门管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 3. 批量抬高 token 版本号，使这些用户所有旧会话失效（全局踢人）
	var bumped int64
	for _, u := range users {
		if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, u.ID); berr != nil {
			l.Logger.Errorf("强制下线失败(部门): user_id=%d err=%v", u.ID, berr)
			return nil, common.NewBizErrorWithMsg(common.ErrInternalServer, "强制下线失败，请稍后重试")
		}
		bumped++
	}
	if l.svcCtx.Redis == nil {
		l.Logger.Errorf("强制下线仅在配置 Redis 后真正生效(当前未配置 Redis，部门=%d 会话未被吊销)", req.DeptID)
	}

	return &types.CommonResp{
		ID:      bumped,
		Message: "已强制下线该部门全部管理员",
	}, nil
}
