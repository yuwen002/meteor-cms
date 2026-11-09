//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	baseURL = envOr("CMS_E2E_BASE_URL", "http://127.0.0.1:8888")
	client  = &http.Client{Timeout: 30 * time.Second}
)

const e2ePassword = "Author@123456"

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type apiResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func call(t *testing.T, method, path, token string, body any) apiResp {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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
	var out apiResp
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s %s: bad JSON %q: %v", method, path, string(raw), err)
	}
	return out
}

// wantCode 用于前置步骤，失败立即终止。
func wantCode(t *testing.T, what string, want int, got apiResp) {
	t.Helper()
	if got.Code != want {
		t.Fatalf("%s: want code=%d, got code=%d msg=%q data=%s", what, want, got.Code, got.Msg, got.Data)
	}
}

// expectCode 用于断言，失败继续执行，便于一次看到全部越权点。
func expectCode(t *testing.T, what string, want int, got apiResp) {
	t.Helper()
	if got.Code != want {
		t.Errorf("%s: want code=%d, got code=%d msg=%q data=%s", what, want, got.Code, got.Msg, got.Data)
	}
}

func respID(t *testing.T, r apiResp) int64 {
	t.Helper()
	var d struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil || d.ID <= 0 {
		t.Fatalf("response has no id: %s (%v)", r.Data, err)
	}
	return d.ID
}

func idsInList(t *testing.T, r apiResp) map[int64]bool {
	t.Helper()
	var d struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		t.Fatalf("parse list: %v (%s)", err, r.Data)
	}
	ids := make(map[int64]bool, len(d.List))
	for _, item := range d.List {
		ids[item.ID] = true
	}
	return ids
}

