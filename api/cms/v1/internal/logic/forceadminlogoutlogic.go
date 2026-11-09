// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"

	"github.com/zeromicro/go-zero/core/logx"
)

type ForceAdminLogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForceAdminLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForceAdminLogoutLogic {
	return &ForceAdminLogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ForceAdminLogout 强制指定用户下线（全局踢人）。
//
// 实现方式：抬高该用户在 Redis 中的 token 版本号（auth:tv:<userID>），
// 其所有旧令牌在下次请求时会被 JWT 中间件判定为失效（返回 2012）。
// 与「禁用/删除账号、改密/重置密码」触发的踢人同源，区别仅在于本接口是管理员主动发起。
//
// 鉴权：仅超级管理员可执行（敏感操作，与重置密码同级）。
func (l *ForceAdminLogoutLogic) ForceAdminLogout(req *types.ForceAdminLogoutReq) (resp *types.CommonResp, err error) {
	// 1. 仅超级管理员可执行（is_super 取 DB 真值，避免旧 token claim 滞后）
	isSuper, err := currentUserIsSuper(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("查询当前用户是否超管失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !isSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "只有超级管理员可以强制用户下线")
	}

	// 2. 目标用户必须存在
	target, err := l.svcCtx.EntClient.AdminUser.Query().Where(adminuser.IDEQ(req.ID)).Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 3. 抬高 token 版本号，使该用户所有旧会话失效（全局踢人）
	if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, target.ID); berr != nil {
		l.Logger.Errorf("强制下线失败(吊销会话): user_id=%d err=%v", target.ID, berr)
		return nil, common.NewBizErrorWithMsg(common.ErrInternalServer, "强制下线失败，请稍后重试")
	}
	// 未配置 Redis 时 Bump 为 no-op（newVersion=0），会话无法被追踪吊销；
	// 属于已知的降级行为（与限流/踢人降级一致），记录告警避免静默成功。
	if l.svcCtx.Redis == nil {
		l.Logger.Errorf("强制下线仅在配置 Redis 后真正生效(当前未配置 Redis，user_id=%d 会话未被吊销)", target.ID)
	}

	return &types.CommonResp{
		ID:      target.ID,
		Message: "已强制该用户下线，其所有会话将在下次请求时失效",
	}, nil
}
