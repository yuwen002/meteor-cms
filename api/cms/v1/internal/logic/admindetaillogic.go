// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/datascope"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDetailLogic {
	return &AdminDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDetailLogic) AdminDetail(req *types.AdminDetailReq) (resp *types.AdminDetailResp, err error) {
	admin, err := l.svcCtx.EntClient.AdminUser.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("获取管理员详情失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 数据权限：非超管仅能查看数据范围内的管理员详情（fail-closed，越权统一按不存在处理）
	uid, ok := utils.CurrentUserID(l.ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}
	isSuper, err := currentUserIsSuper(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("查询当前用户是否超管失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !isSuper {
		scope, serr := datascope.Resolve(l.ctx, l.svcCtx.EntClient, uid)
		if serr != nil {
			scope = datascope.BuildFallbackFilter(uid)
		}
		if !scope.AllowsUser(admin.ID, admin.DeptID) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
	}

	return &types.AdminDetailResp{
		ID:       admin.ID,
		Username: admin.Username,
		Nickname: admin.Nickname,
		Email:    admin.Email,
		Phone:    admin.Phone,
		IsActive: admin.IsActive,
	}, nil
}
