package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCaptchaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCaptchaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCaptchaLogic {
	return &GetCaptchaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCaptchaLogic) GetCaptcha() (resp *types.CaptchaResp, err error) {
	// 关闭验证码时不下发图片，前端据 enabled=false 隐藏输入项。
	// 保留这个接口是为了让前端在不知道后端配置的情况下也能自适应。
	if !l.svcCtx.Config.Login.CaptchaEnabled {
		return &types.CaptchaResp{Enabled: false}, nil
	}

	captchaID, captchaBase64, err := utils.GenerateCaptcha()
	if err != nil {
		l.Logger.Errorf("生成验证码失败: %v", err)
		return nil, common.NewBizError(common.ErrCaptchaGenerate)
	}

	return &types.CaptchaResp{
		Enabled:       true,
		CaptchaID:     captchaID,
		CaptchaBase64: captchaBase64,
	}, nil
}
