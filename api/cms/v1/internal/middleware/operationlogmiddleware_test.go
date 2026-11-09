package middleware

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizeParams(t *testing.T) {
	in := `{"username":"alice","password":"secret123","email":"a@b.com","captcha":"X9Z"}`
	out := sanitizeParams("application/json", in)
	if strings.Contains(out, "secret123") {
		t.Fatal("密码未脱敏")
	}
	if strings.Contains(out, "X9Z") {
		t.Fatal("验证码未脱敏")
	}
	if !strings.Contains(out, "******") {
		t.Fatal("敏感字段未替换为 ******")
	}
	if !strings.Contains(out, "a@b.com") {
		t.Fatal("非敏感字段被误改")
	}
}

func TestSanitizeParams_FormMasked(t *testing.T) {
	got := sanitizeParams("application/x-www-form-urlencoded", "password=secret&page=2")
	if strings.Contains(got, "secret") {
		t.Fatalf("表单密码未脱敏: %s", got)
	}
	if !strings.Contains(got, "page=2") {
		t.Fatalf("非敏感筛选条件应保留: %s", got)
	}
}

func TestSanitizeParams_UnknownTypeDiscarded(t *testing.T) {
	// 未知 Content-Type 无法逐字段脱敏，应直接丢弃避免泄露明文
	if got := sanitizeParams("text/plain", "password=secret"); got != "" {
		t.Fatalf("未知类型应返回空，实际: %s", got)
	}
}

func TestSanitizeParams_NestedMasked(t *testing.T) {
	in := `{"user":{"password":"secret","profile":{"phone":"13800000000"}},"items":[{"accessToken":"abc"}],"email":"a@b.com"}`
	got := sanitizeParams("application/json", in)
	for _, secret := range []string{"secret", "13800000000", "abc"} {
		if strings.Contains(got, secret) {
			t.Fatalf("嵌套/数组内的敏感值 %q 未脱敏: %s", secret, got)
		}
	}
	if !strings.Contains(got, "a@b.com") {
		t.Fatalf("非敏感字段被误改: %s", got)
	}
}

func TestSanitizeQuery(t *testing.T) {
	got := sanitizeQuery("page=2&password=secret&keyword=abc")
	if strings.Contains(got, "secret") {
		t.Fatalf("query 中的密码未脱敏: %s", got)
	}
	if !strings.Contains(got, "page=2") || !strings.Contains(got, "keyword=abc") {
		t.Fatalf("query 中的非敏感条件应保留: %s", got)
	}
}

func TestReadQuerySnapshot(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/roles?page=2&token=abc", nil)
	got := readQuerySnapshot(req)
	if strings.Contains(got, "abc") {
		t.Fatalf("query 中的令牌未脱敏: %s", got)
	}
	if !strings.Contains(got, "page=2") {
		t.Fatalf("query 筛选条件应可回溯: %s", got)
	}
}

