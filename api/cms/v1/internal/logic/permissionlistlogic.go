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
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPermissionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionListLogic {
	return &PermissionListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionListLogic) PermissionList(req *types.PermissionListReq) (resp *types.PermissionListResp, err error) {
	// 1. 规范化分页参数（每页上限 MaxPageSize，默认 20 条）
	offset, pageSize := utils.NormalizePage(req.Page, req.PageSize, 20)
	page := offset/pageSize + 1

	// 2. 构建查询条件
	query := l.svcCtx.EntClient.AdminPermission.Query()

	// 添加搜索条件
	if req.Keyword != "" {
		query = query.Where(
			adminpermission.Or(
				adminpermission.NameContains(req.Keyword),
				adminpermission.PermissionContains(req.Keyword),
			),
		)
	}

	// 添加权限类型过滤
	if req.Type > 0 {
		query = query.Where(adminpermission.Type(req.Type))
	}

	// 添加状态过滤
	if req.IsActive != nil {
		query = query.Where(adminpermission.IsActive(*req.IsActive))
	}

	// 3. 查询总数
	total, err := query.Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询权限总数失败，错误: %v", err)
		return nil, common.NewBizError(common.ErrPermissionListFail)
	}

	// 4. 查询分页数据
	permissions, err := query.
		Order(ent.Asc(adminpermission.FieldSort)).
		Order(ent.Desc(adminpermission.FieldCreatedAt)).
		Offset(offset).
		Limit(pageSize).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询权限列表失败，错误: %v", err)
		return nil, common.NewBizError(common.ErrPermissionListFail)
	}

	// 5. 转换数据格式
	list := make([]types.PermissionDetailItem, len(permissions))
	for i, permission := range permissions {
		var parentID int64
		if permission.ParentID != nil {
			parentID = *permission.ParentID
		}

		list[i] = types.PermissionDetailItem{
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
		}
	}

	l.Logger.Infof("查询权限列表成功，页码: %d, 每页数量: %d, 总数: %d", page, pageSize, total)

	// 6. 返回结果
	return &types.PermissionListResp{
		Total:    int64(total),
		Page:     page,
		PageSize: pageSize,
		List:     list,
	}, nil
}
