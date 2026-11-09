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
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type DisableAdminLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDisableAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableAdminLogic {
	return &DisableAdminLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DisableAdminLogic) DisableAdmin(req *types.DisableAdminReq) (resp *types.CommonResp, err error) {
	// 1. 检查管理员是否存在
	target, err := l.svcCtx.EntClient.AdminUser.Query().Where(
		adminuser.IDEQ(req.ID),
	).Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 2. 不能禁用自己，也不能禁用超级管理员
	if currentUserID, ok := utils.CurrentUserID(l.ctx); ok && currentUserID == req.ID {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "不能禁用自己")
	}
	if target.IsSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "超级管理员不可禁用")
	}

	// 3. 更新管理员状态为禁用
	_, err = l.svcCtx.EntClient.AdminUser.UpdateOneID(req.ID).
		SetIsActive(false).
		Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("禁用管理员失败: %v", err)
		return nil, common.NewBizErrorWithMsg(common.ErrInternalServer, "禁用管理员失败")
	}

	// 禁用后吊销该用户所有会话（全局踢人）
	if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, req.ID); berr != nil {
		l.Logger.Errorf("吊销用户旧会话失败: user_id=%d err=%v", req.ID, berr)
	}

	return &types.CommonResp{
		ID:      req.ID,
		Message: "禁用管理员成功",
	}, nil
}
