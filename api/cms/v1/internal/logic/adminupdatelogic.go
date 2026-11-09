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
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateLogic {
	return &AdminUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateLogic) AdminUpdate(req *types.UpdateAdminReq) (resp *types.CommonResp, err error) {
	id := req.ID
	if id <= 0 {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}

	// 1. 目标管理员必须存在
	target, err := l.svcCtx.EntClient.AdminUser.Get(l.ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 2. 超级管理员资料只能由超级管理员修改（is_super 取 DB 真值，避免旧 token claim 滞后）
	isSuper, err := currentUserIsSuper(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("查询当前用户是否超管失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if target.IsSuper && !isSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "不能修改超级管理员的资料")
	}

	// 3. 不能把自己禁用，避免把自己锁在系统外
	currentUserID, _ := utils.CurrentUserID(l.ctx)
	if req.IsActive != nil && !*req.IsActive && currentUserID == id {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "不能禁用自己")
	}

	// 4. 邮箱查重（排除自身）
	if req.Email != nil && *req.Email != "" {
		emailExists, err := l.svcCtx.EntClient.AdminUser.
			Query().
			Where(adminuser.EmailEQ(*req.Email), adminuser.IDNEQ(id)).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("检查管理员邮箱失败: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if emailExists {
			return nil, common.NewBizError(common.ErrAdminEmailAlreadyExists)
		}
	}

	// 5. 按传入字段做局部更新，未传的字段保持不变
	update := l.svcCtx.EntClient.AdminUser.UpdateOneID(id)
	changed := false
	if req.Nickname != nil {
		update = update.SetNickname(*req.Nickname)
		changed = true
	}
	if req.Email != nil {
		update = update.SetEmail(*req.Email)
		changed = true
	}
	if req.Phone != nil {
		update = update.SetPhone(*req.Phone)
		changed = true
	}
	if req.IsActive != nil {
		update = update.SetIsActive(*req.IsActive)
		changed = true
	}

	if !changed {
		return &types.CommonResp{
			ID:      id,
			Message: "没有需要更新的字段",
		}, nil
	}

	if _, err = update.Save(l.ctx); err != nil {
		l.Logger.Errorf("更新管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	return &types.CommonResp{
		ID:      id,
		Message: "更新成功",
	}, nil
}
