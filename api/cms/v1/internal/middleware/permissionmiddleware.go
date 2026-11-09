package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/config"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/permcheck"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var permissionBypassRoutes = map[string]struct{}{
	http.MethodPost + " /admin/logout":                        {},
	http.MethodPost + " /admin/logout-all":                    {}, // 任意登录用户可注销自己的全部会话
	http.MethodGet + " /admin/test-token":                     {},
	http.MethodPut + " /admin/admin-users/me/change-password": {},
	// 个人中心相关接口必须放行：普通管理员在拿到任何权限之前需要取回菜单，
	// 并需要维护自己的资料（不涉及角色与启用状态）
	http.MethodGet + " /admin/profile/permissions": {},
	http.MethodGet + " /admin/profile":             {},
	http.MethodPut + " /admin/profile":             {},
}

type PermissionMiddleware struct {
	config  *config.Config
	db      *ent.Client
	checker *permcheck.Checker
}

type matchedPermission struct {
	permission *ent.AdminPermission
	staticHits int
}

func NewPermissionMiddleware(c *config.Config, db *ent.Client, checker *permcheck.Checker) rest.Middleware {
	m := &PermissionMiddleware{
		config:  c,
		db:      db,
		checker: checker,
	}
	return m.Handle
}

func (m *PermissionMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := utils.CurrentUserID(r.Context())
		if !ok {
			common.Fail(w, common.ErrMissingUserContext, common.GetErrorMessage(common.ErrMissingUserContext))
			return
		}

		user, err := m.db.AdminUser.
			Query().
			Where(adminuser.IDEQ(userID)).
			Only(r.Context())

		if err != nil {
			if ent.IsNotFound(err) {
				common.Fail(w, common.ErrMissingUserContext, common.GetErrorMessage(common.ErrMissingUserContext))
				return
			}
			logx.Errorf("query admin user failed: %v", err)
			common.Fail(w, common.ErrInternalServer, common.GetErrorMessage(common.ErrInternalServer))
			return
		}

		if !user.IsActive {
			common.Fail(w, common.ErrAccountInactive, common.GetErrorMessage(common.ErrAccountInactive))
			return
		}

		if user.IsSuper || isPermissionBypassRoute(r.Method, r.URL.Path) {
			next(w, r)
			return
		}

		perms, err := m.checker.UserPermissions(r.Context(), userID)
		if err != nil {
			logx.Errorf("load user permissions failed: user_id=%d err=%v", userID, err)
			common.Fail(w, common.ErrInternalServer, common.GetErrorMessage(common.ErrInternalServer))
			return
		}

		permission, err := m.findPermissionForRequest(r.Context(), r.Method, r.URL.Path)
		if err != nil {
			logx.Errorf("match api permission failed: method=%s path=%s err=%v", r.Method, r.URL.Path, err)
			common.Fail(w, common.ErrInternalServer, common.GetErrorMessage(common.ErrInternalServer))
			return
		}
		if permission == nil {
			logx.Errorf("api permission missing: user_id=%d method=%s path=%s", userID, r.Method, r.URL.Path)
			common.Fail(w, common.ErrPermissionDenied, common.GetErrorMessage(common.ErrPermissionDenied))
			return
		}
		if !permcheck.Has(perms, permission.Permission) {
			logx.Errorf("permission denied: user_id=%d method=%s path=%s permission=%s", userID, r.Method, r.URL.Path, permission.Permission)
			common.Fail(w, common.ErrPermissionDenied, common.GetErrorMessage(common.ErrPermissionDenied))
			return
		}

		// 把中间件判定所用的同一份权限快照传给下游 logic，
		// 文章行级守卫据此判定审核角色，避免重复查库与两套结论不一致。
		r = r.WithContext(permcheck.WithPermissions(r.Context(), perms))
		next(w, r)
	}
}

// findPermissionForRequest 从共享缓存取全量 API 权限定义后在内存中做路径匹配。
// 缓存 key 与具体 URL 无关，路径型接口不会随 ID 增长产生大量缓存条目。
func (m *PermissionMiddleware) findPermissionForRequest(ctx context.Context, method, requestPath string) (*ent.AdminPermission, error) {
	permissions, err := m.checker.APIPermissions(ctx)
	if err != nil {
		return nil, err
	}

	var best *matchedPermission
	for _, permission := range permissions {
		if !strings.EqualFold(permission.Method, method) {
			continue
		}
		staticHits, matched := matchPermissionPath(permission.APIPath, requestPath)
		if !matched {
			continue
		}

		candidate := &matchedPermission{
			permission: permission,
			staticHits: staticHits,
		}

		if best == nil || candidate.staticHits > best.staticHits {
			best = candidate
			continue
		}

		if candidate.staticHits == best.staticHits {
			logx.Errorf("ambiguous api permission config: method=%s path=%s matched %q and %q", method, requestPath, best.permission.APIPath, candidate.permission.APIPath)
			return nil, nil
		}
	}

	if best == nil {
		return nil, nil
	}

	return best.permission, nil
}

func isPermissionBypassRoute(method, path string) bool {
	_, ok := permissionBypassRoutes[strings.ToUpper(method)+" "+normalizePath(path)]
	return ok
}

func matchPermissionPath(templatePath, requestPath string) (int, bool) {
	templateSegments := splitPathSegments(templatePath)
	requestSegments := splitPathSegments(requestPath)
	if len(templateSegments) != len(requestSegments) {
		return 0, false
	}

	staticHits := 0
	for i := range templateSegments {
		templateSegment := templateSegments[i]
		requestSegment := requestSegments[i]

		if isPathParamSegment(templateSegment) {
			if requestSegment == "" {
				return 0, false
			}
			continue
		}

		if templateSegment != requestSegment {
			return 0, false
		}
		staticHits++
	}

	return staticHits, true
}

func splitPathSegments(path string) []string {
	normalized := normalizePath(path)
	if normalized == "/" {
		return []string{}
	}

	return strings.Split(strings.TrimPrefix(normalized, "/"), "/")
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}

	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}

	return trimmed
}

func isPathParamSegment(segment string) bool {
	return strings.HasPrefix(segment, ":") && len(segment) > 1
}
