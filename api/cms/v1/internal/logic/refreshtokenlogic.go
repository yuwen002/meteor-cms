package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/ent/tokenblacklist"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/authsession"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// RefreshToken 使用刷新令牌签发新的访问令牌。
// 刷新令牌使用独立密钥与 access token 区分，且携带 token_type=refresh 标识，防止被当作 access token 使用。
func (l *RefreshTokenLogic) RefreshToken(req *types.RefreshTokenReq) (resp *types.RefreshTokenResp, err error) {
	claims, err := utils.ParseToken(l.svcCtx.Config.Auth.RefreshSecret, req.RefreshToken)
	if err != nil {
		return nil, common.NewBizError(common.ErrInvalidToken)
	}

	// 校验令牌类型，防止用 access token 冒充 refresh token
	if typ, _ := claims["token_type"].(string); typ != "refresh" {
		return nil, common.NewBizError(common.ErrInvalidToken)
	}

	// 检查刷新令牌是否已被吊销
	// 轮换会把旧令牌立即写入黑名单，但并发场景下同一旧令牌可能在极短时间内被重复提交，
	// 因此在 Auth.RefreshGraceSeconds 宽限期内仍视为有效（避免误登出），超过宽限期判定为重放并拒绝。
	usable, ok := l.refreshTokenUsable(utils.GenerateTokenHash(req.RefreshToken))
	if !ok {
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !usable {
		return nil, common.NewBizError(common.ErrInvalidToken)
	}

	uid, _ := claims["user_id"].(float64)
	userID := int64(uid)
	if userID <= 0 {
		return nil, common.NewBizError(common.ErrInvalidToken)
	}

	// 用户被删除或禁用时，刷新令牌立即失效
	user, err := l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.IDEQ(userID)).
		Select(
			adminuser.FieldID,
			adminuser.FieldUsername,
			adminuser.FieldNickname,
			adminuser.FieldEmail,
			adminuser.FieldPhone,
			adminuser.FieldIsSuper,
			adminuser.FieldIsActive,
		).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrInvalidToken)
		}
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !user.IsActive {
		return nil, common.NewBizError(common.ErrAccountInactive)
	}

	// 读取当前 token 版本号，写入新令牌；用于全局踢人 / 改密后旧令牌失效
	tv, _ := tokenversion.Get(l.ctx, l.svcCtx.Redis, user.ID)

	// 沿用 refresh token 中的会话 ID；升级前签发的旧令牌没有 sid，补建一条会话。
	origSID, hasSID := claims["sid"].(string)
	sid := origSID
	if !hasSID || sid == "" {
		sid = authsession.NewSID()
		hasSID = false
	}
	// 已吊销的会话不能再轮换出新令牌；不带 sid 的旧令牌仍由 tv / 黑名单控制。
	if hasSID {
		if e := authsession.CheckUsable(l.svcCtx.Redis, sid); e != nil {
			return nil, common.NewBizError(common.ErrTokenRevoked)
		}
	}

	accessToken, err := utils.GenerateToken(l.svcCtx.Config.Auth.AccessSecret, l.svcCtx.Config.Auth.AccessExpire, map[string]interface{}{
		"user_id":    user.ID,
		"username":   user.Username,
		"nickname":   user.Nickname,
		"email":      user.Email,
		"phone":      user.Phone,
		"is_super":   user.IsSuper,
		"is_active":  user.IsActive,
		"login_time": time.Now().Unix(),
		"tv":         tv,
		"sid":        sid,
	})
	if err != nil {
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 刷新令牌轮换：签发新的刷新令牌，并将本次使用的旧刷新令牌立即吊销，
	// 降低 refresh token 泄露后的重放风险。黑名单 TTL 取 RefreshExpire，可被清理任务回收。
	newRefreshToken, err := utils.GenerateToken(l.svcCtx.Config.Auth.RefreshSecret, l.svcCtx.Config.Auth.RefreshExpire, map[string]interface{}{
		"user_id":    user.ID,
		"username":   user.Username,
		"token_type": "refresh",
		"tv":         tv,
		"sid":        sid,
	})
	if err != nil {
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 会话存在则更新当前 access hash（吊销时可据此把令牌加入黑名单），否则补登。
	sessions := authsession.New(l.svcCtx.Redis, time.Duration(l.svcCtx.Config.Auth.RefreshExpire)*time.Second)
	accessHash := utils.GenerateTokenHash(accessToken)
	if hasSID {
		if e := sessions.Touch(l.ctx, user.ID, sid, accessHash); e != nil {
			logx.Errorf("更新登录会话失败: sid=%s err=%v", sid, e)
		}
	} else if e := sessions.Create(l.ctx, user.ID, sid,
		utils.GetClientIP(l.ctx), utils.GetUserAgent(l.ctx), accessHash); e != nil {
		logx.Errorf("补登登录会话失败: user_id=%d err=%v", user.ID, e)
	}

	oldHash := utils.GenerateTokenHash(req.RefreshToken)
	if _, e := l.svcCtx.EntClient.TokenBlacklist.Create().
		SetTokenHash(oldHash).
		SetToken(req.RefreshToken).
		SetExpiredAt(time.Now().Add(time.Duration(l.svcCtx.Config.Auth.RefreshExpire) * time.Second)).
		Save(l.ctx); e != nil {
		logx.Errorf("Failed to revoke old refresh token: %v", e)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	return &types.RefreshTokenResp{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil
}

// refreshTokenUsable 判断刷新令牌是否仍可使用：
//   - 不在黑名单：可用；
//   - 已在黑名单：仅当该记录是「刚写入」的（即轮换导致的并发重放）时，在宽限期内放行；
//     超出宽限期视为盗用重放，拒绝。
//
// 返回值 (usable, ok)：ok=false 表示查询异常，调用方应按服务端错误处理。
func (l *RefreshTokenLogic) refreshTokenUsable(hash string) (usable bool, ok bool) {
	entry, err := l.svcCtx.EntClient.TokenBlacklist.
		Query().
		Where(tokenblacklist.TokenHashEQ(hash)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return true, true
		}
		logx.Errorf("query token blacklist failed: %v", err)
		return false, false
	}
	grace := l.svcCtx.Config.Auth.RefreshGraceSeconds
	if grace <= 0 {
		grace = 5
	}
	return time.Since(entry.CreatedAt) <= time.Duration(grace)*time.Second, true
}
