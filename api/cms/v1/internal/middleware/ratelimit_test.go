package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Redis 不可用（本用例传 nil 客户端）时仍必须限流，避免缓存故障让公开接口失去防护。
func TestIPRateLimiterMemoryFallback(t *testing.T) {
	l := &ipRateLimiter{
		prefix: "test_rl:", max: 2, window: 60, blockSeconds: 30,
		mem: &sync.Map{},
	}
	for i := 0; i < 2; i++ {
		if blocked, _ := l.allow("1.2.3.4"); blocked {
			t.Fatalf("第 %d 次请求不应被拦截", i+1)
		}
	}
	blocked, retry := l.allow("1.2.3.4")
	if !blocked || retry <= 0 {
		t.Fatalf("超过阈值应被拦截并给出重试时间，blocked=%v retry=%d", blocked, retry)
	}
	if blocked, _ := l.allow("5.6.7.8"); blocked {
		t.Fatal("不同 IP 的计数必须互相隔离")
	}
}

func TestPublicRateLimitMiddlewareScope(t *testing.T) {
	called := 0
	next := func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}
	mw := NewPublicRateLimitMiddleware(nil, true, 1, 60, 30)(next)

	// 后台接口不进入公开限流，即使请求次数超过阈值也不消耗配额
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		mw(rec, httptest.NewRequest(http.MethodGet, "/admin/articles", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("后台接口被误拦截：%d", rec.Code)
		}
	}
	if called != 3 {
		t.Fatalf("后台接口调用次数 = %d, want 3", called)
	}

	first := httptest.NewRecorder()
	mw(first, httptest.NewRequest(http.MethodGet, "/api/articles", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("首次公开请求应放行，got %d", first.Code)
	}

	second := httptest.NewRecorder()
	mw(second, httptest.NewRequest(http.MethodGet, "/api/articles", nil))
	if second.Header().Get("Retry-After") == "" {
		t.Error("限流响应应带 Retry-After")
	}
	var body map[string]interface{}
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析限流响应失败: %v", err)
	}
	if body["code"] != float64(loginRateLimitCode) {
		t.Fatalf("限流业务码 = %v, want %d", body["code"], loginRateLimitCode)
	}
}

func TestLoginRateLimitMiddlewareDisabled(t *testing.T) {
	called := 0
	next := func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}
	mw := NewLoginRateLimitMiddleware(nil, false, 1, 60, 30, nil)(next)
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		mw(rec, httptest.NewRequest(http.MethodPost, "/admin/login", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("关闭限流时应直接透传，got %d", rec.Code)
		}
	}
	if called != 3 {
		t.Fatalf("透传次数 = %d, want 3", called)
	}
}
