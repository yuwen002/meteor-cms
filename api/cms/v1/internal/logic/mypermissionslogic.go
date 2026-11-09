// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type MyPermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMyPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MyPermissionsLogic {
	return &MyPermissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// MyPermissions 返回当前登录用户可见的菜单树与可使用的功能权限标识
//
// 超级管理员拥有全部菜单与全部功能权限；普通管理员由其角色关联的启用权限决定。
// 该接口本身不受权限中间件约束（见 permissionBypassRoutes），否则普通管理员
// 在尚未获得任何权限时无法取回自己的菜单。
func (l *MyPermissionsLogic) MyPermissions() (resp *types.MyPermissionResp, err error) {
	// 1. 当前登录用户
	userID, ok := utils.CurrentUserID(l.ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}

	user, err := l.svcCtx.EntClient.AdminUser.Get(l.ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询当前用户失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 2. 全部启用菜单
	menus, err := l.svcCtx.EntClient.AdminPermission.
		Query().
		Where(
			adminpermission.TypeEQ(common.PermissionTypeMenu),
			adminpermission.IsActive(true),
		).
		Order(ent.Asc(adminpermission.FieldSort), ent.Asc(adminpermission.FieldID)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询菜单失败: %v", err)
		return nil, common.NewBizError(common.ErrPermissionListFail)
	}

	// 3. 全部启用功能权限标识
	buttons, err := l.svcCtx.EntClient.AdminPermission.
		Query().
		Where(
			adminpermission.TypeEQ(common.PermissionTypeButton),
			adminpermission.IsActive(true),
		).
		Order(ent.Asc(adminpermission.FieldSort), ent.Asc(adminpermission.FieldID)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询功能权限失败: %v", err)
		return nil, common.NewBizError(common.ErrPermissionListFail)
	}

	allMenuCodes := permissionCodeSet(menus)
	allButtonCodes := collectPermissionCodes(buttons)

	// 4. 超级管理员拥有全部权限
	if user.IsSuper {
		return &types.MyPermissionResp{
			IsSuper:     true,
			Menus:       buildMenuTree(menus, allMenuCodes),
			Permissions: allButtonCodes,
		}, nil
	}

	// 5. 普通管理员：取其所有启用角色下的启用权限
	owned, err := l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.IDEQ(userID)).
		QueryRoles().
		Where(adminrole.IsActive(true)).
		QueryPermissions().
		Where(adminpermission.IsActive(true)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询用户权限失败: %v", err)
		return nil, common.NewBizError(common.ErrPermissionListFail)
	}
	ownedCodes := make(map[string]struct{}, len(owned))
	for _, p := range owned {
		ownedCodes[p.Permission] = struct{}{}
	}

	visibleMenus := make([]*ent.AdminPermission, 0, len(menus))
	for _, m := range menus {
		if _, ok := ownedCodes[m.Permission]; ok {
			visibleMenus = append(visibleMenus, m)
		}
	}
	visibleCodes := make([]string, 0, len(allButtonCodes))
	for _, code := range allButtonCodes {
		if _, ok := ownedCodes[code]; ok {
			visibleCodes = append(visibleCodes, code)
		}
	}

	return &types.MyPermissionResp{
		IsSuper:     false,
		Menus:       buildMenuTree(visibleMenus, ownedCodes),
		Permissions: visibleCodes,
	}, nil
}

// permissionCodeSet 提取权限标识集合
func permissionCodeSet(perms []*ent.AdminPermission) map[string]struct{} {
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		set[p.Permission] = struct{}{}
	}
	return set
}

// collectPermissionCodes 提取权限标识并去重
func collectPermissionCodes(perms []*ent.AdminPermission) []string {
	codes := make([]string, 0, len(perms))
	seen := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		if _, ok := seen[p.Permission]; ok {
			continue
		}
		seen[p.Permission] = struct{}{}
		codes = append(codes, p.Permission)
	}
	return codes
}

// buildMenuTree 按 parent_id 组装菜单树，父菜单缺失时该节点作为根节点返回
func buildMenuTree(menus []*ent.AdminPermission, allowed map[string]struct{}) []types.MyMenuItem {
	nodes := make(map[int64]*types.MyMenuItem, len(menus))
	for _, m := range menus {
		if _, ok := allowed[m.Permission]; !ok {
			continue
		}
		var parentID int64
		if m.ParentID != nil {
			parentID = *m.ParentID
		}
		nodes[m.ID] = &types.MyMenuItem{
			ID:         m.ID,
			Name:       m.Name,
			ParentID:   parentID,
			Path:       m.Path,
			Component:  m.Component,
			Icon:       m.Icon,
			Permission: m.Permission,
			Sort:       m.Sort,
			Children:   make([]types.MyMenuItem, 0),
		}
	}

	// 先统一把子节点挂到父节点上，再收集根节点。
	// 注意：menus 按 sort 升序返回（父菜单 sort 更小，排在子菜单之前），
	// 如果边收集根节点边挂载子节点，父节点会先被拷贝进 roots，
	// 之后追加到 map 中指针节点上的 Children 不会反映到已拷贝的副本里，
	// 结果是父菜单 children 为空、子菜单整体丢失。因此这里分两趟处理。
	attached := make(map[int64]bool, len(nodes))
	for _, m := range menus {
		node, ok := nodes[m.ID]
		if !ok {
			continue
		}
		var parentID int64
		if m.ParentID != nil {
			parentID = *m.ParentID
		}
		if parentID > 0 {
			if parent, exists := nodes[parentID]; exists {
				parent.Children = append(parent.Children, *node)
				attached[m.ID] = true
			}
		}
	}

	roots := make([]types.MyMenuItem, 0, len(nodes))
	for _, m := range menus {
		node, ok := nodes[m.ID]
		if !ok || attached[m.ID] {
			continue
		}
		roots = append(roots, *node)
	}

	return roots
}
