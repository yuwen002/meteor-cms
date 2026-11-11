package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/audit"
	"github.com/yuwen002/meteor-cms/internal/pkg/authsession"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (*types.LoginResp, error) {
	// 验证码校验。LOGIN_CAPTCHA_ENABLED=false 时跳过，供本地开发与自动化 E2E 使用；
	// 生产环境必须保持开启（配置默认 true）。
	if l.svcCtx.Config.Login.CaptchaEnabled {
		if req.Captcha == "" || req.CaptchaID == "" {
			l.recordLoginLog(0, req.Username, common.GetErrorMessage(common.ErrMissingCaptcha))
			return nil, common.NewBizError(common.ErrMissingCaptcha)
		}
		if !utils.VerifyCaptcha(req.CaptchaID, req.Captcha) {
			l.recordLoginLog(0, req.Username, common.GetErrorMessage(common.ErrInvalidCaptcha))
			return nil, common.NewBizError(common.ErrInvalidCaptcha)
		}
	}

	// 查询数据库用户
	user, err := l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.UsernameEQ(req.Username)).
		First(l.ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			// 用户名不存在与密码错误返回同一错误，避免账号枚举
			l.recordLoginLog(0, req.Username, "账号不存在")
			return nil, common.NewBizError(common.ErrLoginFailed)
		}
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 账号是否处于锁定状态（连续登录失败过多）
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remaining := int(time.Until(*user.LockedUntil).Minutes())
		if remaining < 1 {
			remaining = 1
		}
		l.recordLoginLog(user.ID, user.Username, "账号已锁定")
		return nil, common.NewBizErrorWithMsg(
			common.ErrAccountLocked,
			fmt.Sprintf("账号已被临时锁定，请 %d 分钟后再试", remaining),
		)
	}

	// 验证密码
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		l.recordLoginLog(user.ID, user.Username, "密码错误")
		l.increaseLoginFail(user)
		return nil, common.NewBizError(common.ErrLoginFailed)
	}

	// 密码正确：清零失败计数与锁定时间
	if user.LoginFailCount > 0 || user.LockedUntil != nil {
		if _, e := l.svcCtx.EntClient.AdminUser.UpdateOneID(user.ID).
			SetLoginFailCount(0).
			ClearLockedUntil().
			Save(l.ctx); e != nil {
			logx.Errorf("登录成功后重置失败计数失败: %v", e)
		}
	}

	// 检查用户账号是否激活
	if !user.IsActive {
		l.recordLoginLog(user.ID, user.Username, "账号未激活")
		return nil, common.NewBizError(common.ErrAccountInactive)
	}

	// 登录成功
	l.recordLoginLogSuccess(user.ID, user.Username)

	// 读取当前 token 版本号，写入新令牌；用于全局踢人 / 改密后旧令牌失效
	tv, _ := tokenversion.Get(l.ctx, l.svcCtx.Redis, user.ID)

	// 会话 ID 贯穿 access / refresh，刷新时沿用，用于会话列表与单会话吊销。
	sid := authsession.NewSID()

	// 签发访问令牌（短期有效）
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

	// 签发刷新令牌（长期有效，token_type 固定为 refresh，防止被当作 access 使用）
	refreshToken, err := utils.GenerateToken(l.svcCtx.Config.Auth.RefreshSecret, l.svcCtx.Config.Auth.RefreshExpire, map[string]interface{}{
		"user_id":    user.ID,
		"username":   user.Username,
		"token_type": "refresh",
		"tv":         tv,
		"sid":        sid,
	})
	if err != nil {
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 登记活跃会话；Redis 不可用时降级为 no-op，不阻断登录主流程。
	sessions := authsession.New(l.svcCtx.Redis, time.Duration(l.svcCtx.Config.Auth.RefreshExpire)*time.Second)
	if err := sessions.Create(l.ctx, user.ID, sid,
		utils.GetClientIP(l.ctx), utils.GetUserAgent(l.ctx),
		utils.GenerateTokenHash(accessToken)); err != nil {
		logx.Errorf("登记登录会话失败: user_id=%d err=%v", user.ID, err)
	}

	go func() {
		// 创建一个新的上下文，设置超时时间
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 更新最后登录时间为当前时间
		now := time.Now()
		_, err = l.svcCtx.EntClient.AdminUser.UpdateOneID(user.ID).
			SetLastLoginAt(now).
			Save(ctx)
		if err != nil {
			// 记录更详细的错误信息
			logx.WithContext(ctx).Errorf("更新用户ID: %d， 最后登录时间失败: %v", user.ID, err)
		} else {
			logx.WithContext(ctx).Infof("用户ID: %d， 最后登录时间已更新: %v", user.ID, now)
		}
	}()

	return &types.LoginResp{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// recordLoginLogSuccess 记录一次成功的登录
func (l *LoginLogic) recordLoginLogSuccess(userID int64, username string) {
	l.saveLoginLog(userID, username, 0, "")
}

// recordLoginLog 记录一次失败的登录，reason 仅用于内部审计，不返回给调用方
func (l *LoginLogic) recordLoginLog(userID int64, username, reason string) {
	l.saveLoginLog(userID, username, 1, reason)
}

// saveLoginLog 异步写入登录日志，避免影响登录接口耗时
func (l *LoginLogic) saveLoginLog(userID int64, username string, status int, reason string) {
	ip := utils.GetClientIP(l.ctx)
	ua := utils.Truncate(utils.GetUserAgent(l.ctx), 512)
	requestID := utils.RequestIDFromCtx(l.ctx)

	audit.Submit("login_log", func(ctx context.Context) error {
		create := l.svcCtx.EntClient.LoginLog.Create().
			SetUserID(userID).
			SetUsername(username).
			SetIP(ip).
			SetUserAgent(ua).
			SetStatus(status).
			SetLoginAt(time.Now())
		if reason != "" {
			create = create.SetFailReason(reason)
		}
		if requestID != "" {
			create = create.SetRequestID(requestID)
		}
		_, err := create.Save(ctx)
		return err
	})
}

// increaseLoginFail 登录失败后累加失败次数；达到阈值（Login.MaxFailCount）时锁定账号一段时间。
// 锁定判定在 Login 入口处完成，这里只负责写计数与锁定时间。
func (l *LoginLogic) increaseLoginFail(user *ent.AdminUser) {
	maxFail := l.svcCtx.Config.Login.MaxFailCount
	if maxFail <= 0 {
		return // 未启用锁定
	}
	lockMin := l.svcCtx.Config.Login.LockMinutes
	if lockMin <= 0 {
		lockMin = 15
	}

	newCount := user.LoginFailCount + 1
	update := l.svcCtx.EntClient.AdminUser.UpdateOneID(user.ID).SetLoginFailCount(newCount)
	if newCount >= maxFail {
		update = update.SetLockedUntil(time.Now().Add(time.Duration(lockMin) * time.Minute))
	}
	if _, err := update.Save(l.ctx); err != nil {
		logx.Errorf("更新登录失败计数失败: %v", err)
	}
}
