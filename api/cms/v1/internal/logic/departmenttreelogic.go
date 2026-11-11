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

	return buildDepartmentTree(list), nil
}

// buildDepartmentTree 先用内部指针节点完成任意层级的挂载，再一次性物化为响应类型。
// 直接把子节点按值拷进父节点，后续挂到子节点上的孙节点不会回填已拷贝的副本，
// 三级及以上部门会静默丢失且结果依赖遍历顺序。这里保持 cms.api 的值切片契约不变。
func buildDepartmentTree(list []*ent.Department) []*types.DepartmentTreeResp {
	type node struct {
		resp     types.DepartmentTreeResp
		children []*node
	}

	nodes := make(map[int64]*node, len(list))
	for _, v := range list {
		nodes[v.ID] = &node{resp: types.DepartmentTreeResp{
			Id:       v.ID,
			Name:     v.Name,
			Sort:     v.Sort,
			IsActive: v.IsActive,
			Children: []types.DepartmentTreeResp{},
		}}
	}

	roots := make([]*node, 0)
	for _, v := range list {
		cur := nodes[v.ID]
		if v.ParentID == nil || *v.ParentID == 0 {
			roots = append(roots, cur)
			continue
		}
		// parent == cur 表示 parent_id 指向自身的脏数据，挂到自己会形成自环
		if parent, ok := nodes[*v.ParentID]; ok && parent != cur {
			parent.children = append(parent.children, cur)
		}
	}

	var materialize func(*node) *types.DepartmentTreeResp
	materialize = func(n *node) *types.DepartmentTreeResp {
		resp := n.resp
		resp.Children = make([]types.DepartmentTreeResp, 0, len(n.children))
		for _, child := range n.children {
			resp.Children = append(resp.Children, *materialize(child))
		}
		return &resp
	}

	tree := make([]*types.DepartmentTreeResp, 0, len(roots))
	for _, root := range roots {
		tree = append(tree, materialize(root))
	}
	return tree
}
