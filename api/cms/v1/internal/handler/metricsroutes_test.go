package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerDisabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metricsHandler(false, "")(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("关闭时应返回 404，实际 %d", rec.Code)
	}
}

func TestMetricsHandlerToken(t *testing.T) {
	const token = "metrics-token-2026-abcdef"
	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + token, http.StatusUnauthorized},
		{"wrong token", "Bearer wrong-token", http.StatusUnauthorized},
		{"valid", "Bearer " + token, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			metricsHandler(true, token)(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("状态码 = %d，期望 %d", rec.Code, tc.want)
			}
			if tc.want != http.StatusOK {
				return
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
				t.Errorf("Content-Type = %q", got)
			}
			for _, want := range []string{"meteor_cms_requests_total", "meteor_cms_build_info"} {
				if !strings.Contains(rec.Body.String(), want) {
					t.Errorf("响应缺少指标 %s", want)
				}
			}
		})
	}
}

// 内网部署时可以不设 token，由网络层负责隔离；此时抓取应免鉴权。
func TestMetricsHandlerWithoutTokenAllowsScrape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metricsHandler(true, "")(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("未设置 token 时应免鉴权抓取，实际 %d", rec.Code)
	}
}
