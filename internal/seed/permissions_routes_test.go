package seed

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 无 JWT 的认证类接口，以及权限中间件显式放行的个人自助接口，不需要 Type=3 API 权限。
// 需与 api/cms/v1/internal/middleware/permissionmiddleware.go 的 permissionBypassRoutes 同步。
var routesWithoutAPIPermission = map[string]bool{
	"POST /admin/forgot-password":               true,
	"POST /admin/login":                         true,
	"POST /admin/register":                      true,
	"POST /admin/reset-password":                true,
	"POST /admin/refresh":                       true,
	"POST /admin/logout":                        true,
	"POST /admin/logout-all":                    true,
	"GET /admin/test-token":                     true,
	"PUT /admin/admin-users/me/change-password": true,
	"GET /admin/profile/permissions":            true,
	"GET /admin/profile":                        true,
	"PUT /admin/profile":                        true,
}

// TestAPIPermissionsCoverAllProtectedRoutes 以真实注册的 Go 路由为基准双向校验权限种子：
// 受保护路由漏登 Type=3 权限，会让非超管被 PermissionMiddleware 拒绝；
// 种子里残留不存在的路由（孤立权限），则说明接口改名/删除后忘了同步清单。
func TestAPIPermissionsCoverAllProtectedRoutes(t *testing.T) {
	re := regexp.MustCompile(`Method:\s*http\.Method(\w+),\s*Path:\s*"([^"]+)"`)
	files, err := filepath.Glob("../../api/cms/v1/internal/handler/*routes.go")
	if err != nil {
		t.Fatalf("扫描路由文件失败: %v", err)
	}
	if len(files) < 5 {
		t.Fatalf("扫描到的路由文件过少（%d）: %v", len(files), files)
	}

	allRoutes := make(map[string]bool)
	protected := make(map[string]bool)
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取路由文件失败 %s: %v", file, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			key := strings.ToUpper(m[1]) + " " + m[2]
			allRoutes[key] = true
			// /api/*、/healthz、/readyz、/metrics 等公开端点不走 RBAC
			if routesWithoutAPIPermission[key] || !strings.HasPrefix(m[2], "/admin/") {
				continue
			}
			protected[key] = true
		}
	}
	if len(protected) < 50 {
		t.Fatalf("解析到的受保护路由数量异常: %d", len(protected))
	}

	seeded := make(map[string]bool)
	for _, p := range DefaultPermissions {
		if p.Type == PermissionTypeAPI {
			seeded[strings.ToUpper(p.Method)+" "+p.APIPath] = true
		}
	}
	for key := range protected {
		if !seeded[key] {
			t.Errorf("受保护路由缺少 Type=3 API 权限：%s", key)
		}
	}
	for key := range seeded {
		if !allRoutes[key] {
			t.Errorf("孤立 API 权限（找不到已注册路由）：%s", key)
		}
	}
}
