package seed

import (
	"context"
	"log"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/internal/common"
)

// 权限类型，定义收敛在 common 包，避免各层各自定义
const (
	// PermissionTypeMenu 菜单
	PermissionTypeMenu = common.PermissionTypeMenu
	// PermissionTypeButton 按钮/功能点
	PermissionTypeButton = common.PermissionTypeButton
	// PermissionTypeAPI 后端 API，权限中间件按此类型匹配路由
	PermissionTypeAPI = common.PermissionTypeAPI
)

// PermissionSpec 权限定义（数据驱动，新增后端接口时只需在此追加一条 Type=API）
type PermissionSpec struct {
	// Name 权限名称
	Name string
	// Permission 权限标识，同一操作在按钮与 API 两种类型上使用相同标识，
	// 便于把「按钮权限」与「接口权限」一次性授权给角色
	Permission string
	// Type 权限类型：1 菜单 2 按钮 3 API
	Type int
	// ParentName 父级权限名称，Type=1 时为空
	ParentName string
	// Path 前端路由路径（Type=1）
	Path string
	// Component 前端组件路径（Type=1）
	Component string
	// Icon 菜单图标（Type=1）
	Icon string
	// Method HTTP 方法（Type=2/3）
	Method string
	// APIPath 后端接口路径（Type=3），需与 cms.api 中的路由保持一致，支持 :id 形式的路径参数
	APIPath string
	// Sort 排序，从小到大
	Sort int
}

