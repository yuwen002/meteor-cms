// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutAllLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutAllLogic {
	return &LogoutAllLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// LogoutAll 注销当前用户自己的全部会话（任意登录用户可用，权限中间件已 bypass）。
//
// 通过抬高当前用户的 token 版本号，使其所有令牌（含当前这一枚）在下次请求即被 JWT 中间件判为失效。
// 与「强制下线」系列接口同源，区别仅在于对象为「自己」且不校验超管。
func (l *LogoutAllLogic) LogoutAll() (resp *types.CommonResp, err error) {
	uid, ok := utils.CurrentUserID(l.ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}

	if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, uid); berr != nil {
		l.Logger.Errorf("注销全部会话失败: user_id=%d err=%v", uid, berr)
		return nil, common.NewBizErrorWithMsg(common.ErrInternalServer, "注销全部会话失败，请稍后重试")
	}
	// 未配置 Redis 时 Bump 为 no-op，会话无法被追踪吊销（与全局踢人降级一致），记录告警避免静默成功。
	if l.svcCtx.Redis == nil {
		l.Logger.Errorf("注销全部会话仅在配置 Redis 后真正生效(当前未配置 Redis，user_id=%d 会话未被吊销)", uid)
	}

	return &types.CommonResp{
		ID:      uid,
		Message: "已注销当前用户全部会话",
	}, nil
}
