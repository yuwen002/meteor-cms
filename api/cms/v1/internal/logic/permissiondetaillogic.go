// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	entadminpermission "github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPermissionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionDetailLogic {
	return &PermissionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionDetailLogic) PermissionDetail(req *types.PermissionDetailReq) (resp *types.PermissionDetailResp, err error) {
	// 参数验证
	if req.ID <= 0 {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}

	// 查询权限详情
	permission, err := l.svcCtx.EntClient.AdminPermission.
		Query().
		Where(
			entadminpermission.ID(req.ID),
		).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrPermissionNotFound)
		}
		l.Errorf("查询权限详情失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 构造响应数据
	var parentID int64
	if permission.ParentID != nil {
		parentID = *permission.ParentID
	}

	resp = &types.PermissionDetailResp{
		PermissionDetailItem: types.PermissionDetailItem{
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
			CreatedAt:  permission.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:  permission.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
	}

	return resp, nil
}
