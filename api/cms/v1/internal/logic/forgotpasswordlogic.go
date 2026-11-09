// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/mail"

	"github.com/zeromicro/go-zero/core/logx"
)

type ForgotPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForgotPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgotPasswordLogic {
	return &ForgotPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ForgotPasswordLogic) ForgotPassword(req *types.ForgotPasswordReq) (resp *types.ForgotPasswordResp, err error) {
	// 1. 查找用户。
	// 注意：账号不存在时必须返回与成功一致的响应，否则会被用于探测哪些用户名有效（账号枚举）。
	u, err := l.svcCtx.EntClient.AdminUser.Query().
		Where(adminuser.UsernameEQ(req.Username)).
		First(l.ctx)

	if err != nil {
		if !ent.IsNotFound(err) {
			l.Logger.Errorf("查询管理员失败: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		return &types.ForgotPasswordResp{
			Message: "重置密码邮件已发送，请检查邮箱",
		}, nil
	}

	// 未绑定邮箱时同样对外返回成功，避免泄露账号是否配置了邮箱
	if u.Email == "" {
		l.Logger.Infof("管理员 %s 未绑定邮箱，跳过发送重置邮件", u.Username)
		return &types.ForgotPasswordResp{
			Message: "重置密码邮件已发送，请检查邮箱",
		}, nil
	}

	// 2. 生成一个随机 Token，用于重置密码
	resetToken := uuid.NewString()
	expire := time.Now().Add(time.Hour * 1) // 1 小时有效期

	// 3. 存入数据库
	_, err = l.svcCtx.EntClient.AdminUser.
		UpdateOneID(u.ID).
		SetResetToken(resetToken).
		SetResetExpire(expire).
		Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("保存重置令牌失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 4. 发送邮件（未配置 SMTP 时降级为控制台输出）
	// 若配置了 WebBaseURL，邮件直接给出可点击的重置链接；否则仅下发裸令牌，由前端自行拼接。
	var body string
	if link := l.svcCtx.Config.WebBaseURL; link != "" {
		resetLink := strings.TrimRight(link, "/") + "/reset-password?token=" + resetToken
		body = fmt.Sprintf(
			"你好，%s：\n\n我们收到了重置你账号密码的请求。\n\n请点击下面链接（1 小时内有效）完成密码重置：\n%s\n\n如果这不是你本人的操作，请忽略本邮件，你的密码不会被修改。\n",
			u.Username, resetLink,
		)
	} else {
		body = fmt.Sprintf(
			"你好，%s：\n\n我们收到了重置你账号密码的请求。\n\n请在 1 小时内使用下面的重置令牌完成密码重置：\n\n%s\n\n如果这不是你本人的操作，请忽略本邮件，你的密码不会被修改。\n",
			u.Username, resetToken,
		)
	}
	err = l.svcCtx.MailSender.Send(mail.Message{
		To:      []string{u.Email},
		Subject: "【Meteor CMS】重置密码",
		Body:    body,
	})
	if err != nil {
		l.Logger.Errorf("发送重置密码邮件失败: %v", err)
		return nil, common.NewBizError(common.ErrMailSendFailed)
	}

	// 5. 返回成功信息
	return &types.ForgotPasswordResp{
		Message: "重置密码邮件已发送，请检查邮箱",
	}, nil
}
