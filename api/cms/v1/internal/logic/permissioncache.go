package logic

import "github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"

// invalidatePermissionCache 权限定义变更后立即失效 API 权限快照。
// permcheck 的 apiperms 缓存按 generation 索引，TTL 60 秒；不主动失效的话
// 后台新建/停用一条 API 权限后最长 60 秒鉴权仍按旧规则执行。
func invalidatePermissionCache(svcCtx *svc.ServiceContext) {
	if svcCtx != nil && svcCtx.PermissionChecker != nil {
		svcCtx.PermissionChecker.InvalidateAll()
	}
}
