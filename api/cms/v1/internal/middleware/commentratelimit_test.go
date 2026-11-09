package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestCommentRateLimitBlocksAfterQuota(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb, err := redis.NewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType})
	if err != nil {
		t.Fatal(err)
	}
	m := NewCommentRateLimitMiddleware(rdb, 2, 60)
	next := m(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	call := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "203.0.113.7:12345"
		rec := httptest.NewRecorder()
		next(rec, req)
		return rec
	}

	for i := 0; i < 2; i++ {
		if got := call(http.MethodPost, "/api/comments").Code; got != http.StatusCreated {
			t.Fatalf("配额内第 %d 次提交被拦截: %d", i+1, got)
		}
	}
	blocked := call(http.MethodPost, "/api/comments")
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(blocked.Body.Bytes(), &body); err != nil || body.Code != loginRateLimitCode {
		t.Fatalf("超额提交应返回业务码 %d，实际 body=%q", loginRateLimitCode, blocked.Body.String())
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("被限流时应返回 Retry-After")
	}

	// 同路径的 GET 与其它接口不占评论配额。
	if got := call(http.MethodGet, "/api/comments/some-slug").Code; got != http.StatusCreated {
		t.Fatalf("只读接口不应被评论限流拦截: %d", got)
	}
	if got := call(http.MethodPost, "/api/subscribe").Code; got != http.StatusCreated {
		t.Fatalf("其它公开接口不应被评论限流拦截: %d", got)
	}
}

func TestCommentRateLimitDisabled(t *testing.T) {
	m := NewCommentRateLimitMiddleware(nil, 0, 0)
	next := m(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		next(rec, httptest.NewRequest(http.MethodPost, "/api/comments", nil))
		if rec.Code != http.StatusCreated {
			t.Fatalf("Max<=0 时应直接透传，第 %d 次返回 %d", i+1, rec.Code)
		}
	}
}
