package handler

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/yuwen002/meteor-cms/internal/pkg/metrics"
	"github.com/zeromicro/go-zero/rest"
)

// RegisterMetricsRoutes 注册 Prometheus 抓取端点 /metrics。
// 默认关闭（返回 404）；开启后若配置了 token，则要求 Authorization: Bearer <token>。
func RegisterMetricsRoutes(server *rest.Server, enabled bool, token string) {
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/metrics", Handler: metricsHandler(enabled, token)},
	})
}

func metricsHandler(enabled bool, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			http.NotFound(w, r)
			return
		}
		if token != "" && !bearerTokenMatches(r.Header.Get("Authorization"), token) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", metrics.PrometheusContentType)
		w.Header().Set("Cache-Control", "no-store")
		if err := metrics.WritePrometheus(w); err != nil {
			http.Error(w, "metrics unavailable", http.StatusInternalServerError)
		}
	}
}

// bearerTokenMatches 用常量时间比较，避免 token 被逐字节计时探测。
func bearerTokenMatches(header, want string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
