// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestTokenLogic {
	return &TestTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestTokenLogic) TestToken() (resp *types.TestTokenResp, err error) {
	claims := utils.GetUserFromCtx(l.ctx)

	return &types.TestTokenResp{
		Message: "Token OK",
		Claims:  claims,
	}, nil
}
