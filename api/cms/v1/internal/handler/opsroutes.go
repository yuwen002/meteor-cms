package handler

import (
	"context"
	"math"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/audit"
	"github.com/yuwen002/meteor-cms/internal/pkg/metrics"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// monitorHandler 汇总运行时指标，与 /healthz 的区别是：healthz 只做探活，
// 这里给管理员排查「慢 / 抖 / 队列堆积」提供可观测数据。
func monitorHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		database := "up"
		if _, err := svcCtx.EntClient.AdminUser.Query().Exist(ctx); err != nil {
			database = "down"
		}
		redisState := "disabled"
		if svcCtx.Redis != nil {
			redisState = "up"
			if !svcCtx.Redis.Ping() {
				redisState = "down"
			}
		}

		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		total, errs, inflight, uptime := metrics.Snapshot()
		depth, dropped := audit.Stats()
		qps := 0.0
		if sec := uptime.Seconds(); sec > 0 {
			qps = math.Round(float64(total)/sec*100) / 100
		}

		status := "ok"
		if database == "down" {
			status = "unhealthy"
		} else if redisState == "down" {
			status = "degraded"
		}

		common.Ok(w, types.MonitorResp{
			Status:              status,
			Database:            database,
			Redis:               redisState,
			UptimeSeconds:       int64(uptime.Seconds()),
			GoVersion:           runtime.Version(),
			Goroutines:          runtime.NumGoroutine(),
			HeapAllocMB:         math.Round(float64(m.HeapAlloc)/(1<<20)*100) / 100,
			SysMB:               math.Round(float64(m.Sys)/(1<<20)*100) / 100,
			NumGC:               m.NumGC,
			RequestsTotal:       total,
			RequestsServerError: errs,
			RequestsInFlight:    inflight,
			AvgQPS:              qps,
			AuditQueueDepth:     depth,
			AuditQueueDropped:   dropped,
		})
	}
}

// publicSearchHandler 全文检索入口：标题 / 摘要 / 正文一起匹配，复用公开列表的
// 可见性过滤（仅已发布、已到发布时间、未软删除），不会泄露草稿。
func publicSearchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PublicArticleListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		kw := strings.TrimSpace(req.Keyword)
		if kw == "" {
			common.Fail(w, common.ErrInvalidParams, "搜索关键词不能为空")
			return
		}
		if len([]rune(kw)) > 64 {
			common.Fail(w, common.ErrInvalidParams, "搜索关键词过长")
			return
		}
		started := time.Now()
		resp, err := logic.NewPublicArticleLogic(r.Context(), svcCtx).PublicArticleList(&req)
		if err != nil {
			writeLogBizError(w, err)
			return
		}
		common.Ok(w, types.SearchResp{
			Keyword: kw, TookMs: time.Since(started).Milliseconds(),
			Total: resp.Total, Page: resp.Page, PageSize: resp.PageSize, List: resp.List,
		})
	}
}
