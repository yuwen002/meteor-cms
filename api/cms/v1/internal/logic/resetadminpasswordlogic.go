// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"golang.org/x/crypto/bcrypt"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetAdminPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetAdminPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetAdminPasswordLogic {
	return &ResetAdminPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResetAdminPasswordLogic) ResetAdminPassword(req *types.ResetAdminPasswordReq) (resp *types.CommonResp, err error) {
	// 1. 获取当前登录用户信息
	currentUserID, ok := utils.CurrentUserID(l.ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}

	// 2. 参数校验
	targetID := req.ID
	if targetID <= 0 {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}
	if targetID == currentUserID {
		return nil, common.NewBizError(common.ErrCannotResetOwnPassword)
	}
	if ok, msg := utils.ValidatePasswordComplexity(req.NewPassword); !ok {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, msg)
	}

	// 3. 只有超级管理员可以重置他人密码（is_super 取 DB 真值）
	isSuper, err := currentUserIsSuper(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("查询当前用户是否超管失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !isSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "只有超级管理员可以重置其他管理员的密码")
	}

	// 4. 检查目标用户是否存在，且不能重置超级管理员
	target, err := l.svcCtx.EntClient.AdminUser.Get(l.ctx, targetID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询目标管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if target.IsSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "不能重置超级管理员的密码")
	}

	// 5. 加密新密码
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Logger.Errorf("密码加密失败: %v", err)
		return nil, common.NewBizError(common.ErrAdminPasswordHashFail)
	}

	// 6. 更新密码
	_, err = l.svcCtx.EntClient.AdminUser.
		UpdateOneID(targetID).
		SetPasswordHash(string(hashed)).
		Save(l.ctx)

	if err != nil {
		l.Logger.Errorf("重置密码失败: %v", err)
		return nil, common.NewBizError(common.ErrPasswordUpdateFailed)
	}

	// 重置密码后吊销目标用户所有其它会话（全局踢人）
	if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, targetID); berr != nil {
		l.Logger.Errorf("吊销目标用户旧会话失败: user_id=%d err=%v", targetID, berr)
	}

	return &types.CommonResp{ID: targetID, Message: "密码已重置"}, nil
}
