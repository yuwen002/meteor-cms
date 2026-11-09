package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 只测内存兜底路径（rds=nil）：这是 Redis 故障时公开写入的最坏情况，
// 也是唯一不依赖外部服务、可在任何机器上复现的基准。
func BenchmarkCommentRateLimitMemoryFallback(b *testing.B) {
	mw := NewCommentRateLimitMiddleware(nil, 1<<30, 60)
	h := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/api/comments", nil)
	req.RemoteAddr = "203.0.113.7:12345"
	rec := httptest.NewRecorder()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h(rec, req)
	}
}
