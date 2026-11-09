package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// healthHandler 供容器编排 / 负载均衡探活：数据库必须可用，Redis 只作为降级信息
// （项目对 Redis 故障有内存降级，不应因缓存抖动把整个服务标记为不健康）。
//
// 走极轻量的 EXISTS 查询而非业务写入，并设置 2 秒超时避免慢依赖拖住探活请求。
func healthHandler(serverCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		database := "up"
		if _, err := serverCtx.EntClient.AdminUser.Query().Exist(ctx); err != nil {
			logx.Errorf("healthz database check failed: %v", err)
			database = "down"
		}

		redis := "disabled"
		if serverCtx.Redis != nil {
			redis = "up"
			if !serverCtx.Redis.Ping() {
				redis = "down"
			}
		}

		status := "ok"
		code := http.StatusOK
		if database == "down" {
			status = "unhealthy"
			code = http.StatusServiceUnavailable
		} else if redis == "down" {
			status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":     0,
			"msg":      status,
			"database": database,
			"redis":    redis,
		})
	}
}
