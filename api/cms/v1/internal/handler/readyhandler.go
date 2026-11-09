package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/internal/migration"
	"github.com/yuwen002/meteor-cms/internal/version"
)

// readyHandler 是「就绪」探针，与 /healthz 的「存活」语义分离：
//   - /healthz 只看 DB / Redis 是否能连上，供容器 healthcheck 与负载均衡；
//   - /readyz 额外校验 schema_migrations 版本，漏跑迁移的实例不会被接流量。
//
// 与 /healthz 一样不走统一响应信封，便于 Kubernetes / 网关直接按 HTTP 状态码判断。
func readyHandler(serverCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		status, err := migration.ReadStatus(ctx, serverCtx.EntClient)
		database := "up"
		if err != nil {
			database = "down"
		}
		ready := err == nil && status.OK
		code := http.StatusOK
		if !ready {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ready":    ready,
			"database": database,
			"version":  version.String(),
			"schema":   status,
		})
	}
}
