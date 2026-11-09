// Package permcheck 提供统一的权限快照与短 TTL 缓存。
// 权限中间件与文章行级守卫共用同一份快照，避免出现「中间件已拒绝、守卫却放行」的窗口。
package permcheck

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminpermission"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/pkg/cacheversion"
	"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const cacheTTL = 60 * time.Second

type permissionsKey struct{}

// WithPermissions 把本次请求解析出的权限集合写入 ctx，供下游 logic 复用。
func WithPermissions(ctx context.Context, perms []string) context.Context {
	return context.WithValue(ctx, permissionsKey{}, perms)
}

// PermissionsFrom 取出中间件注入的权限快照；ok=false 表示请求未经过权限校验。
func PermissionsFrom(ctx context.Context) ([]string, bool) {
	perms, ok := ctx.Value(permissionsKey{}).([]string)
	return perms, ok
}

// Has 判断权限集合中是否包含指定标识。
func Has(perms []string, permission string) bool {
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAny 判断权限集合中是否包含任一候选标识。
func HasAny(perms []string, candidates ...string) bool {
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	for _, c := range candidates {
		if _, ok := set[c]; ok {
			return true
		}
	}
	return false
}

// Checker 缓存用户权限与全量 API 权限定义。缓存 key 带版本号，
// 权限变更时递增版本号即可让旧条目立刻失效，无需遍历删除。
type Checker struct {
	db           *ent.Client
	cache        *collection.Cache
	generation   atomic.Int64
	version      *cacheversion.Sentinel
	mu           sync.Mutex
	userVersions map[int64]int64
}

// New 的可选 rds 用于跨实例失效；不传时退化为纯进程内 TTL。
func New(db *ent.Client, rds ...*redis.Redis) (*Checker, error) {
	cache, err := collection.NewCache(cacheTTL)
	if err != nil {
		return nil, err
	}
	var r *redis.Redis
	if len(rds) > 0 {
		r = rds[0]
	}
	return &Checker{
		db:           db,
		cache:        cache,
		version:      cacheversion.New(r, "cms:cache:permcheck:version", 0),
		userVersions: make(map[int64]int64),
	}, nil
}

// InvalidateAll 用于角色/权限定义变更（授权、停用、删除角色等）。
func (c *Checker) InvalidateAll() {
	c.generation.Add(1)
	c.version.Bump()
}

// InvalidateUser 用于单个用户的角色绑定变更。
// Bump 全局版本后，其它实例的 generation 也会前移，缓存 key 随之失效。
func (c *Checker) InvalidateUser(userID int64) {
	c.mu.Lock()
	c.userVersions[userID]++
	c.mu.Unlock()
	c.version.Bump()
}

// syncGeneration 在读缓存前检查其它实例是否改过权限定义。
// generation 进入 cache key，递增即等价于全量失效，不需要遍历删除条目。
func (c *Checker) syncGeneration() {
	if c.version.Changed() {
		c.generation.Add(1)
	}
}

func (c *Checker) userVersion(userID int64) int64 {
	c.mu.Lock()
	v := c.userVersions[userID]
	c.mu.Unlock()
	return v
}

// UserPermissions 返回用户当前拥有的全部权限标识。
func (c *Checker) UserPermissions(ctx context.Context, userID int64) ([]string, error) {
	c.syncGeneration()
	key := fmt.Sprintf("perm:%d:%d:%d", c.generation.Load(), userID, c.userVersion(userID))
	v, err := c.cache.Take(key, func() (interface{}, error) {
		var perms []string
		err := c.db.AdminUser.
			Query().
			Where(adminuser.IDEQ(userID)).
			QueryRoles().
			Where(adminrole.IsActive(true)).
			QueryPermissions().
			Where(adminpermission.IsActive(true)).
			Select(adminpermission.FieldPermission).
			Scan(ctx, &perms)
		if err != nil {
			return nil, err
		}
		return perms, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]string), nil
}

// APIPermissions 缓存全量启用中的 API 权限定义；路径匹配在内存中完成，
// 避免每个具体 URL（/admin/articles/1、/admin/articles/2 …）各占一条缓存 key。
func (c *Checker) APIPermissions(ctx context.Context) ([]*ent.AdminPermission, error) {
	c.syncGeneration()
	key := fmt.Sprintf("apiperms:%d", c.generation.Load())
	v, err := c.cache.Take(key, func() (interface{}, error) {
		return c.db.AdminPermission.
			Query().
			Where(
				adminpermission.TypeEQ(3),
				adminpermission.IsActive(true),
			).
			All(ctx)
	})
	if err != nil {
		return nil, err
	}
	return v.([]*ent.AdminPermission), nil
}
