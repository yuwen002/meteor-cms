//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// rawRequest 用于导出文件这类不走统一 JSON 封装的接口。
func rawRequest(t *testing.T, method, path, token string, body []byte, contentType string) (int, http.Header, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
	return res.StatusCode, res.Header, raw
}

func e2eAdmin(t *testing.T) string {
	t.Helper()
	waitHealthy(t)
	return login(t, "admin", envOr("CMS_E2E_ADMIN_PASSWORD", "123456"))
}

// TestPublicSiteAndDicts 验证公开站点信息是白名单输出，字典只读且只返回启用项。
func TestPublicSiteAndDicts(t *testing.T) {
	waitHealthy(t)
	_, _, raw := rawRequest(t, http.MethodGet, "/api/site", "", nil, "")
	var resp apiResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("public site bad JSON: %q", string(raw))
	}
	wantCode(t, "public site", 0, resp)
	var site map[string]any
	if err := json.Unmarshal(resp.Data, &site); err != nil {
		t.Fatalf("public site data: %v", err)
	}
	if site["name"] == nil || site["name"] == "" {
		t.Fatalf("public site 缺少站名: %s", resp.Data)
	}
	// 默认页脚是 {year} 模板，公开接口必须输出展开后的年份；
	// 运营自定义页脚允许不含年份，但任何情况下都不应把占位符原样返回。
	footer, _ := site["footer"].(string)
	if strings.Contains(footer, "{year}") {
		t.Fatalf("公开站点页脚未展开年份模板: %q", footer)
	}
	if strings.Contains(footer, "Meteor CMS") && !strings.Contains(footer, time.Now().Format("2006")) {
		t.Fatalf("默认页脚应包含当前年份: %q", footer)
	}
	for _, key := range []string{"mail_smtp_password", "auth_secret", "password"} {
		if _, ok := site[key]; ok {
			t.Fatalf("公开站点信息泄露了敏感配置 %s", key)
		}
	}

	r := call(t, http.MethodGet, "/api/dicts/config_group", "", nil)
	wantCode(t, "public dict config_group", 0, r)
	var d struct {
		Code  string `json:"code"`
		Items []struct {
			Value string `json:"value"`
		} `json:"items"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		t.Fatalf("public dict data: %v", err)
	}
	if d.Code != "config_group" || len(d.Items) == 0 {
		t.Fatalf("公开字典内容异常: %s", r.Data)
	}

	r = call(t, http.MethodGet, "/api/dicts/not_exists_xyz", "", nil)
	wantCode(t, "public dict missing", 0, r)
	var empty struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(r.Data, &empty); err != nil || len(empty.Items) != 0 {
		t.Fatalf("不存在字典应返回空列表: %s", r.Data)
	}
}

// TestReadyzAndSchemaVersion 验证就绪探针会校验 schema 版本，
// 且响应里包含可追溯的构建版本信息。
func TestReadyzAndSchemaVersion(t *testing.T) {
	waitHealthy(t)
	status, _, raw := rawRequest(t, http.MethodGet, "/readyz", "", nil, "")
	if status != http.StatusOK {
		t.Fatalf("readyz HTTP %d: %q", status, string(raw))
	}
	var d struct {
		Ready    bool   `json:"ready"`
		Database string `json:"database"`
		Version  string `json:"version"`
		Schema   struct {
			Version         string `json:"version"`
			ExpectedVersion string `json:"expected_version"`
			OK              bool   `json:"ok"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("readyz bad JSON: %q", string(raw))
	}
	if !d.Ready || d.Database != "up" || !d.Schema.OK {
		t.Fatalf("迁移完成后实例应处于就绪状态: %s", string(raw))
	}
	if d.Schema.Version != d.Schema.ExpectedVersion || d.Schema.Version == "" {
		t.Fatalf("schema 版本不一致: %s", string(raw))
	}
	if !strings.Contains(d.Version, "schema=") {
		t.Fatalf("readyz 未返回构建版本: %s", string(raw))
	}
}

// TestAdminMonitor 验证运行监控接口能返回进程指标与依赖状态。
func TestAdminMonitor(t *testing.T) {
	admin := e2eAdmin(t)
	r := call(t, http.MethodGet, "/admin/monitor", admin, nil)
	wantCode(t, "admin monitor", 0, r)
	var m struct {
		Status        string `json:"status"`
		Database      string `json:"database"`
		GoVersion     string `json:"go_version"`
		Goroutines    int    `json:"goroutines"`
		RequestsTotal int64  `json:"requests_total"`
	}
	if err := json.Unmarshal(r.Data, &m); err != nil {
		t.Fatalf("monitor data: %v", err)
	}
	if m.Status != "ok" || m.Database != "up" || m.GoVersion == "" || m.Goroutines < 1 {
		t.Fatalf("运行监控指标异常: %s", r.Data)
	}
}

