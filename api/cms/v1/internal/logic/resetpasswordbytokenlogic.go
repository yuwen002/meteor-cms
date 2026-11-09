// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"golang.org/x/crypto/bcrypt"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetPasswordByTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetPasswordByTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordByTokenLogic {
	return &ResetPasswordByTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ResetPasswordByToken 忘记密码流程第二步：用邮件中的重置令牌设置新密码。
//
// 令牌是一次性的，使用后立即失效，避免同一个链接被反复使用。
func (l *ResetPasswordByTokenLogic) ResetPasswordByToken(req *types.ResetPasswordByTokenReq) (resp *types.RegisterResp, err error) {
	// 1. 参数校验
	if req.Token == "" {
		return nil, common.NewBizError(common.ErrResetTokenInvalid)
	}
	if ok, msg := utils.ValidatePasswordComplexity(req.NewPassword); !ok {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, msg)
	}

	// 2. 按令牌查找管理员
	u, err := l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.ResetTokenEQ(req.Token)).
		First(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrResetTokenInvalid)
		}
		l.Logger.Errorf("按重置令牌查询管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 3. 校验有效期，过期后清理令牌
	if u.ResetExpire == nil || u.ResetExpire.Before(time.Now()) {
		if cErr := l.clearResetToken(u.ID); cErr != nil {
			l.Logger.Errorf("清理过期重置令牌失败，管理员ID: %d, 错误: %v", u.ID, cErr)
		}
		return nil, common.NewBizError(common.ErrResetTokenExpired)
	}

	// 4. 加密新密码
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Logger.Errorf("密码加密失败: %v", err)
		return nil, common.NewBizError(common.ErrAdminPasswordHashFail)
	}

	// 5. 更新密码并使令牌失效
	_, err = l.svcCtx.EntClient.AdminUser.
		UpdateOneID(u.ID).
		SetPasswordHash(string(hashed)).
		SetResetToken("").
		SetResetExpire(time.Now().Add(-time.Second)).
		Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("重置密码失败: %v", err)
		return nil, common.NewBizError(common.ErrPasswordUpdateFailed)
	}

	// 重置密码后吊销该用户所有其它会话（全局踢人）
	if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, u.ID); berr != nil {
		l.Logger.Errorf("吊销旧会话失败: user_id=%d err=%v", u.ID, berr)
	}

	return &types.RegisterResp{Message: "密码重置成功，请使用新密码登录"}, nil
}

// clearResetToken 清理已过期或已使用的重置令牌
func (l *ResetPasswordByTokenLogic) clearResetToken(userID int64) error {
	_, err := l.svcCtx.EntClient.AdminUser.
		UpdateOneID(userID).
		SetResetToken("").
		SetResetExpire(time.Now().Add(-time.Second)).
		Save(l.ctx)
	return err
}