func categoryArticleCount(t *testing.T, r apiResp, categoryID int64) int {
	t.Helper()
	var d struct {
		List []struct {
			ID           int64 `json:"id"`
			ArticleCount int   `json:"article_count"`
		} `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		t.Fatalf("parse category list: %v (%s)", err, r.Data)
	}
	for _, c := range d.List {
		if c.ID == categoryID {
			return c.ArticleCount
		}
	}
	t.Fatalf("分类 %d 不在列表响应中: %s", categoryID, r.Data)
	return -1
}

func login(t *testing.T, username, password string) string {
	t.Helper()
	r := call(t, http.MethodPost, "/admin/login", "", map[string]string{
		"username": username,
		"password": password,
	})
	wantCode(t, "login "+username, 0, r)
	var d struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil || d.AccessToken == "" {
		t.Fatalf("login %s: missing access_token: %s", username, r.Data)
	}
	return d.AccessToken
}

func waitHealthy(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		res, err := client.Get(baseURL + "/healthz")
		if err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("API 未在 3 分钟内就绪")
}

// contentPermissions 返回测试角色应分配的权限 ID（文章模块 + 回收站模块）。
// 作者角色保留 cms:article:review（仅查看自己审核记录），排除 approve/reject；
// 审核员角色额外持有 approve/reject，因此可跨作者查看；escalate 单独返回这两个权限，
// 用于验证「授权/撤销后立即生效」。
func contentPermissions(t *testing.T, admin string) (author, reviewer, escalate []int64) {
	t.Helper()
	for page := 1; page <= 10; page++ {
		r := call(t, http.MethodGet,
			fmt.Sprintf("/admin/permissions?page=%d&page_size=100", page), admin, nil)
		wantCode(t, "list permissions", 0, r)
		var d struct {
			Total int64 `json:"total"`
			List  []struct {
				ID         int64  `json:"id"`
				Permission string `json:"permission"`
			} `json:"list"`
		}
		if err := json.Unmarshal(r.Data, &d); err != nil {
			t.Fatalf("parse permissions: %v", err)
		}
		for _, p := range d.List {
			isArticleOrTrash := strings.HasPrefix(p.Permission, "cms:article:") ||
				strings.HasPrefix(p.Permission, "cms:trash:")
			// 分类列表用于校验文章计数口径，作者角色只需读权限即可。
			if !isArticleOrTrash && p.Permission != "cms:category:list" {
				continue
			}
			reviewer = append(reviewer, p.ID)
			if p.Permission == "cms:article:approve" || p.Permission == "cms:article:reject" {
				escalate = append(escalate, p.ID)
			} else {
				author = append(author, p.ID)
			}
		}
		if int64(page)*100 >= d.Total {
			break
		}
	}
	if len(author) == 0 || len(escalate) == 0 {
		t.Fatalf("文章权限不完整: author=%d escalate=%d", len(author), len(escalate))
	}
	return author, reviewer, escalate
}

func createRole(t *testing.T, admin, suffix string, perms []int64) int64 {
	t.Helper()
	r := call(t, http.MethodPost, "/admin/roles", admin, map[string]any{
		"name":       "E2E " + suffix,
		"code":       "e2e_" + suffix,
		"desc":       "data scope e2e",
		"data_scope": 1,
		"is_active":  true,
		"sort":       0,
	})
	wantCode(t, "create role "+suffix, 0, r)
	id := respID(t, r)
	r = call(t, http.MethodPut, fmt.Sprintf("/admin/roles/%d/permissions", id), admin,
		map[string]any{"permission_ids": perms})
	wantCode(t, "assign role permissions", 0, r)
	return id
}

func createUser(t *testing.T, admin, username, phone string, roleID int64) int64 {
	t.Helper()
	r := call(t, http.MethodPost, "/admin/admin-users/create", admin, map[string]any{
		"username":  username,
		"password":  e2ePassword,
		"nickname":  username,
		"email":     username + "@example.com",
		"phone":     phone,
		"is_active": true,
		"is_super":  false,
	})
	wantCode(t, "create user "+username, 0, r)
	id := respID(t, r)
	r = call(t, http.MethodPut, fmt.Sprintf("/admin/admin-users/%d/roles", id), admin,
		map[string]any{"role_ids": []int64{roleID}})
	wantCode(t, "assign user role", 0, r)
	return id
}

func createCategory(t *testing.T, admin, suffix string) int64 {
	t.Helper()
	r := call(t, http.MethodPost, "/admin/categories", admin, map[string]any{
		"name":      "E2E Cat " + suffix,
		"slug":      "e2e-cat-" + suffix,
		"is_active": true,
	})
	wantCode(t, "create category", 0, r)
	return respID(t, r)
}

func createArticle(t *testing.T, token, suffix, who string, categoryID int64) int64 {
	t.Helper()
	r := call(t, http.MethodPost, "/admin/articles", token, map[string]any{
		"title":       "E2E " + who,
		"slug":        "e2e-" + who + "-" + suffix,
		"content":     "# " + who,
		"category_id": categoryID,
	})
	wantCode(t, "create article "+who, 0, r)
	return respID(t, r)
}

func cleanup(t *testing.T, admin string, articleIDs, userIDs, roleIDs []int64, categoryID int64) {
	t.Helper()
	for _, id := range articleIDs {
		if id <= 0 {
			continue
		}
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/articles/%d", id), admin, nil)
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/trash/article/%d/purge", id), admin, nil)
	}
	// 删除用户会同步清理 admin_user_roles，之后角色才能被删除。
	for _, id := range userIDs {
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/admin-users/%d", id), admin, nil)
	}
	for _, id := range roleIDs {
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/roles/%d", id), admin, nil)
	}
	// 分类下已无文章，此时才能删除。
	if categoryID > 0 {
		_ = call(t, http.MethodDelete, fmt.Sprintf("/admin/categories/%d", categoryID), admin, nil)
	}
}

func TestArticleRowLevelScope(t *testing.T) {
	waitHealthy(t)
	admin := login(t, "admin", envOr("CMS_E2E_ADMIN_PASSWORD", "123456"))
	authorPerms, reviewerPerms, escalatePerms := contentPermissions(t, admin)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	roleAuthor := createRole(t, admin, "author_"+suffix, authorPerms)
	roleReviewer := createRole(t, admin, "reviewer_"+suffix, reviewerPerms)
	userA := createUser(t, admin, "e2e_a_"+suffix, "13800000001", roleAuthor)
	userB := createUser(t, admin, "e2e_b_"+suffix, "13800000002", roleAuthor)
	userC := createUser(t, admin, "e2e_c_"+suffix, "13800000003", roleReviewer)
	articleIDs := make([]int64, 0, 2)
	categoryID := int64(0)
	defer func() {
		cleanup(t, admin, articleIDs, []int64{userA, userB, userC}, []int64{roleAuthor, roleReviewer}, categoryID)
	}()

	tokenA := login(t, "e2e_a_"+suffix, e2ePassword)
	tokenB := login(t, "e2e_b_"+suffix, e2ePassword)
	tokenC := login(t, "e2e_c_"+suffix, e2ePassword)

	categoryID = createCategory(t, admin, suffix)
	articleA := createArticle(t, tokenA, suffix, "a", categoryID)
	articleB := createArticle(t, tokenB, suffix, "b", categoryID)
	articleIDs = append(articleIDs, articleA, articleB)

	// 分类文章数必须与文章列表的可见范围一致：A 只见自己的 1 篇，超管见 2 篇。
	catA := call(t, http.MethodGet, "/admin/categories", tokenA, nil)
	wantCode(t, "A list categories", 0, catA)
	if got := categoryArticleCount(t, catA, categoryID); got != 1 {
		t.Errorf("A 的分类文章数应为 1，实际 %d", got)
	}
	catAdmin := call(t, http.MethodGet, "/admin/categories", admin, nil)
	wantCode(t, "admin list categories", 0, catAdmin)
	if got := categoryArticleCount(t, catAdmin, categoryID); got != 2 {
		t.Errorf("超管的分类文章数应为 2，实际 %d", got)
	}

	listA := call(t, http.MethodGet, "/admin/articles?page=1&page_size=100", tokenA, nil)
	wantCode(t, "A list articles", 0, listA)
	idsA := idsInList(t, listA)
	if !idsA[articleA] {
		t.Errorf("A 应能看到自己的文章 %d", articleA)
	}
	if idsA[articleB] {
		t.Errorf("A 不应在列表看到 B 的文章 %d", articleB)
	}

	expectCode(t, "A read B detail", 6037,
		call(t, http.MethodGet, fmt.Sprintf("/admin/articles/%d", articleB), tokenA, nil))
	expectCode(t, "A read B revisions", 6037,
		call(t, http.MethodGet, fmt.Sprintf("/admin/articles/%d/revisions", articleB), tokenA, nil))
	// 作者角色持有 cms:article:review，仍不得跨作者读取审核记录。
	expectCode(t, "A read B reviews", 6037,
		call(t, http.MethodGet, fmt.Sprintf("/admin/articles/%d/reviews", articleB), tokenA, nil))

	// 权限缓存主动失效：授予 approve 后立即跨作者可见，撤销后立即恢复隔离。
	// 若仍依赖 60s TTL，这两条断言会失败。
	granted := append(append([]int64{}, authorPerms...), escalatePerms...)
	wantCode(t, "grant approve to author role", 0,
		call(t, http.MethodPut, fmt.Sprintf("/admin/roles/%d/permissions", roleAuthor), admin,
			map[string]any{"permission_ids": granted}))
	expectCode(t, "B sees A right after grant", 0,
		call(t, http.MethodGet, fmt.Sprintf("/admin/articles/%d", articleA), tokenB, nil))
	wantCode(t, "revoke approve from author role", 0,
		call(t, http.MethodPut, fmt.Sprintf("/admin/roles/%d/permissions", roleAuthor), admin,
			map[string]any{"permission_ids": authorPerms}))
	expectCode(t, "B isolated right after revoke", 6037,
		call(t, http.MethodGet, fmt.Sprintf("/admin/articles/%d", articleA), tokenB, nil))

	wantCode(t, "B delete own article", 0,
		call(t, http.MethodDelete, fmt.Sprintf("/admin/articles/%d", articleB), tokenB, nil))
	trashA := call(t, http.MethodGet, "/admin/trash?type=article&page=1&page_size=100", tokenA, nil)
	wantCode(t, "A trash list", 0, trashA)
	if idsInList(t, trashA)[articleB] {
		t.Errorf("A 不应在回收站看到 B 的文章 %d", articleB)
	}
	expectCode(t, "A purge B article", 6037,
		call(t, http.MethodDelete, fmt.Sprintf("/admin/trash/article/%d/purge", articleB), tokenA, nil))
	// 越权 purge 被拒后，文章必须仍在回收站。
	trashAdmin := call(t, http.MethodGet, "/admin/trash?type=article&page=1&page_size=100", admin, nil)
	wantCode(t, "admin trash list", 0, trashAdmin)
	if !idsInList(t, trashAdmin)[articleB] {
		t.Errorf("越权 purge 不应删除 B 的文章 %d", articleB)
	}

	listC := call(t, http.MethodGet, "/admin/articles?page=1&page_size=100", tokenC, nil)
	wantCode(t, "reviewer list articles", 0, listC)
	if !idsInList(t, listC)[articleA] {
		t.Errorf("持有 approve/reject 的审核角色应能看到他人文章 %d", articleA)
	}
	listAdmin := call(t, http.MethodGet, "/admin/articles?page=1&page_size=100", admin, nil)
	wantCode(t, "admin list articles", 0, listAdmin)
	if !idsInList(t, listAdmin)[articleA] {
		t.Errorf("超管应能看到全部文章 %d", articleA)
	}
}
