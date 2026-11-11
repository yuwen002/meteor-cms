// Package datascope 处理 RBAC 的数据权限（行级可见范围）。
//
// 取值范围定义见 AdminRole.data_scope：
//
//	1 全部数据 / 2 本部门 / 3 本部门及子部门 / 4 仅自己 / 5 自定义部门（读取 admin_role_depts）
//
// 安全原则：无法确定可见范围时按最严格处理（不返回数据），而不是退回全量。
package datascope

import (
	"context"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminroledept"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
)

// 数据范围取值，与 AdminRole.data_scope 保持一致
const (
	// ScopeAll 全部数据，不做行级过滤
	ScopeAll = 1
	// ScopeDept 仅本部门
	ScopeDept = 2
	// ScopeDeptAndChildren 本部门及全部子部门
	ScopeDeptAndChildren = 3
	// ScopeSelf 仅自己的数据
	ScopeSelf = 4
	// ScopeCustom 自定义部门集合，读取 admin_role_depts
	ScopeCustom = 5
)

// ValidScope 判断 data_scope 是否为已定义的取值，供写入口做前置校验。
func ValidScope(scope int) bool {
	return scope >= ScopeAll && scope <= ScopeCustom
}

// ActiveDepartments 只保留仍启用的部门；软删除行由 Ent 拦截器在查询阶段排除。
// 停用部门不能再通过旧的 admin_users.dept_id / admin_role_depts 引用继续放权。
func ActiveDepartments(departments []*ent.Department) []*ent.Department {
	active := make([]*ent.Department, 0, len(departments))
	for _, d := range departments {
		if d != nil && d.IsActive {
			active = append(active, d)
		}
	}
	return active
}

// Filter 当前用户的数据可见范围
type Filter struct {
	// Restricted 是否需要进行行级过滤，false 表示可见全部数据
	Restricted bool
	// DeptIDs 允许访问的部门集合
	DeptIDs []int64
	// OnlySelf 是否允许看到自己的数据
	OnlySelf bool
	// UserID 当前用户 ID
	UserID int64
}

// Empty 是否无任何可见范围，true 时调用方应直接返回空结果集（fail-closed）
func (f *Filter) Empty() bool {
	return f.Restricted && len(f.DeptIDs) == 0 && !f.OnlySelf
}

// AllowsDept 判断指定部门是否落在允许访问的部门集合内。
// 供「按部门筛选」类入口把用户传入的 dept_id 与可见范围求交，防止探测越权数据。
func (f *Filter) AllowsDept(id int64) bool {
	for _, d := range f.DeptIDs {
		if d == id {
			return true
		}
	}
	return false
}

// Resolve 解析指定用户的数据范围。
//
// 超级管理员与拥有 ScopeAll 角色的用户不做行级过滤；
// 拥有多个角色时取各角色可见范围的并集；
// 未分配任何启用角色的用户只能看到自己的数据。
func Resolve(ctx context.Context, client *ent.Client, userID int64) (*Filter, error) {
	user, err := client.AdminUser.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 超级管理员不受数据范围限制
	if user.IsSuper {
		return &Filter{Restricted: false, UserID: userID}, nil
	}

	roles, err := client.AdminUser.
		Query().
		Where(adminuser.IDEQ(userID)).
		QueryRoles().
		Where(adminrole.IsActive(true)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// 没有任何启用角色时按最严格处理：仅自己
	if len(roles) == 0 {
		return &Filter{Restricted: true, OnlySelf: true, UserID: userID}, nil
	}

	filter := &Filter{Restricted: true, UserID: userID}
	deptSet := make(map[int64]struct{})
	needChildren := false

	// 只把「仍存在且启用」的部门计入可见范围：部门软删除或停用后，
	// admin_users.dept_id 与 admin_role_depts 里的旧引用不应继续放权。
	// Ent 的软删除拦截器会在查询时自动排除已删除行。
	var activeDepartments []*ent.Department
	activeDeptIDs := make(map[int64]struct{})
	loaded := false
	loadActiveDepartments := func() error {
		if loaded {
			return nil
		}
		all, err := client.Department.Query().All(ctx)
		if err != nil {
			return err
		}
		activeDepartments = ActiveDepartments(all)
		for _, d := range activeDepartments {
			activeDeptIDs[d.ID] = struct{}{}
		}
		loaded = true
		return nil
	}

	for _, role := range roles {
		switch role.DataScope {
		case ScopeAll:
			// 任一角色为「全部数据」即不做限制
			return &Filter{Restricted: false, UserID: userID}, nil

		case ScopeDept, ScopeDeptAndChildren:
			if user.DeptID == nil {
				// 未分配部门却要求按部门过滤，该角色贡献的可见范围为空
				continue
			}
			if err := loadActiveDepartments(); err != nil {
				return nil, err
			}
			if _, ok := activeDeptIDs[*user.DeptID]; !ok {
				continue
			}
			deptSet[*user.DeptID] = struct{}{}
			if role.DataScope == ScopeDeptAndChildren {
				needChildren = true
			}

		case ScopeSelf:
			filter.OnlySelf = true

		case ScopeCustom:
			if err := loadActiveDepartments(); err != nil {
				return nil, err
			}
			roleDepts, err := client.AdminRoleDept.
				Query().
				Where(adminroledept.RoleID(role.ID)).
				All(ctx)
			if err != nil {
				return nil, err
			}
			for _, rd := range roleDepts {
				if _, ok := activeDeptIDs[rd.DeptID]; ok {
					deptSet[rd.DeptID] = struct{}{}
				}
			}

		default:
			// 未知取值不做猜测：只跳过这一条脏角色，其余角色仍按并集生效。
			// 若所有角色都无效，filter 为空集，由上层 fail-closed 处理。
			continue
		}
	}

	// 展开「本部门及子部门」：只沿启用部门展开，停用/已删部门不提供可见范围
	if needChildren && len(deptSet) > 0 {
		roots := make([]int64, 0, len(deptSet))
		for id := range deptSet {
			roots = append(roots, id)
		}
		for _, id := range CollectDescendantIDs(activeDepartments, roots) {
			deptSet[id] = struct{}{}
		}
	}

	for id := range deptSet {
		filter.DeptIDs = append(filter.DeptIDs, id)
	}

	return filter, nil
}

// BuildFallbackFilter 解析失败时的兜底范围：仅自己，遵循 fail-closed 原则
func BuildFallbackFilter(userID int64) *Filter {
	return &Filter{Restricted: true, OnlySelf: true, UserID: userID}
}

// AllowsUser 判断给定用户（id + 所属部门）是否在当前数据权限范围内。
// 用于单条数据（如管理员详情）的可见性判定，遵循 fail-closed：
// 受限且无匹配时返回 false。
func (f *Filter) AllowsUser(userID int64, deptID *int64) bool {
	if !f.Restricted {
		return true
	}
	if f.OnlySelf {
		return userID == f.UserID
	}
	if len(f.DeptIDs) == 0 {
		return false
	}
	if deptID == nil {
		return false
	}
	for _, d := range f.DeptIDs {
		if d == *deptID {
			return true
		}
	}
	return false
}
