// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type MyProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMyProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MyProfileLogic {
	return &MyProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MyProfileLogic) MyProfile() (resp *types.AdminProfileResp, err error) {
	userID, ok := utils.CurrentUserID(l.ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}

	user, err := l.svcCtx.EntClient.AdminUser.Get(l.ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrMissingUserContext)
		}
		l.Logger.Errorf("查询当前用户信息失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	return toProfileResp(user), nil
}

// toProfileResp 将管理员实体转换为个人资料响应
func toProfileResp(user *ent.AdminUser) *types.AdminProfileResp {
	var deptID int64
	if user.DeptID != nil {
		deptID = *user.DeptID
	}

	resp := &types.AdminProfileResp{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Phone:     user.Phone,
		Avatar:    user.Avatar,
		IsSuper:   user.IsSuper,
		DeptID:    deptID,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
	if user.LastLoginAt != nil {
		resp.LastLoginAt = user.LastLoginAt.Format(time.RFC3339)
	}

	return resp
}
