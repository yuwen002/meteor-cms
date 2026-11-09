// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/department"

	"github.com/zeromicro/go-zero/core/logx"
)

type DepartmentTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDepartmentTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DepartmentTreeLogic {
	return &DepartmentTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DepartmentTreeLogic) DepartmentTree() (resp []*types.DepartmentTreeResp, err error) {
	list, err := l.svcCtx.EntClient.Department.
		Query().
		Where(department.IsActiveEQ(true)).
		Order(ent.Asc(department.FieldSort)).
		All(l.ctx)

	if err != nil {
		return nil, err
	}

	// 构建 map
	nodeMap := make(map[int64]*types.DepartmentTreeResp)

	for _, v := range list {
		nodeMap[v.ID] = &types.DepartmentTreeResp{
			Id:       v.ID,
			Name:     v.Name,
			Sort:     v.Sort,
			IsActive: v.IsActive,
			Children: []types.DepartmentTreeResp{},
		}
	}

	var tree []*types.DepartmentTreeResp

	for _, v := range list {
		node := nodeMap[v.ID]
		if v.ParentID == nil || *v.ParentID == 0 {
			tree = append(tree, node)
		} else {
			parent, ok := nodeMap[*v.ParentID]
			if ok {
				parent.Children = append(parent.Children, *node)
			}
		}
	}

	return tree, nil
}
