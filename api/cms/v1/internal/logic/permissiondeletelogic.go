// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPermissionDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionDeleteLogic {
	return &PermissionDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionDeleteLogic) PermissionDelete(req *types.PermissionDeleteReq) (resp *types.CommonResp, err error) {
	// 检查权限是否存在
	exists, err := l.svcCtx.EntClient.AdminPermission.
		Query().
		Where(adminpermission.ID(req.ID)).
		Exist(l.ctx)
	if err != nil {
		l.Logger.Errorf("检查权限失败，权限ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !exists {
		l.Logger.Errorf("权限不存在，权限ID: %d", req.ID)
		return nil, common.NewBizError(common.ErrPermissionNotFound)
	}

	// 检查是否有子权限
	childrenCount, err := l.svcCtx.EntClient.AdminPermission.
		Query().
		Where(adminpermission.ParentID(req.ID)).
		Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("检查子权限失败，权限ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if childrenCount > 0 {
		l.Logger.Errorf("权限下存在子权限，禁止删除，权限ID: %d, 子权限数量: %d", req.ID, childrenCount)
		return nil, common.NewBizError(common.ErrPermissionInUse)
	}

	// 执行删除
	err = l.svcCtx.EntClient.AdminPermission.
		DeleteOneID(req.ID).
		Exec(l.ctx)
	if err != nil {
		l.Logger.Errorf("删除权限失败，权限ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrPermissionDeleteFail)
	}

	l.Logger.Infof("删除权限成功，权限ID: %d", req.ID)
	invalidatePermissionCache(l.svcCtx)

	// 返回删除结果
	return &types.CommonResp{
		ID:      req.ID,
		Message: "权限删除成功",
	}, nil
}