// DefaultPermissions 系统内置权限清单。
//
// 约定：
//   - Type=1 菜单使用「列表」权限标识（如 system:role:list），菜单可见即拥有该标识；
//   - Type=2 按钮与 Type=3 API 使用完全相同的权限标识，保证授权一次即可同时生效；
//   - Type=3 的 APIPath 必须与 cms.api 的真实路由一致，否则非超级管理员会被权限中间件拒绝。
var DefaultPermissions = []PermissionSpec{
	// ---------- 菜单（Type=1） ----------
	{Name: "控制台", Permission: "system:dashboard:view", Type: PermissionTypeMenu,
		Path: "/dashboard", Component: "views/dashboard/index.vue", Icon: "dashboard", Sort: 1},
	{Name: "管理员管理", Permission: "system:admin_user:list", Type: PermissionTypeMenu,
		Path: "/system/users", Component: "views/admin/user/index.vue", Icon: "user", Sort: 10},
	{Name: "部门管理", Permission: "system:dept:list", Type: PermissionTypeMenu,
		Path: "/system/depts", Component: "views/admin/dept/index.vue", Icon: "dept", Sort: 20},
	{Name: "角色管理", Permission: "system:role:list", Type: PermissionTypeMenu,
		Path: "/system/roles", Component: "views/admin/role/index.vue", Icon: "role", Sort: 30},
	{Name: "权限管理", Permission: "system:permission:list", Type: PermissionTypeMenu,
		Path: "/system/permissions", Component: "views/admin/permission/index.vue", Icon: "permission", Sort: 40},
	{Name: "操作日志", Permission: "system:log:operation_list", Type: PermissionTypeMenu,
		Path: "/system/operation-log", Component: "views/system/log/operation.vue", Icon: "document", Sort: 50},
	{Name: "登录日志", Permission: "system:log:login_list", Type: PermissionTypeMenu,
		Path: "/system/login-log", Component: "views/system/log/login.vue", Icon: "clock", Sort: 55},

	// ---------- 控制台（菜单与统计接口同用一个标识） ----------
	{Name: "查看控制台", Permission: "system:dashboard:view", Type: PermissionTypeButton, ParentName: "控制台", Sort: 2},
	{Name: "查看控制台", Permission: "system:dashboard:view", Type: PermissionTypeAPI, ParentName: "控制台", Method: "GET", APIPath: "/admin/dashboard/stats", Sort: 2},

	// ---------- 管理员管理（Type=2 按钮 / Type=3 API） ----------
	{Name: "查看管理员", Permission: "system:admin_user:list", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 11},
	{Name: "查看管理员", Permission: "system:admin_user:list", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "GET", APIPath: "/admin/admin-users", Sort: 11},
	{Name: "管理员详情", Permission: "system:admin_user:detail", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 12},
	{Name: "管理员详情", Permission: "system:admin_user:detail", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "GET", APIPath: "/admin/admin-users/:id", Sort: 12},
	{Name: "新增管理员", Permission: "system:admin_user:add", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 13},
	{Name: "新增管理员", Permission: "system:admin_user:add", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "POST", APIPath: "/admin/admin-users/create", Sort: 13},
	{Name: "编辑管理员", Permission: "system:admin_user:edit", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 14},
	{Name: "编辑管理员", Permission: "system:admin_user:edit", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "PUT", APIPath: "/admin/admin-users/:id", Sort: 14},
	{Name: "删除管理员", Permission: "system:admin_user:delete", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 15},
	{Name: "删除管理员", Permission: "system:admin_user:delete", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "DELETE", APIPath: "/admin/admin-users/:id", Sort: 15},
	{Name: "禁用管理员", Permission: "system:admin_user:disable", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 16},
	{Name: "禁用管理员", Permission: "system:admin_user:disable", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "PUT", APIPath: "/admin/admin-users/:id/disable", Sort: 16},
	{Name: "启用管理员", Permission: "system:admin_user:enable", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 17},
	{Name: "启用管理员", Permission: "system:admin_user:enable", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "PUT", APIPath: "/admin/admin-users/:id/enable", Sort: 17},
	{Name: "重置管理员密码", Permission: "system:admin_user:reset_pwd", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 18},
	{Name: "重置管理员密码", Permission: "system:admin_user:reset_pwd", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "PUT", APIPath: "/admin/admin-users/reset-password", Sort: 18},
	{Name: "强制下线", Permission: "system:admin_user:force_logout", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 20},
	{Name: "强制下线", Permission: "system:admin_user:force_logout", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "PUT", APIPath: "/admin/admin-users/:id/force-logout", Sort: 20},
	{Name: "强制下线(部门)", Permission: "system:admin_user:force_logout_dept", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 26},
	{Name: "强制下线(部门)", Permission: "system:admin_user:force_logout_dept", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "POST", APIPath: "/admin/admin-users/force-logout-by-dept", Sort: 26},
	{Name: "强制下线(全部)", Permission: "system:admin_user:force_logout_all", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 27},
	{Name: "强制下线(全部)", Permission: "system:admin_user:force_logout_all", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "POST", APIPath: "/admin/admin-users/force-logout-all", Sort: 27},
	{Name: "查看登录会话", Permission: "system:session:list", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 28},
	{Name: "查看登录会话", Permission: "system:session:list", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "GET", APIPath: "/admin/sessions", Sort: 28},
	{Name: "吊销登录会话", Permission: "system:session:revoke", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 29},
	{Name: "吊销登录会话", Permission: "system:session:revoke", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "DELETE", APIPath: "/admin/sessions/:sid", Sort: 29},
	{Name: "绑定部门", Permission: "system:admin_user:bind_dept", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 19},
	{Name: "绑定部门", Permission: "system:admin_user:bind_dept", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "PUT", APIPath: "/admin/admin-users/:id/bind-department", Sort: 19},
	{Name: "查看管理员角色", Permission: "system:admin_user:role_list", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 21},
	{Name: "查看管理员角色", Permission: "system:admin_user:role_list", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "GET", APIPath: "/admin/admin-users/:id/roles", Sort: 21},
	{Name: "分配管理员角色", Permission: "system:admin_user:role_assign", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 22},
	{Name: "分配管理员角色", Permission: "system:admin_user:role_assign", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "PUT", APIPath: "/admin/admin-users/:id/roles", Sort: 22},
	{Name: "移除管理员角色", Permission: "system:admin_user:role_remove", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 23},
	{Name: "移除管理员角色", Permission: "system:admin_user:role_remove", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "DELETE", APIPath: "/admin/admin-users/:id/roles/:role_id", Sort: 23},

	// ---------- 部门管理 ----------
	{Name: "查看部门", Permission: "system:dept:list", Type: PermissionTypeButton, ParentName: "部门管理", Sort: 24},
	{Name: "查看部门", Permission: "system:dept:list", Type: PermissionTypeAPI, ParentName: "部门管理", Method: "GET", APIPath: "/admin/departments/tree", Sort: 24},
	{Name: "部门详情", Permission: "system:dept:detail", Type: PermissionTypeButton, ParentName: "部门管理", Sort: 25},
	{Name: "部门详情", Permission: "system:dept:detail", Type: PermissionTypeAPI, ParentName: "部门管理", Method: "GET", APIPath: "/admin/departments/:id", Sort: 25},
	{Name: "新增部门", Permission: "system:dept:add", Type: PermissionTypeButton, ParentName: "部门管理", Sort: 26},
	{Name: "新增部门", Permission: "system:dept:add", Type: PermissionTypeAPI, ParentName: "部门管理", Method: "POST", APIPath: "/admin/departments", Sort: 26},
	{Name: "编辑部门", Permission: "system:dept:edit", Type: PermissionTypeButton, ParentName: "部门管理", Sort: 27},
	{Name: "编辑部门", Permission: "system:dept:edit", Type: PermissionTypeAPI, ParentName: "部门管理", Method: "PUT", APIPath: "/admin/departments/:id", Sort: 27},
	{Name: "删除部门", Permission: "system:dept:delete", Type: PermissionTypeButton, ParentName: "部门管理", Sort: 28},
	{Name: "删除部门", Permission: "system:dept:delete", Type: PermissionTypeAPI, ParentName: "部门管理", Method: "DELETE", APIPath: "/admin/departments/:id", Sort: 28},

	// ---------- 角色管理 ----------
	{Name: "查看角色", Permission: "system:role:list", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 31},
	{Name: "查看角色", Permission: "system:role:list", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "GET", APIPath: "/admin/roles", Sort: 31},
	{Name: "角色详情", Permission: "system:role:detail", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 32},
	{Name: "角色详情", Permission: "system:role:detail", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "GET", APIPath: "/admin/roles/:id", Sort: 32},
	{Name: "新增角色", Permission: "system:role:add", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 33},
	{Name: "新增角色", Permission: "system:role:add", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "POST", APIPath: "/admin/roles", Sort: 33},
	{Name: "编辑角色", Permission: "system:role:edit", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 34},
	{Name: "编辑角色", Permission: "system:role:edit", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "PUT", APIPath: "/admin/roles/:id", Sort: 34},
	{Name: "删除角色", Permission: "system:role:delete", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 35},
	{Name: "删除角色", Permission: "system:role:delete", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "DELETE", APIPath: "/admin/roles/:id", Sort: 35},
	{Name: "查看角色权限", Permission: "system:role:perm_list", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 36},
	{Name: "查看角色权限", Permission: "system:role:perm_list", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "GET", APIPath: "/admin/roles/:id/permissions", Sort: 36},
	{Name: "查看角色权限(含名称)", Permission: "system:role:perm_list_names", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 36},
	{Name: "查看角色权限(含名称)", Permission: "system:role:perm_list_names", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "GET", APIPath: "/admin/roles/:id/permissions-with-names", Sort: 36},
	{Name: "分配角色权限", Permission: "system:role:perm_assign", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 37},
	{Name: "分配角色权限", Permission: "system:role:perm_assign", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "PUT", APIPath: "/admin/roles/:id/permissions", Sort: 37},
	{Name: "查看角色部门", Permission: "system:role:dept_list", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 38},
	{Name: "查看角色部门", Permission: "system:role:dept_list", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "GET", APIPath: "/admin/roles/:id/departments", Sort: 38},
	{Name: "设置角色部门", Permission: "system:role:dept_assign", Type: PermissionTypeButton, ParentName: "角色管理", Sort: 39},
	{Name: "设置角色部门", Permission: "system:role:dept_assign", Type: PermissionTypeAPI, ParentName: "角色管理", Method: "PUT", APIPath: "/admin/roles/:id/departments", Sort: 39},

	// ---------- 权限管理 ----------
	{Name: "查看权限", Permission: "system:permission:list", Type: PermissionTypeButton, ParentName: "权限管理", Sort: 41},
	{Name: "查看权限", Permission: "system:permission:list", Type: PermissionTypeAPI, ParentName: "权限管理", Method: "GET", APIPath: "/admin/permissions", Sort: 41},
	{Name: "权限树", Permission: "system:permission:tree", Type: PermissionTypeButton, ParentName: "权限管理", Sort: 42},
	{Name: "权限树", Permission: "system:permission:tree", Type: PermissionTypeAPI, ParentName: "权限管理", Method: "GET", APIPath: "/admin/permissions/tree", Sort: 42},
	{Name: "权限详情", Permission: "system:permission:detail", Type: PermissionTypeButton, ParentName: "权限管理", Sort: 43},
	{Name: "权限详情", Permission: "system:permission:detail", Type: PermissionTypeAPI, ParentName: "权限管理", Method: "GET", APIPath: "/admin/permissions/:id", Sort: 43},
	{Name: "新增权限", Permission: "system:permission:add", Type: PermissionTypeButton, ParentName: "权限管理", Sort: 44},
	{Name: "新增权限", Permission: "system:permission:add", Type: PermissionTypeAPI, ParentName: "权限管理", Method: "POST", APIPath: "/admin/permissions", Sort: 44},
	{Name: "编辑权限", Permission: "system:permission:edit", Type: PermissionTypeButton, ParentName: "权限管理", Sort: 45},
	{Name: "编辑权限", Permission: "system:permission:edit", Type: PermissionTypeAPI, ParentName: "权限管理", Method: "PUT", APIPath: "/admin/permissions/:id", Sort: 45},
	{Name: "删除权限", Permission: "system:permission:delete", Type: PermissionTypeButton, ParentName: "权限管理", Sort: 46},
	{Name: "删除权限", Permission: "system:permission:delete", Type: PermissionTypeAPI, ParentName: "权限管理", Method: "DELETE", APIPath: "/admin/permissions/:id", Sort: 46},

	// ---------- 日志管理 ----------
	{Name: "查看操作日志", Permission: "system:log:operation_list", Type: PermissionTypeButton, ParentName: "操作日志", Sort: 51},
	{Name: "查看操作日志", Permission: "system:log:operation_list", Type: PermissionTypeAPI, ParentName: "操作日志", Method: "GET", APIPath: "/admin/logs/operations", Sort: 51},
	{Name: "查看登录日志", Permission: "system:log:login_list", Type: PermissionTypeButton, ParentName: "登录日志", Sort: 52},
	{Name: "查看登录日志", Permission: "system:log:login_list", Type: PermissionTypeAPI, ParentName: "登录日志", Method: "GET", APIPath: "/admin/logs/logins", Sort: 52},
	{Name: "查看操作日志详情", Permission: "system:log:operation_list", Type: PermissionTypeAPI, ParentName: "操作日志", Method: "GET", APIPath: "/admin/logs/operations/:id", Sort: 53},
	{Name: "查看登录日志详情", Permission: "system:log:login_list", Type: PermissionTypeAPI, ParentName: "登录日志", Method: "GET", APIPath: "/admin/logs/logins/:id", Sort: 54},
	{Name: "导出审计日志", Permission: "system:log:export", Type: PermissionTypeButton, ParentName: "操作日志", Sort: 55},
	{Name: "导出审计日志", Permission: "system:log:export", Type: PermissionTypeAPI, ParentName: "操作日志", Method: "GET", APIPath: "/admin/logs/export", Sort: 55},
	{Name: "日志统计", Permission: "system:log:stats", Type: PermissionTypeButton, ParentName: "操作日志", Sort: 56},
	{Name: "日志统计", Permission: "system:log:stats", Type: PermissionTypeAPI, ParentName: "操作日志", Method: "GET", APIPath: "/admin/logs/stats", Sort: 56},
	{Name: "查看全量日志", Permission: "system:log:view_all", Type: PermissionTypeButton, ParentName: "操作日志", Sort: 57},

	{Name: "上传文件", Permission: "system:upload:file", Type: PermissionTypeButton, ParentName: "管理员管理", Sort: 25},
	{Name: "上传文件", Permission: "system:upload:file", Type: PermissionTypeAPI, ParentName: "管理员管理", Method: "POST", APIPath: "/admin/upload", Sort: 25},

	// ---------- 文件管理 ----------
	{Name: "文件管理", Permission: "system:file:list", Type: PermissionTypeMenu,
		Path: "/system/files", Component: "views/system/file/index.vue", Icon: "file", Sort: 60},
	{Name: "查看文件", Permission: "system:file:list", Type: PermissionTypeButton, ParentName: "文件管理", Sort: 61},
	{Name: "查看文件", Permission: "system:file:list", Type: PermissionTypeAPI, ParentName: "文件管理", Method: "GET", APIPath: "/admin/files", Sort: 61},
	{Name: "重命名文件", Permission: "system:file:rename", Type: PermissionTypeButton, ParentName: "文件管理", Sort: 62},
	{Name: "重命名文件", Permission: "system:file:rename", Type: PermissionTypeAPI, ParentName: "文件管理", Method: "PUT", APIPath: "/admin/files/rename", Sort: 62},
	{Name: "删除文件", Permission: "system:file:delete", Type: PermissionTypeButton, ParentName: "文件管理", Sort: 63},
	{Name: "删除文件", Permission: "system:file:delete", Type: PermissionTypeAPI, ParentName: "文件管理", Method: "POST", APIPath: "/admin/files/delete", Sort: 63},

	// ---------- 系统管理（分组，用于演示多级菜单层级） ----------
	{Name: "系统管理", Permission: "system:group:system", Type: PermissionTypeMenu,
		Path: "", Component: "", Icon: "setting", Sort: 65},

	// ---------- 数据字典 ----------
	{Name: "字典管理", Permission: "system:dict:list", Type: PermissionTypeMenu,
		Path: "/system/dicts", Component: "views/system/dict/index.vue", Icon: "dict", Sort: 70, ParentName: "系统管理"},
	{Name: "查看字典", Permission: "system:dict:list", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 71},
	{Name: "查看字典", Permission: "system:dict:list", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "GET", APIPath: "/admin/dicts", Sort: 71},
	{Name: "新增字典", Permission: "system:dict:add", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 72},
	{Name: "新增字典", Permission: "system:dict:add", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "POST", APIPath: "/admin/dicts", Sort: 72},
	{Name: "编辑字典", Permission: "system:dict:edit", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 73},
	{Name: "编辑字典", Permission: "system:dict:edit", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "PUT", APIPath: "/admin/dicts/:id", Sort: 73},
	{Name: "删除字典", Permission: "system:dict:delete", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 74},
	{Name: "删除字典", Permission: "system:dict:delete", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "DELETE", APIPath: "/admin/dicts/:id", Sort: 74},
	{Name: "查看字典项", Permission: "system:dict_item:list", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 75},
	{Name: "查看字典项", Permission: "system:dict_item:list", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "GET", APIPath: "/admin/dicts/items", Sort: 75},
	{Name: "新增字典项", Permission: "system:dict_item:add", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 76},
	{Name: "新增字典项", Permission: "system:dict_item:add", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "POST", APIPath: "/admin/dicts/items", Sort: 76},
	{Name: "编辑字典项", Permission: "system:dict_item:edit", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 77},
	{Name: "编辑字典项", Permission: "system:dict_item:edit", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "PUT", APIPath: "/admin/dicts/items/:id", Sort: 77},
	{Name: "删除字典项", Permission: "system:dict_item:delete", Type: PermissionTypeButton, ParentName: "字典管理", Sort: 78},
	{Name: "删除字典项", Permission: "system:dict_item:delete", Type: PermissionTypeAPI, ParentName: "字典管理", Method: "DELETE", APIPath: "/admin/dicts/items/:id", Sort: 78},

	// ---------- 系统配置 ----------
	{Name: "系统配置", Permission: "system:config:list", Type: PermissionTypeMenu,
		Path: "/system/configs", Component: "views/system/config/index.vue", Icon: "setting", Sort: 80, ParentName: "系统管理"},
	{Name: "查看配置", Permission: "system:config:list", Type: PermissionTypeButton, ParentName: "系统配置", Sort: 81},
	{Name: "查看配置", Permission: "system:config:list", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "GET", APIPath: "/admin/configs", Sort: 81},
	{Name: "新增配置", Permission: "system:config:add", Type: PermissionTypeButton, ParentName: "系统配置", Sort: 82},
	{Name: "新增配置", Permission: "system:config:add", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "POST", APIPath: "/admin/configs", Sort: 82},
	{Name: "编辑配置", Permission: "system:config:edit", Type: PermissionTypeButton, ParentName: "系统配置", Sort: 83},
	{Name: "编辑配置", Permission: "system:config:edit", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "PUT", APIPath: "/admin/configs/:id", Sort: 83},
	{Name: "删除配置", Permission: "system:config:delete", Type: PermissionTypeButton, ParentName: "系统配置", Sort: 84},
	{Name: "删除配置", Permission: "system:config:delete", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "DELETE", APIPath: "/admin/configs/:id", Sort: 84},
	// 配置增强接口：复用已有列表/编辑权限，避免为站点设置页新增授权负担
	{Name: "按 key 读取配置", Permission: "system:config:list", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "GET", APIPath: "/admin/configs/key/:key", Sort: 85},
	{Name: "配置详情", Permission: "system:config:list", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "GET", APIPath: "/admin/configs/:id", Sort: 86},
	{Name: "批量保存配置", Permission: "system:config:edit", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "POST", APIPath: "/admin/configs/batch", Sort: 87},
	{Name: "配置变更历史", Permission: "system:config:list", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "GET", APIPath: "/admin/config-versions/:id", Sort: 88},
	{Name: "配置回滚", Permission: "system:config:edit", Type: PermissionTypeAPI, ParentName: "系统配置", Method: "POST", APIPath: "/admin/config-versions/:id/rollback", Sort: 89},

	// ---------- 内容管理（分组与菜单，父级必须写在子级之前） ----------
	{Name: "内容管理", Permission: "cms:group:content", Type: PermissionTypeMenu,
		Path: "", Component: "", Icon: "content", Sort: 100},
	{Name: "文章管理", Permission: "cms:article:list", Type: PermissionTypeMenu,
		Path: "/content/articles", Component: "views/content/article/index.vue", Icon: "article", Sort: 101, ParentName: "内容管理"},
	{Name: "分类管理", Permission: "cms:category:list", Type: PermissionTypeMenu,
		Path: "/content/categories", Component: "views/content/category/index.vue", Icon: "category", Sort: 102, ParentName: "内容管理"},
	{Name: "标签管理", Permission: "cms:tag:list", Type: PermissionTypeMenu,
		Path: "/content/tags", Component: "views/content/tag/index.vue", Icon: "tag", Sort: 103, ParentName: "内容管理"},

	// ---------- 文章管理（按钮 / API） ----------
	{Name: "查看文章", Permission: "cms:article:list", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 110},
	{Name: "查看文章", Permission: "cms:article:list", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "GET", APIPath: "/admin/articles", Sort: 110},
	{Name: "文章详情", Permission: "cms:article:detail", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 111},
	{Name: "文章详情", Permission: "cms:article:detail", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "GET", APIPath: "/admin/articles/:id", Sort: 111},
	{Name: "新增文章", Permission: "cms:article:add", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 112},
	{Name: "新增文章", Permission: "cms:article:add", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles", Sort: 112},
	{Name: "编辑文章", Permission: "cms:article:edit", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 113},
	{Name: "编辑文章", Permission: "cms:article:edit", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "PUT", APIPath: "/admin/articles/:id", Sort: 113},
	{Name: "删除文章", Permission: "cms:article:delete", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 114},
	{Name: "删除文章", Permission: "cms:article:delete", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "DELETE", APIPath: "/admin/articles/:id", Sort: 114},
	{Name: "发布文章", Permission: "cms:article:publish", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 115},
	{Name: "发布文章", Permission: "cms:article:publish", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/:id/publish", Sort: 115},
	{Name: "撤回文章", Permission: "cms:article:withdraw", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 116},
	{Name: "撤回文章", Permission: "cms:article:withdraw", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/:id/withdraw", Sort: 116},
	{Name: "置顶文章", Permission: "cms:article:toggle_top", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 117},
	{Name: "置顶文章", Permission: "cms:article:toggle_top", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/:id/toggle-top", Sort: 117},
	// 批量操作复用单条撤回 / 删除权限码，但 API 路径必须单独登记，否则非超管会被权限中间件拒绝
	{Name: "批量下线", Permission: "cms:article:withdraw", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/batch-offline", Sort: 118},
	{Name: "批量删除", Permission: "cms:article:delete", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/batch-delete", Sort: 118},
	{Name: "导出内容包", Permission: "cms:content:export", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "GET", APIPath: "/admin/content/export", Sort: 150},
	{Name: "导入内容包", Permission: "cms:content:import", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/content/import", Sort: 151},
	{Name: "运行监控", Permission: "system:monitor:view", Type: PermissionTypeAPI, ParentName: "控制台", Method: "GET", APIPath: "/admin/monitor", Sort: 152},

	// ---------- 站点扩展（友链 / 评论 / 订阅） ----------
	{Name: "站点扩展", Permission: "system:siteext:list", Type: PermissionTypeMenu,
		Path: "/system/siteext", Component: "views/system/siteext/index.vue", Icon: "link", Sort: 90, ParentName: "系统管理"},
	{Name: "管理友链", Permission: "system:siteext:link", Type: PermissionTypeButton, ParentName: "站点扩展", Sort: 91},
	{Name: "友链列表", Permission: "system:siteext:link", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "GET", APIPath: "/admin/links", Sort: 91},
	{Name: "新增友链", Permission: "system:siteext:link", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "POST", APIPath: "/admin/links", Sort: 92},
	{Name: "编辑友链", Permission: "system:siteext:link", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "PUT", APIPath: "/admin/links/:id", Sort: 93},
	{Name: "删除友链", Permission: "system:siteext:link", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "DELETE", APIPath: "/admin/links/:id", Sort: 94},
	{Name: "管理评论", Permission: "system:siteext:comment", Type: PermissionTypeButton, ParentName: "站点扩展", Sort: 95},
	{Name: "评论列表", Permission: "system:siteext:comment", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "GET", APIPath: "/admin/comments", Sort: 95},
	{Name: "审核评论", Permission: "system:siteext:comment", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "PUT", APIPath: "/admin/comments/:id/status", Sort: 96},
	{Name: "删除评论", Permission: "system:siteext:comment", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "DELETE", APIPath: "/admin/comments/:id", Sort: 97},
	{Name: "管理订阅", Permission: "system:siteext:subscriber", Type: PermissionTypeButton, ParentName: "站点扩展", Sort: 98},
	{Name: "订阅列表", Permission: "system:siteext:subscriber", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "GET", APIPath: "/admin/subscribers", Sort: 98},
	{Name: "删除订阅", Permission: "system:siteext:subscriber", Type: PermissionTypeAPI, ParentName: "站点扩展", Method: "DELETE", APIPath: "/admin/subscribers/:id", Sort: 99},

	// ---------- 分类管理（按钮 / API） ----------
	{Name: "查看分类", Permission: "cms:category:list", Type: PermissionTypeButton, ParentName: "分类管理", Sort: 120},
	{Name: "查看分类", Permission: "cms:category:list", Type: PermissionTypeAPI, ParentName: "分类管理", Method: "GET", APIPath: "/admin/categories", Sort: 120},
	{Name: "新增分类", Permission: "cms:category:add", Type: PermissionTypeButton, ParentName: "分类管理", Sort: 121},
	{Name: "新增分类", Permission: "cms:category:add", Type: PermissionTypeAPI, ParentName: "分类管理", Method: "POST", APIPath: "/admin/categories", Sort: 121},
	{Name: "编辑分类", Permission: "cms:category:edit", Type: PermissionTypeButton, ParentName: "分类管理", Sort: 122},
	{Name: "编辑分类", Permission: "cms:category:edit", Type: PermissionTypeAPI, ParentName: "分类管理", Method: "PUT", APIPath: "/admin/categories/:id", Sort: 122},
	{Name: "删除分类", Permission: "cms:category:delete", Type: PermissionTypeButton, ParentName: "分类管理", Sort: 123},
	{Name: "删除分类", Permission: "cms:category:delete", Type: PermissionTypeAPI, ParentName: "分类管理", Method: "DELETE", APIPath: "/admin/categories/:id", Sort: 123},

	// ---------- 标签管理（按钮 / API） ----------
	{Name: "查看标签", Permission: "cms:tag:list", Type: PermissionTypeButton, ParentName: "标签管理", Sort: 130},
	{Name: "查看标签", Permission: "cms:tag:list", Type: PermissionTypeAPI, ParentName: "标签管理", Method: "GET", APIPath: "/admin/tags", Sort: 130},
	{Name: "新增标签", Permission: "cms:tag:add", Type: PermissionTypeButton, ParentName: "标签管理", Sort: 131},
	{Name: "新增标签", Permission: "cms:tag:add", Type: PermissionTypeAPI, ParentName: "标签管理", Method: "POST", APIPath: "/admin/tags", Sort: 131},
	{Name: "编辑标签", Permission: "cms:tag:edit", Type: PermissionTypeButton, ParentName: "标签管理", Sort: 132},
	{Name: "编辑标签", Permission: "cms:tag:edit", Type: PermissionTypeAPI, ParentName: "标签管理", Method: "PUT", APIPath: "/admin/tags/:id", Sort: 132},
	{Name: "删除标签", Permission: "cms:tag:delete", Type: PermissionTypeButton, ParentName: "标签管理", Sort: 133},
	{Name: "删除标签", Permission: "cms:tag:delete", Type: PermissionTypeAPI, ParentName: "标签管理", Method: "DELETE", APIPath: "/admin/tags/:id", Sort: 133},

	// ---------- 回收站（菜单 / 按钮 / API） ----------
	{Name: "回收站", Permission: "cms:trash:list", Type: PermissionTypeMenu,
		Path: "/content/trash", Component: "views/content/trash/index.vue", Icon: "trash", Sort: 104, ParentName: "内容管理"},
	{Name: "查看回收站", Permission: "cms:trash:list", Type: PermissionTypeButton, ParentName: "回收站", Sort: 140},
	{Name: "查看回收站", Permission: "cms:trash:list", Type: PermissionTypeAPI, ParentName: "回收站", Method: "GET", APIPath: "/admin/trash", Sort: 140},
	{Name: "恢复内容", Permission: "cms:trash:restore", Type: PermissionTypeButton, ParentName: "回收站", Sort: 141},
	{Name: "恢复内容", Permission: "cms:trash:restore", Type: PermissionTypeAPI, ParentName: "回收站", Method: "POST", APIPath: "/admin/trash/:type/:id/restore", Sort: 141},
	{Name: "彻底删除", Permission: "cms:trash:purge", Type: PermissionTypeButton, ParentName: "回收站", Sort: 142},
	{Name: "彻底删除", Permission: "cms:trash:purge", Type: PermissionTypeAPI, ParentName: "回收站", Method: "DELETE", APIPath: "/admin/trash/:type/:id/purge", Sort: 142},

	// ---------- 审核流与修订历史（按钮 / API） ----------
	{Name: "提交审核", Permission: "cms:article:submit", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 118},
	{Name: "提交审核", Permission: "cms:article:submit", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/:id/submit", Sort: 118},
	{Name: "审核通过", Permission: "cms:article:approve", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 119},
	{Name: "审核通过", Permission: "cms:article:approve", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/:id/approve", Sort: 119},
	{Name: "驳回审核", Permission: "cms:article:reject", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 120},
	{Name: "驳回审核", Permission: "cms:article:reject", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/:id/reject", Sort: 120},
	{Name: "查看修订历史", Permission: "cms:article:revision", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 121},
	{Name: "查看修订历史", Permission: "cms:article:revision", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "GET", APIPath: "/admin/articles/:id/revisions", Sort: 121},
	{Name: "修订版本详情", Permission: "cms:article:revision", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "GET", APIPath: "/admin/articles/:id/revisions/:rid", Sort: 121},
	{Name: "恢复修订版本", Permission: "cms:article:revision_restore", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 122},
	{Name: "恢复修订版本", Permission: "cms:article:revision_restore", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "POST", APIPath: "/admin/articles/:id/revisions/:rid/restore", Sort: 122},
	{Name: "查看审核记录", Permission: "cms:article:review", Type: PermissionTypeButton, ParentName: "文章管理", Sort: 123},
	{Name: "查看审核记录", Permission: "cms:article:review", Type: PermissionTypeAPI, ParentName: "文章管理", Method: "GET", APIPath: "/admin/articles/:id/reviews", Sort: 123},
}

// legacyPermissions 历史版本的权限标识，已被 DefaultPermissions 中的新标识取代。
// 首次运行迁移生成的旧记录若不清理，会出现在权限列表里成为无法解释的孤儿数据。
var legacyPermissions = []struct {
	Type       int
	Permission string
}{
	{PermissionTypeMenu, "dashboard:view"},
}

// CleanupLegacyPermissions 清理已被废弃的历史权限记录（幂等）
func CleanupLegacyPermissions(ctx context.Context, tx *ent.Tx) error {
	for _, legacy := range legacyPermissions {
		deleted, err := tx.AdminPermission.
			Delete().
			Where(
				adminpermission.TypeEQ(legacy.Type),
				adminpermission.PermissionEQ(legacy.Permission),
			).
			Exec(ctx)
		if err != nil {
			return err
		}
		if deleted > 0 {
			log.Printf("🧹 清理废弃权限 %s %d 条", legacy.Permission, deleted)
		}
	}
	return nil
}

// UpsertPermissions 幂等写入权限清单，返回全部权限实体。
//
// 菜单/按钮按 (type, permission) 去重；API 权限按 (type, permission, method, api_path) 去重，
// 允许同一个权限码绑定多条接口（例如 cms:article:delete 同时覆盖单条删除与批量删除）。
// 早期版本对 API 也只按 (type, permission) 去重，后写入的 spec 会把前面的接口记录
// 覆盖掉，导致非超管访问这些接口被权限中间件以「未登记权限」拒绝。
// 现在表上有 (type, permission, method, api_path) 唯一索引，且整个种子过程在单事务 +
// 迁移锁内执行，并发的 migrate 进程不会插入重复记录。
func UpsertPermissions(ctx context.Context, tx *ent.Tx, specs []PermissionSpec) ([]*ent.AdminPermission, error) {
	result := make([]*ent.AdminPermission, 0, len(specs))
	// 菜单名称 -> 实体 ID，用于解析 ParentName
	parentIndex := make(map[string]int64, len(specs))

	writePermission := func(spec PermissionSpec, parentID int64) (*ent.AdminPermission, error) {
		query := tx.AdminPermission.
			Query().
			Where(
				adminpermission.TypeEQ(spec.Type),
				adminpermission.PermissionEQ(spec.Permission),
			)
		if spec.Type == PermissionTypeAPI {
			query = query.Where(
				adminpermission.MethodEqualFold(spec.Method),
				adminpermission.APIPathEQ(spec.APIPath),
			)
		}
		existing, err := query.First(ctx)
		if err == nil {
			// 已存在则同步展示信息与接口属性
			update := existing.Update().
				SetName(spec.Name).
				SetSort(spec.Sort).
				SetIsActive(true)
			if spec.Type == PermissionTypeMenu {
				update = update.SetPath(spec.Path).SetComponent(spec.Component).SetIcon(spec.Icon)
				if parentID > 0 {
					update = update.SetParentID(parentID)
				}
			} else {
				update = update.SetMethod(spec.Method).SetAPIPath(spec.APIPath)
				if parentID > 0 {
					update = update.SetParentID(parentID)
				}
			}
			return update.Save(ctx)
		}
		if !ent.IsNotFound(err) {
			return nil, err
		}

		create := tx.AdminPermission.
			Create().
			SetName(spec.Name).
			SetPermission(spec.Permission).
			SetType(spec.Type).
			SetSort(spec.Sort).
			SetIsActive(true)
		if spec.Type == PermissionTypeMenu {
			create = create.SetPath(spec.Path).SetComponent(spec.Component).SetIcon(spec.Icon)
			if parentID > 0 {
				create = create.SetParentID(parentID)
			}
		} else {
			create = create.SetMethod(spec.Method).SetAPIPath(spec.APIPath)
			if parentID > 0 {
				create = create.SetParentID(parentID)
			}
		}

		return create.Save(ctx)
	}

	// 1. 菜单（父级菜单需在切片中位于其子菜单之前，parentIndex 才能正确解析）
	for _, spec := range specs {
		if spec.Type != PermissionTypeMenu {
			continue
		}
		var parentID int64
		if spec.ParentName != "" {
			parentID = parentIndex[spec.ParentName]
		}
		p, err := writePermission(spec, parentID)
		if err != nil {
			return nil, err
		}
		parentIndex[spec.Name] = p.ID
		result = append(result, p)
	}

	// 2. 按钮与 API
	for _, spec := range specs {
		if spec.Type == PermissionTypeMenu {
			continue
		}
		var parentID int64
		if spec.ParentName != "" {
			parentID = parentIndex[spec.ParentName]
		}
		p, err := writePermission(spec, parentID)
		if err != nil {
			return nil, err
		}
		parentIndex[spec.Name] = p.ID
		result = append(result, p)
	}

	return result, nil
}
