// Package metrics 提供进程级轻量请求指标，供 /admin/monitor 展示。
//
// 只做原子计数，不引入 Prometheus 客户端依赖；外部监控系统可自行轮询
// /admin/monitor 拉取，或后续在此包上扩展 /metrics 输出。
package metrics

import (
	"net/http"
	"sync/atomic"
	"time"
)

var (
	startedAt    = time.Now()
	total        atomic.Int64
	serverErrors atomic.Int64
	inFlight     atomic.Int64
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Middleware 记录请求总数、5xx 数与当前并发；不阻塞、不采样请求体。
func Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		total.Add(1)
		inFlight.Add(1)
		defer inFlight.Add(-1)
		sw := &statusWriter{ResponseWriter: w}
		next(sw, r)
		if sw.code >= http.StatusInternalServerError {
			serverErrors.Add(1)
		}
	}
}

// Snapshot 返回累计请求数、5xx 数、在途请求数与进程运行时长。
func Snapshot() (requestTotal, serverErrorTotal, inFlightCount int64, uptime time.Duration) {
	return total.Load(), serverErrors.Load(), inFlight.Load(), time.Since(startedAt)
}
