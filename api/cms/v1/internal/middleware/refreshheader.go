package middleware

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/rest"
)

// NewRefreshTokenHeaderMiddleware 将请求头 X-Refresh-Token 透传进 context，
// 供登出接口一并吊销刷新令牌使用。该头仅在登出时由前端携带，缺失时不影响其它请求。
func NewRefreshTokenHeaderMiddleware() rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if token := r.Header.Get("X-Refresh-Token"); token != "" {
				r = r.WithContext(utils.WithRefreshTokenCtx(r.Context(), token))
			}
			next(w, r)
		}
	}
}