func TestResolveModule(t *testing.T) {
	cases := map[string]string{
		"/admin/admin-users/1":       "管理员管理",
		"/admin/departments":         "部门管理",
		"/admin/roles/1/departments": "角色管理",
		"/admin/permissions":         "权限管理",
		"/admin/profile":             "个人中心",
		"/admin/logs/operations":     "日志管理",
		"/admin/articles":            "文章管理",
		"/admin/trash/articles":      "回收站",
		"/admin/files":               "文件管理",
		"/admin/upload":              "文件管理",
		"/admin/dicts/1":             "字典管理",
		"/admin/configs":             "配置管理",
		"/admin/dashboard/stats":     "控制台",
		"/unknown/path":              "其他",
	}
	for path, want := range cases {
		if got := resolveModule(path); got != want {
			t.Fatalf("resolveModule(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestResolveAction(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPost, "/admin/admin-users/create", "新增"},
		{http.MethodPost, "/admin/articles/1/approve", "审核通过"},
		{http.MethodPost, "/admin/articles/batch-offline", "批量下线"},
		{http.MethodDelete, "/admin/trash/articles/1/purge", "彻底删除"},
		{http.MethodPost, "/admin/trash/articles/1/restore", "恢复"},
		{http.MethodPost, "/admin/admin-users/1/force-logout", "强制下线"},
		{http.MethodPut, "/admin/files/rename", "重命名"},
		{http.MethodGet, "/admin/articles", "查询"},
		{http.MethodPut, "/admin/roles/2", "修改"},
		{http.MethodDelete, "/admin/tags/3", "删除"},
	}
	for _, tc := range cases {
		if got := resolveAction(tc.method, tc.path); got != tc.want {
			t.Fatalf("resolveAction(%s %s) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

// 日志查询接口必须放行但不再产生新的操作日志，避免查询自放大。
func TestHandleSkipsLogQueries(t *testing.T) {
	m := &OperationLogMiddleware{}
	called := false
	h := m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/admin/logs/operations?page=1", nil))
	if !called {
		t.Fatal("日志查询请求应继续交给下游 handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("日志查询响应被改写，实际状态码 %d", rec.Code)
	}
}

func TestBusinessResult(t *testing.T) {
	rec := &responseRecorder{status: 200, body: bytes.NewBufferString(`{"code":4001,"msg":"bad"}`)}
	code, msg := rec.businessResult()
	if code != 4001 || msg != "bad" {
		t.Fatalf("businessResult 解析错误: %d %s", code, msg)
	}

	// 非 JSON 响应按 HTTP 状态码判定
	rec2 := &responseRecorder{status: 200, body: bytes.NewBufferString("not json")}
	if code2, _ := rec2.businessResult(); code2 != 0 {
		t.Fatalf("非 JSON 应判成功, 实际 %d", code2)
	}
	rec3 := &responseRecorder{status: 500, body: bytes.NewBufferString("not json")}
	if code3, _ := rec3.businessResult(); code3 != 500 {
		t.Fatalf("非 JSON 且 5xx 应返回状态码, 实际 %d", code3)
	}
}

func TestReadBodySnapshot_MultipartEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if _, err := w.CreateFormFile("file", "a.png"); err != nil {
		t.Fatal(err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if got := readBodySnapshot(req); got != "" {
		t.Fatalf("multipart 应返回空快照，实际: %s", got)
	}
}

func TestReadBodySnapshot_JSONMasked(t *testing.T) {
	body := `{"username":"bob","password":"p"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got := readBodySnapshot(req)
	if got == "" {
		t.Fatal("JSON 请求应返回快照")
	}
	if strings.Contains(got, `"p"`) {
		t.Fatal("快照中密码未脱敏")
	}
	// 原始 body 应被回填，下游仍可读取
	b, _ := io.ReadAll(req.Body)
	if !strings.Contains(string(b), `"username":"bob"`) {
		t.Fatal("请求体未回填，下游无法读取")
	}
}

func TestShouldSkipAudit(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodGet, "/admin/logs/operations", true},
		{http.MethodGet, "/admin/logs/logins", true},
		{http.MethodGet, "/admin/logs/stats", false},
		{http.MethodGet, "/admin/logs/operations/12", false},
		{http.MethodPost, "/admin/logs/export", false},
		{http.MethodGet, "/api/captcha", true},
		{http.MethodGet, "/admin/roles", false},
	}
	for _, tc := range cases {
		if got := shouldSkipAudit(tc.method, tc.path); got != tc.want {
			t.Fatalf("shouldSkipAudit(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

// 回归：快照截断不能把请求体一起截断，否则 >4KB 的 JSON 请求下游会拿到非法 JSON。
func TestReadBodySnapshot_LargeBodyIntact(t *testing.T) {
	body := `{"content":"` + strings.Repeat("x", maxLogBodyBytes*2) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got := readBodySnapshot(req)
	if got == "" {
		t.Fatal("大请求仍应产生参数快照")
	}
	if len(got) > maxLogBodyBytes {
		t.Fatalf("参数快照未截断: %d 字节", len(got))
	}
	b, _ := io.ReadAll(req.Body)
	if string(b) != body {
		t.Fatalf("请求体被截断，下游只能读到 %d / %d 字节", len(b), len(body))
	}
}

func TestUsernameFromSnapshot(t *testing.T) {
	if got := usernameFromSnapshot(`{"username":"alice","password":"******"}`); got != "alice" {
		t.Fatalf("应提取账号名 alice，实际 %q", got)
	}
	if got := usernameFromSnapshot(`{"email":"a@b.com"}`); got != "a@b.com" {
		t.Fatalf("应回退到 email，实际 %q", got)
	}
	if got := usernameFromSnapshot(`not-json`); got != "" {
		t.Fatalf("非法 JSON 应返回空，实际 %q", got)
	}
}