// TestFullTextSearch 验证 /api/search 能命中正文，且不会返回未发布的草稿。
func TestFullTextSearch(t *testing.T) {
	admin := e2eAdmin(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	marker := "zqmarker" + suffix
	slug := "e2e-fulltext-" + suffix
	catID := createCategory(t, admin, suffix)
	defer func() { _ = call(t, http.MethodDelete, fmt.Sprintf("/admin/categories/%d", catID), admin, nil) }()

	r := call(t, http.MethodPost, "/admin/articles", admin, map[string]any{
		"title":       "全文检索 " + suffix,
		"slug":        slug,
		"content":     "正文包含独特标记 " + marker,
		"category_id": catID,
	})
	wantCode(t, "create fulltext article", 0, r)
	id := respID(t, r)
	defer func() {
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/articles/%d", id), admin, nil)
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/trash/article/%d/purge", id), admin, nil)
	}()

	// 草稿不应被公开检索命中
	r = call(t, http.MethodGet, "/api/search?keyword="+marker, "", nil)
	wantCode(t, "search draft", 0, r)
	var before struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(r.Data, &before); err != nil || before.Total != 0 {
		t.Fatalf("草稿不应出现在公开检索中: %s", r.Data)
	}

	wantCode(t, "publish", 0,
		call(t, http.MethodPost, fmt.Sprintf("/admin/articles/%d/publish", id), admin, nil))

	r = call(t, http.MethodGet, "/api/search?keyword="+marker, "", nil)
	wantCode(t, "search published", 0, r)
	var after struct {
		Total   int64  `json:"total"`
		Keyword string `json:"keyword"`
		List    []struct {
			Slug string `json:"slug"`
		} `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &after); err != nil {
		t.Fatalf("search data: %v", err)
	}
	if after.Total != 1 || len(after.List) != 1 || after.List[0].Slug != slug || after.Keyword != marker {
		t.Fatalf("全文检索未命中正文标记: %s", r.Data)
	}
}

// TestContentBundleRoundTrip 验证导出的内容包可以幂等重新导入。
func TestContentBundleRoundTrip(t *testing.T) {
	admin := e2eAdmin(t)
	status, header, raw := rawRequest(t, http.MethodGet, "/admin/content/export", admin, nil, "")
	if status != http.StatusOK {
		t.Fatalf("导出内容包 HTTP %d: %q", status, string(raw))
	}
	if ct := header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("导出内容包 Content-Type 异常: %q", ct)
	}
	var bundle struct {
		Version  string            `json:"version"`
		Articles []json.RawMessage `json:"articles"`
	}
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("导出内容包不是合法 JSON: %v, body=%q", err, string(raw))
	}
	if bundle.Version == "" {
		t.Fatalf("导出内容包缺少版本: %q", string(raw))
	}

	// 原样导回：所有条目都已存在，应该全部跳过而不是报错或重复创建
	status, _, body := rawRequest(t, http.MethodPost, "/admin/content/import", admin, raw, "application/json")
	if status != http.StatusOK {
		t.Fatalf("导入内容包 HTTP %d: %q", status, string(body))
	}
	var resp apiResp
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("import bad JSON: %q", string(body))
	}
	wantCode(t, "import bundle", 0, resp)
	var result struct {
		ArticlesCreated int `json:"articles_created"`
		ArticlesSkipped int `json:"articles_skipped"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		t.Fatalf("import result: %v", err)
	}
	if result.ArticlesCreated != 0 || result.ArticlesSkipped != len(bundle.Articles) {
		t.Fatalf("幂等导入结果异常: %s, 导出文章数=%d", resp.Data, len(bundle.Articles))
	}
}

// TestSiteExtensions 验证友链/订阅公开接口与后台管理链路，并确认评论开关生效。
func TestSiteExtensions(t *testing.T) {
	admin := e2eAdmin(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// 评论默认关闭，公开写入应被拒绝
	r := call(t, http.MethodPost, "/api/comments", "", map[string]any{
		"slug": "not-exist", "author": "e2e", "content": "hello",
	})
	if r.Code == 0 {
		t.Fatal("评论功能未开启时不应允许提交")
	}

	// 友链后台新增 -> 公开可见 -> 后台删除
	name := "E2E Link " + suffix
	r = call(t, http.MethodPost, "/admin/links", admin, map[string]any{
		"name": name, "url": "https://example.com/" + suffix, "status": 1,
	})
	wantCode(t, "create link", 0, r)
	r = call(t, http.MethodGet, "/admin/links", admin, nil)
	wantCode(t, "list links", 0, r)
	var links struct {
		List []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &links); err != nil {
		t.Fatalf("links data: %v", err)
	}
	var linkID int64
	for _, l := range links.List {
		if l.Name == name {
			linkID = l.ID
		}
	}
	if linkID == 0 {
		t.Fatalf("新建友链未出现在列表: %s", r.Data)
	}
	defer func() { _ = call(t, http.MethodDelete, fmt.Sprintf("/admin/links/%d", linkID), admin, nil) }()

	r = call(t, http.MethodGet, "/api/links", "", nil)
	wantCode(t, "public links", 0, r)
	var publicLinks struct {
		List []struct {
			Name string `json:"name"`
		} `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &publicLinks); err != nil {
		t.Fatalf("public links data: %v", err)
	}
	found := false
	for _, l := range publicLinks.List {
		if l.Name == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("公开友链列表缺少新建项: %s", r.Data)
	}

	// 订阅幂等：连续两次提交同一个邮箱不应报错
	email := "e2e-" + suffix + "@example.com"
	for i := 0; i < 2; i++ {
		wantCode(t, "subscribe", 0,
			call(t, http.MethodPost, "/api/subscribe", "", map[string]any{"email": email}))
	}
	r = call(t, http.MethodGet, "/admin/subscribers?keyword="+email, admin, nil)
	wantCode(t, "list subscribers", 0, r)
	var subs struct {
		List []struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		} `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &subs); err != nil {
		t.Fatalf("subscribers data: %v", err)
	}
	if len(subs.List) != 1 || subs.List[0].Email != email {
		t.Fatalf("订阅未幂等写入: %s", r.Data)
	}
	defer func() {
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/subscribers/%d", subs.List[0].ID), admin, nil)
	}()
}
