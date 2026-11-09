// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPermissionTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionTreeLogic {
	return &PermissionTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionTreeLogic) PermissionTree() (resp []types.PermissionTreeItem, err error) {
	// 1. 查询所有启用的权限，按排序和创建时间排序
	permissions, err := l.svcCtx.EntClient.AdminPermission.Query().
		Where(adminpermission.IsActive(true)).
		Order(ent.Asc(adminpermission.FieldSort)).
		Order(ent.Asc(adminpermission.FieldID)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询权限树失败，错误: %v", err)
		return nil, common.NewBizError(common.ErrPermissionListFail)
	}

	// 2. 构建权限映射
	permissionMap := make(map[int64]*types.PermissionTreeItem)
	var roots []int64

	for _, permission := range permissions {
		var parentID int64
		if permission.ParentID != nil {
			parentID = *permission.ParentID
		}

		item := &types.PermissionTreeItem{
			ID:         permission.ID,
			Name:       permission.Name,
			ParentID:   parentID,
			Type:       permission.Type,
			Path:       permission.Path,
			Component:  permission.Component,
			Icon:       permission.Icon,
			Method:     permission.Method,
			ApiPath:    permission.APIPath,
			Permission: permission.Permission,
			IsActive:   permission.IsActive,
			Sort:       permission.Sort,
			Children:   make([]types.PermissionTreeItem, 0),
		}

		permissionMap[permission.ID] = item

		// 记录根节点（parent_id 为 0 或 null）
		if permission.ParentID == nil || *permission.ParentID == 0 {
			roots = append(roots, permission.ID)
		}
	}

	// 3. 构建树结构
	for _, permission := range permissions {
		var parentID int64
		if permission.ParentID != nil {
			parentID = *permission.ParentID
		}

		if parentID > 0 {
			if parent, exists := permissionMap[parentID]; exists {
				if child, exists := permissionMap[permission.ID]; exists {
					parent.Children = append(parent.Children, *child)
				}
			}
		}
	}

	// 4. 构建结果
	resp = make([]types.PermissionTreeItem, 0, len(roots))
	for _, rootID := range roots {
		if root, exists := permissionMap[rootID]; exists {
			resp = append(resp, *root)
		}
	}

	l.Logger.Infof("查询权限树成功，根节点数量: %d, 总节点数量: %d", len(resp), len(permissions))

	return resp, nil
}
