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

	"github.com/zeromicro/go-zero/core/logx"
)

type EnableAdminLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEnableAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableAdminLogic {
	return &EnableAdminLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EnableAdminLogic) EnableAdmin(req *types.EnableAdminReq) (resp *types.CommonResp, err error) {
	// 1. 检查管理员是否存在
	_, err = l.svcCtx.EntClient.AdminUser.Query().Where(
		adminuser.IDEQ(req.ID),
	).Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 2. 更新管理员状态为启用
	_, err = l.svcCtx.EntClient.AdminUser.UpdateOneID(req.ID).
		SetIsActive(true).
		Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("启用管理员失败: %v", err)
		return nil, common.NewBizErrorWithMsg(common.ErrInternalServer, "启用管理员失败")
	}

	return &types.CommonResp{
		ID:      req.ID,
		Message: "启用管理员成功",
	}, nil
}
