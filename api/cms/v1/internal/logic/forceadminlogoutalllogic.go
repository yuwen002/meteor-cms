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
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type ForceAdminLogoutAllLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForceAdminLogoutAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForceAdminLogoutAllLogic {
	return &ForceAdminLogoutAllLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ForceAdminLogoutAll 强制下线系统内全部其他管理员（超管专用），当前操作者自己不受影响，
// 避免把自己也踢下线导致无法继续管理。
func (l *ForceAdminLogoutAllLogic) ForceAdminLogoutAll() (resp *types.CommonResp, err error) {
	// 1. 仅超级管理员可执行
	isSuper, err := currentUserIsSuper(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("查询当前用户是否超管失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !isSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "只有超级管理员可以强制用户下线")
	}

	// 2. 当前操作者 ID（用于排除自己）
	uid, ok := utils.CurrentUserID(l.ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}

	// 3. 查询除自己外的全部管理员
	users, err := l.svcCtx.EntClient.AdminUser.Query().Where(adminuser.IDNEQ(uid)).All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 4. 批量抬高 token 版本号，使这些用户所有旧会话失效（全局踢人）
	var bumped int64
	for _, u := range users {
		if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, u.ID); berr != nil {
			l.Logger.Errorf("强制下线失败(全部): user_id=%d err=%v", u.ID, berr)
			return nil, common.NewBizErrorWithMsg(common.ErrInternalServer, "强制下线失败，请稍后重试")
		}
		bumped++
	}
	if l.svcCtx.Redis == nil {
		l.Logger.Errorf("强制下线仅在配置 Redis 后真正生效(当前未配置 Redis，会话未被吊销)")
	}

	return &types.CommonResp{
		ID:      bumped,
		Message: "已强制下线系统内全部其他管理员",
	}, nil
}
