package middleware

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/internal/utils"
)

// ClientInfoMiddleware 把客户端 IP 与 User-Agent 注入 context。
//
// logic 层拿不到 *http.Request，登录日志、审计日志等需要来源信息的场景都依赖这里注入的值。
type ClientInfoMiddleware struct{}

// NewClientInfoMiddleware 创建客户端信息中间件
func NewClientInfoMiddleware() ClientInfoMiddleware {
	return ClientInfoMiddleware{}
}

// Handle 返回中间件处理函数
func (ClientInfoMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := utils.ResolveRequestID(r)
		w.Header().Set(utils.RequestIDHeader, requestID)
		ctx := utils.WithClientInfo(
			r.Context(),
			utils.ClientIPFromRequest(r),
			utils.Truncate(r.UserAgent(), 512),
		)
		ctx = utils.WithRequest(ctx, r)
		ctx = utils.WithRequestID(ctx, requestID)
		next(w, r.WithContext(ctx))
	}
}
