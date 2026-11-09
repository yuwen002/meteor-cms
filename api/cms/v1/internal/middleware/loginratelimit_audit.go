package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/pkg/audit"
	"github.com/yuwen002/meteor-cms/internal/utils"
)

// RecordRateLimitedLogin 为被登录限流拦截的请求补记一条失败登录日志。
// 这类请求在全局中间件就被拒绝，不会进入 LoginLogic 的登录日志链路，
// 但它恰恰是爆破尝试最需要留痕的部分。
func RecordRateLimitedLogin(r *http.Request, db *ent.Client) {
	username := usernameFromBody(r)
	ip := clientIP(r)
	ua := utils.Truncate(r.UserAgent(), 512)
	// 限流是全局中间件，早于路由组的 ClientInfoMiddleware，需自行解析/生成链路标识
	requestID := utils.ResolveRequestID(r)

	audit.Submit("rate_limited_login_log", func(ctx context.Context) error {
		create := db.LoginLog.Create().
			SetUsername(username).
			SetIP(ip).
			SetUserAgent(ua).
			SetStatus(1).
			SetFailReason("触发登录限流").
			SetLoginAt(time.Now())
		if requestID != "" {
			create = create.SetRequestID(requestID)
		}
		_, err := create.Save(ctx)
		return err
	})
}

// usernameFromBody 读取登录请求体里的账号名并原样回填，避免下游无法继续读取。
// 只用于审计展示，不做任何身份判定。
func usernameFromBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxLogBodyBytes))
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	if v, ok := payload["username"].(string); ok {
		return utils.Truncate(v, 50)
	}
	return ""
}
