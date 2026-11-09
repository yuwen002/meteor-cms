//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type auditRow struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	Query      string `json:"query"`
	RequestID  string `json:"request_id"`
	BizCode    int    `json:"biz_code"`
	HTTPStatus int    `json:"http_status"`
	Username   string `json:"username"`
	CreatedAt  string `json:"created_at"`
}

// callRaw 用于 CSV 等非 JSON 响应
func callRaw(t *testing.T, method, path, token string) (int, string, string) {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("%s %s: read body: %v", method, path, err)
	}
	return res.StatusCode, res.Header.Get("Content-Type"), string(raw)
}

// TestLogExtendedAudit 覆盖本轮扩展：链路标识 / query 快照 / 业务码、
// 认证组留痕、详情/统计/导出接口，以及超过快照上限的请求体不再被截断。
func TestLogExtendedAudit(t *testing.T) {
	waitHealthy(t)
	admin := login(t, "admin", envOr("CMS_E2E_ADMIN_PASSWORD", "123456"))

	// 探针带 query，用于验证 GET 请求也能回溯筛选条件
	wantCode(t, "probe with query", 0,
		call(t, http.MethodGet, "/admin/roles?page=1&page_size=1", admin, nil))

	var target auditRow
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		r := call(t, http.MethodGet, "/admin/logs/operations?page=1&page_size=100", admin, nil)
		wantCode(t, "list operation logs", 0, r)
		var d struct {
			List []auditRow `json:"list"`
		}
		if err := json.Unmarshal(r.Data, &d); err != nil {
			t.Fatalf("parse operation logs: %v", err)
		}
		for _, row := range d.List {
			if strings.Contains(row.Query, "page_size") {
				target = row
				break
			}
		}
		if target.ID > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if target.ID == 0 {
		t.Fatal("未找到带 query 快照的操作日志")
	}

	// 链路标识与响应码必须落库
	if target.RequestID == "" {
		t.Errorf("操作日志缺少 request_id: %+v", target)
	}
	if target.HTTPStatus == 0 {
		t.Errorf("操作日志缺少 http_status: %+v", target)
	}
	if !strings.Contains(target.Query, "page_size") {
		t.Errorf("query 快照应保留筛选条件: %q", target.Query)
	}
	if strings.Contains(target.Query, "password") {
		t.Errorf("query 快照不应出现敏感键: %q", target.Query)
	}

	// 认证组接口也应有审计记录（登录本身在测试开头发生）
	loginLogs := call(t, http.MethodGet, "/admin/logs/operations?path=/admin/login&page=1&page_size=100", admin, nil)
	wantCode(t, "list login audit logs", 0, loginLogs)
	var loginRows struct {
		Total int64      `json:"total"`
		List  []auditRow `json:"list"`
	}
	if err := json.Unmarshal(loginLogs.Data, &loginRows); err != nil {
		t.Fatalf("parse login audit logs: %v", err)
	}
	if loginRows.Total == 0 {
		t.Errorf("登录请求应留下操作日志（认证组已接入采集中间件）")
	} else if loginRows.List[0].Username == "" {
		t.Errorf("无 JWT 的登录日志也应从参数快照补出账号名: %+v", loginRows.List[0])
	}

	// 详情接口
	detail := call(t, http.MethodGet, "/admin/logs/operations/"+strconv.FormatInt(target.ID, 10), admin, nil)
	wantCode(t, "operation log detail", 0, detail)
	var detailRow auditRow
	if err := json.Unmarshal(detail.Data, &detailRow); err != nil {
		t.Fatalf("parse operation log detail: %v", err)
	}
	if detailRow.ID != target.ID {
		t.Errorf("详情返回了错误的记录: want %d got %d", target.ID, detailRow.ID)
	}

	// 统计接口：近 7 天按天聚合
	stats := call(t, http.MethodGet, "/admin/logs/stats?days=7", admin, nil)
	wantCode(t, "log stats", 0, stats)
	var s struct {
		Days  int `json:"days"`
		ByDay []struct {
			Date string `json:"date"`
		} `json:"by_day"`
	}
	if err := json.Unmarshal(stats.Data, &s); err != nil {
		t.Fatalf("parse log stats: %v", err)
	}
	if s.Days != 7 || len(s.ByDay) != 7 {
		t.Errorf("统计应按天返回 7 组: days=%d by_day=%d", s.Days, len(s.ByDay))
	}

	// CSV 导出（非 JSON 响应，走原始 HTTP）
	status, contentType, body := callRaw(t, http.MethodGet, "/admin/logs/export?type=operation", admin)
	if status != http.StatusOK {
		t.Fatalf("导出操作日志失败: status=%d body=%s", status, body)
	}
	if !strings.Contains(contentType, "text/csv") {
		t.Errorf("导出响应应为 text/csv，实际 %q", contentType)
	}
	if !strings.Contains(body, "链路ID") || !strings.Contains(body, "操作人") {
		t.Errorf("导出 CSV 缺少表头: %q", body[:minLen(len(body), 200)])
	}

	status, contentType, body = callRaw(t, http.MethodGet, "/admin/logs/export?type=login", admin)
	if status != http.StatusOK || !strings.Contains(contentType, "text/csv") {
		t.Errorf("导出登录日志失败: status=%d type=%q body=%s", status, contentType, body[:minLen(len(body), 200)])
	}

	// 非法导出类型应被拒绝
	bad := call(t, http.MethodGet, "/admin/logs/export?type=unknown", admin, nil)
	if bad.Code == 0 {
		t.Errorf("未知导出类型应返回业务错误，实际 code=0")
	}

	// 让本用例产生的异步日志先落盘，避免干扰后续用例的「查询不自放大」断言
	time.Sleep(2 * time.Second)
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}
