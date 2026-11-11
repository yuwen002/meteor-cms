// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"errors"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/tokenblacklist"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/authsession"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminLogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminLogoutLogic {
	return &AdminLogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminLogoutLogic) AdminLogout() (resp *types.CommonResp, err error) {
	// 从上下文中获取用户信息
	userClaims := utils.GetUserFromCtx(l.ctx)
	if userClaims == nil {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}

	// 从上下文中获取 token
	token := utils.GetTokenFromCtx(l.ctx)
	if token == "" {
		return nil, common.NewBizError(common.ErrInvalidToken)
	}

	// 结束当前会话：revoked 标记让该 sid 的所有令牌被中间件拒绝，
	// 与只覆盖当前这一枚令牌的 token 黑名单互补。
	if sid, ok := userClaims["sid"].(string); ok && sid != "" {
		if uid, ok := utils.ExtractUserID(userClaims); ok && uid > 0 {
			sessions := authsession.New(l.svcCtx.Redis, time.Duration(l.svcCtx.Config.Auth.RefreshExpire)*time.Second)
			if _, e := sessions.Revoke(l.ctx, uid, sid); e != nil &&
				!errors.Is(e, authsession.ErrNotFound) && !errors.Is(e, authsession.ErrUnavailable) {
				logx.Errorf("吊销登录会话失败: sid=%s err=%v", sid, e)
			}
		}
	}

	// 获取token过期时间
	exp, ok := userClaims["exp"].(float64)
	if !ok {
		exp = float64(time.Now().Add(24 * time.Hour).Unix()) // 默认24小时后过期
	}
	expTime := time.Unix(int64(exp), 0)

	// 生成 token 的哈希值
	tokenHash := utils.GenerateTokenHash(token)

	// 重复登出时 token 已在黑名单中，直接返回成功，避免撞唯一索引
	exists, err := l.svcCtx.EntClient.TokenBlacklist.
		Query().
		Where(tokenblacklist.TokenHashEQ(tokenHash)).
		Exist(l.ctx)
	if err != nil {
		logx.Errorf("Failed to check token blacklist: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return &types.CommonResp{
			ID:      0,
			Message: "Logout successful",
		}, nil
	}

	// 将token加入黑名单
	_, err = l.svcCtx.EntClient.TokenBlacklist.Create().
		SetTokenHash(tokenHash).
		SetToken(token).
		SetExpiredAt(expTime).
		Save(l.ctx)

	if err != nil {
		logx.Errorf("Failed to add token to blacklist: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 一并吊销刷新令牌：从中间件写入 context 的 X-Refresh-Token 读取并写入黑名单
	// （TTL 用 RefreshExpire，可被清理任务回收）
	if rt := utils.GetRefreshTokenFromCtx(l.ctx); rt != "" {
		rtHash := utils.GenerateTokenHash(rt)
		exists2, e2 := l.svcCtx.EntClient.TokenBlacklist.
			Query().
			Where(tokenblacklist.TokenHashEQ(rtHash)).
			Exist(l.ctx)
		if e2 == nil && !exists2 {
			rtExp := time.Now().Add(time.Duration(l.svcCtx.Config.Auth.RefreshExpire) * time.Second)
			if _, e3 := l.svcCtx.EntClient.TokenBlacklist.Create().
				SetTokenHash(rtHash).
				SetToken(rt).
				SetExpiredAt(rtExp).
				Save(l.ctx); e3 != nil {
				logx.Errorf("Failed to add refresh token to blacklist: %v", e3)
				return nil, common.NewBizError(common.ErrInternalServer)
			}
		}
	}

	return &types.CommonResp{
		ID:      0,
		Message: "Logout successful",
	}, nil
}
