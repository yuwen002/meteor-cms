package middleware

import (
	"net/http"
	"sync"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

// NewCommentRateLimitMiddleware 对公开评论提交按客户端 IP 施加独立配额。
//
// 全局公开限流是全部 /api/ 请求共享的（默认 60 次/分钟），量级远大于评论写入；
// feature_comment_enabled 打开后需要单独收紧，避免单个 IP 灌水或刷待审队列。
// 复用 ipRateLimiter 的 Redis + 内存兜底实现，Redis 不可用时不会失效。
// max<=0 或 window<=0 时直接透传，便于通过环境变量临时关闭。
func NewCommentRateLimitMiddleware(rds *redis.Redis, max, window int) rest.Middleware {
	if max <= 0 || window <= 0 {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return next
		}
	}
	limiter := &ipRateLimiter{
		rds: rds, prefix: "comment_rl:", max: max, window: window,
		blockSeconds: 0, mem: &sync.Map{},
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/comments" {
				next(w, r)
				return
			}
			blocked, retryAfter := limiter.allow(clientIP(r))
			if blocked {
				writeRateLimited(w, retryAfter)
				return
			}
			next(w, r)
		}
	}
}
