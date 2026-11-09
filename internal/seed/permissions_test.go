package seed

import (
	"strconv"
	"strings"
	"testing"
)

// 权限中间件按 method + api_path 查 Type=3 记录，重复配置会判定为歧义并直接拒绝请求。
func TestAPIPermissionsAreUnique(t *testing.T) {
	seen := make(map[string]string)
	for _, p := range DefaultPermissions {
		if p.Type != PermissionTypeAPI {
			continue
		}
		if p.Method == "" || p.APIPath == "" {
			t.Errorf("权限 %s 缺少 Method 或 APIPath", p.Permission)
			continue
		}
		key := strings.ToUpper(p.Method) + " " + p.APIPath
		if prev, ok := seen[key]; ok {
			t.Errorf("API 权限路径重复：%s（%s 与 %s）", key, prev, p.Permission)
		}
		seen[key] = p.Permission
	}
}

// 文章模块的每个受保护接口都必须在种子里有一条 Type=3 记录，
// 否则非超级管理员会被 PermissionMiddleware 以「未登记权限」拒绝。
func TestContentAPIPermissionsComplete(t *testing.T) {
	want := []string{
		"cms:article:list", "cms:article:detail", "cms:article:add", "cms:article:edit",
		"cms:article:delete", "cms:article:publish", "cms:article:withdraw", "cms:article:toggle_top",
		"cms:category:list", "cms:category:add", "cms:category:edit", "cms:category:delete",
		"cms:tag:list", "cms:tag:add", "cms:tag:edit", "cms:tag:delete",
		"cms:article:submit", "cms:article:approve", "cms:article:reject",
		"cms:article:revision", "cms:article:revision_restore", "cms:article:review",
		"cms:trash:list", "cms:trash:restore", "cms:trash:purge",
	}
	apis := make(map[string]bool)
	buttons := make(map[string]bool)
	menus := make(map[string]bool)
	for _, p := range DefaultPermissions {
		switch p.Type {
		case PermissionTypeAPI:
			apis[p.Permission] = true
		case PermissionTypeButton:
			buttons[p.Permission] = true
		case PermissionTypeMenu:
			menus[p.Permission] = true
		}
	}
	for _, code := range want {
		if !apis[code] {
			t.Errorf("缺少 Type=3 API 权限：%s", code)
		}
		if !buttons[code] {
			t.Errorf("缺少同标识的 Type=2 按钮权限：%s", code)
		}
	}
	for _, code := range []string{"cms:group:content", "cms:article:list", "cms:category:list", "cms:tag:list"} {
		if !menus[code] {
			t.Errorf("缺少菜单：%s", code)
		}
	}
}

// 菜单的 ParentName 必须能在菜单列表里找到，且父级排在子级之前，
// 否则 UpsertPermissions 解析不到 parentID，子菜单会挂在根节点。
func TestMenuParentsDeclaredBeforeChildren(t *testing.T) {
	declared := make(map[string]bool)
	for _, p := range DefaultPermissions {
		if p.Type != PermissionTypeMenu {
			continue
		}
		if p.ParentName != "" && !declared[p.ParentName] {
			t.Errorf("菜单 %s 的父级 %s 尚未声明", p.Name, p.ParentName)
		}
		declared[p.Name] = true
	}
}

// contentroutes.go 中受保护的后台路由必须逐条登记 Type=3 权限，
// 权限中间件按 method + api_path 精确匹配，缺一条就会让非超管收到「权限不足」。
func TestContentRoutesCoveredByAPIPermissions(t *testing.T) {
	routes := []struct{ method, path string }{
		{"GET", "/admin/articles"}, {"POST", "/admin/articles"},
		{"GET", "/admin/articles/:id"}, {"PUT", "/admin/articles/:id"}, {"DELETE", "/admin/articles/:id"},
		{"POST", "/admin/articles/:id/publish"}, {"POST", "/admin/articles/:id/withdraw"},
		{"POST", "/admin/articles/:id/toggle-top"},
		{"POST", "/admin/articles/batch-offline"}, {"POST", "/admin/articles/batch-delete"},
		{"GET", "/admin/categories"}, {"POST", "/admin/categories"},
		{"PUT", "/admin/categories/:id"}, {"DELETE", "/admin/categories/:id"},
		{"GET", "/admin/tags"}, {"POST", "/admin/tags"},
		{"PUT", "/admin/tags/:id"}, {"DELETE", "/admin/tags/:id"},
		{"POST", "/admin/articles/:id/submit"}, {"POST", "/admin/articles/:id/approve"},
		{"POST", "/admin/articles/:id/reject"}, {"GET", "/admin/articles/:id/reviews"},
		{"GET", "/admin/articles/:id/revisions"}, {"GET", "/admin/articles/:id/revisions/:rid"},
		{"POST", "/admin/articles/:id/revisions/:rid/restore"},
		{"GET", "/admin/trash"}, {"POST", "/admin/trash/:type/:id/restore"},
		{"DELETE", "/admin/trash/:type/:id/purge"},
	}
	registered := make(map[string]bool)
	for _, p := range DefaultPermissions {
		if p.Type == PermissionTypeAPI {
			registered[strings.ToUpper(p.Method)+" "+p.APIPath] = true
		}
	}
	for _, r := range routes {
		key := strings.ToUpper(r.method) + " " + r.path
		if !registered[key] {
			t.Errorf("受保护路由缺少 Type=3 API 权限：%s", key)
		}
	}
}

// 同一权限码会绑定多条接口（单条操作 + 批量操作、修订列表 + 修订详情）。
// UpsertPermissions 必须按 (type, permission, method, api_path) 去重，
// 否则后写入的 spec 会覆盖前面的记录，运行时只剩最后一条接口可用。
func TestSameCodeAPIPermissionsUseDistinctRoutes(t *testing.T) {
	byCode := make(map[string]map[string]bool)
	for _, p := range DefaultPermissions {
		if p.Type != PermissionTypeAPI {
			continue
		}
		if byCode[p.Permission] == nil {
			byCode[p.Permission] = make(map[string]bool)
		}
		byCode[p.Permission][strings.ToUpper(p.Method)+" "+p.APIPath] = true
	}
	for _, code := range []string{"cms:article:delete", "cms:article:withdraw", "cms:article:revision"} {
		if len(byCode[code]) < 2 {
			t.Errorf("权限 %s 应绑定多条接口，实际 %d 条", code, len(byCode[code]))
		}
	}
}

// admin_permissions 上有 (type, permission, method, api_path) 唯一索引。
// 种子清单本身若出现重复键，迁移时会在建索引或写入时失败，这里提前拦下。
func TestPermissionSpecsSatisfyUniqueIndex(t *testing.T) {
	seen := make(map[string]string)
	for _, p := range DefaultPermissions {
		key := strings.ToUpper(strings.TrimSpace(p.Method)) + " " + p.APIPath
		full := strings.Join([]string{
			strconv.Itoa(p.Type), p.Permission, key,
		}, "|")
		if prev, ok := seen[full]; ok {
			t.Errorf("权限清单违反唯一索引：%s（%s 与 %s）", full, prev, p.Name)
		}
		seen[full] = p.Name
	}
}
