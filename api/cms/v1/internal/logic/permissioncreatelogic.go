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

type PermissionCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPermissionCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionCreateLogic {
	return &PermissionCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionCreateLogic) PermissionCreate(req *types.PermissionCreateReq) (resp *types.CommonResp, err error) {
	// 1. 参数验证
	if req.Name == "" {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}
	if req.Type < 1 || req.Type > 3 {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}

	// 2. 检查权限标识是否重复
	if req.Permission != "" {
		exists, err := l.svcCtx.EntClient.AdminPermission.
			Query().
			Where(adminpermission.Permission(req.Permission)).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("检查权限标识失败，权限标识: %s, 错误: %v", req.Permission, err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if exists {
			l.Logger.Errorf("权限标识已存在，权限标识: %s", req.Permission)
			return nil, common.NewBizError(common.ErrPermissionExists)
		}
	}

	// 3. 如果有父级ID，检查父级权限是否存在
	if req.ParentID > 0 {
		parentExists, err := l.svcCtx.EntClient.AdminPermission.
			Query().
			Where(adminpermission.ID(req.ParentID)).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("检查父级权限失败，父级ID: %d, 错误: %v", req.ParentID, err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if !parentExists {
			l.Logger.Errorf("父级权限不存在，父级ID: %d", req.ParentID)
			return nil, common.NewBizError(common.ErrParentPermissionNotFound)
		}
	}

	// 4. 创建权限
	permissionBuilder := l.svcCtx.EntClient.AdminPermission.
		Create().
		SetName(req.Name).
		SetType(req.Type).
		SetIsActive(req.IsActive).
		SetSort(req.Sort)

	// 设置可选字段
	if req.ParentID > 0 {
		permissionBuilder.SetParentID(req.ParentID)
	}
	if req.Path != "" {
		permissionBuilder.SetPath(req.Path)
	}
	if req.Component != "" {
		permissionBuilder.SetComponent(req.Component)
	}
	if req.Icon != "" {
		permissionBuilder.SetIcon(req.Icon)
	}
	if req.Method != "" {
		permissionBuilder.SetMethod(req.Method)
	}
	if req.ApiPath != "" {
		permissionBuilder.SetAPIPath(req.ApiPath)
	}
	if req.Permission != "" {
		permissionBuilder.SetPermission(req.Permission)
	}

	permission, err := permissionBuilder.Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("创建权限失败，权限名称: %s, 错误: %v", req.Name, err)
		return nil, common.NewBizError(common.ErrPermissionCreateFail)
	}

	l.Logger.Infof("创建权限成功，权限ID: %d, 权限名称: %s", permission.ID, permission.Name)
	invalidatePermissionCache(l.svcCtx)

	// 5. 返回创建结果
	return &types.CommonResp{
		ID:      permission.ID,
		Message: "权限创建成功",
	}, nil
}
