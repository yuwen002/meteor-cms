package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func intPtr(v int) *int { return &v }

// 回归：status=0（成功）与「未传 status」必须可区分，否则按成功筛选时会被当成零值忽略。
func TestLogListStatusOptional(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  *int
	}{
		{name: "未筛选", query: "", want: nil},
		{name: "成功", query: "?status=0", want: intPtr(0)},
		{name: "失败", query: "?status=1", want: intPtr(1)},
	}

	for _, tc := range cases {
		t.Run("操作日志/"+tc.name, func(t *testing.T) {
			var req types.OperationLogListReq
			if err := httpx.Parse(httptest.NewRequest("GET", "/admin/logs/operations"+tc.query, nil), &req); err != nil {
				t.Fatalf("解析请求失败: %v", err)
			}
			assertOptionalStatus(t, req.Status, tc.want)
		})
		t.Run("登录日志/"+tc.name, func(t *testing.T) {
			var req types.LoginLogListReq
			if err := httpx.Parse(httptest.NewRequest("GET", "/admin/logs/logins"+tc.query, nil), &req); err != nil {
				t.Fatalf("解析请求失败: %v", err)
			}
			assertOptionalStatus(t, req.Status, tc.want)
		})
	}
}

func assertOptionalStatus(t *testing.T, got, want *int) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("未传 status 时应为 nil，实际为 %d", *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Fatalf("status 解析错误，期望 %d，实际 %v", *want, got)
	}
}
