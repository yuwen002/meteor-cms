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

type PermissionUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPermissionUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionUpdateLogic {
	return &PermissionUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionUpdateLogic) PermissionUpdate(req *types.PermissionUpdateReq) (resp *types.CommonResp, err error) {
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

	// 参数验证
	if req.Name != nil && *req.Name == "" {
		return nil, common.NewBizError(common.ErrPermissionNameCannotBeEmpty)
	}
	if req.Type != nil && (*req.Type < 1 || *req.Type > 3) {
		return nil, common.NewBizError(common.ErrPermissionTypeInvalid)
	}

	// 检查权限标识是否重复（排除当前权限）
	if req.Permission != nil && *req.Permission != "" {
		exists, err := l.svcCtx.EntClient.AdminPermission.
			Query().
			Where(
				adminpermission.Permission(*req.Permission),
				adminpermission.IDNEQ(req.ID),
			).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("检查权限标识失败，权限标识: %s, 错误: %v", *req.Permission, err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if exists {
			l.Logger.Errorf("权限标识已存在，权限标识: %s", *req.Permission)
			return nil, common.NewBizError(common.ErrPermissionExists)
		}
	}

	// 如果有父级ID，检查父级权限是否存在且不能是自己
	if req.ParentID != nil && *req.ParentID > 0 {
		if *req.ParentID == req.ID {
			l.Logger.Errorf("不能将自己设为父级权限，权限ID: %d", req.ID)
			return nil, common.NewBizError(common.ErrInvalidParams)
		}
		parentExists, err := l.svcCtx.EntClient.AdminPermission.
			Query().
			Where(adminpermission.ID(*req.ParentID)).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("检查父级权限失败，父级ID: %d, 错误: %v", *req.ParentID, err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if !parentExists {
			l.Logger.Errorf("父级权限不存在，父级ID: %d", *req.ParentID)
			return nil, common.NewBizError(common.ErrParentPermissionNotFound)
		}
	}

	// 构建更新操作
	updateBuilder := l.svcCtx.EntClient.AdminPermission.
		UpdateOneID(req.ID)

	// 设置需要更新的字段
	if req.Name != nil {
		updateBuilder.SetName(*req.Name)
	}
	if req.Type != nil {
		updateBuilder.SetType(*req.Type)
	}
	if req.IsActive != nil {
		updateBuilder.SetIsActive(*req.IsActive)
	}
	if req.Sort != nil {
		updateBuilder.SetSort(*req.Sort)
	}
	if req.ParentID != nil {
		updateBuilder.SetParentID(*req.ParentID)
	}
	if req.Path != nil {
		updateBuilder.SetPath(*req.Path)
	}
	if req.Component != nil {
		updateBuilder.SetComponent(*req.Component)
	}
	if req.Icon != nil {
		updateBuilder.SetIcon(*req.Icon)
	}
	if req.Method != nil {
		updateBuilder.SetMethod(*req.Method)
	}
	if req.ApiPath != nil {
		updateBuilder.SetAPIPath(*req.ApiPath)
	}
	if req.Permission != nil {
		updateBuilder.SetPermission(*req.Permission)
	}

	// 执行更新
	permission, err := updateBuilder.Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("更新权限失败，权限ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrPermissionUpdateFail)
	}

	l.Logger.Infof("更新权限成功，权限ID: %d, 权限名称: %s", permission.ID, permission.Name)
	invalidatePermissionCache(l.svcCtx)

	// 返回更新结果
	return &types.CommonResp{
		ID:      permission.ID,
		Message: "权限更新成功",
	}, nil
}
