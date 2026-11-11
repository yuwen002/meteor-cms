package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/alicebob/miniredis/v2"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/config"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/authsession"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

// sessionTestScanner 让 ent 查询恒为空结果，使测试不依赖真实 MySQL。
type sessionTestScanner struct{}

func (sessionTestScanner) Close() error                            { return nil }
func (sessionTestScanner) ColumnTypes() ([]*sql.ColumnType, error) { return nil, nil }
func (sessionTestScanner) Columns() ([]string, error)              { return []string{"id"}, nil }
func (sessionTestScanner) Err() error                              { return nil }
func (sessionTestScanner) Next() bool                              { return false }
func (sessionTestScanner) NextResultSet() bool                     { return false }
func (sessionTestScanner) Scan(...any) error                       { return nil }

type sessionTestResult struct{}

func (sessionTestResult) LastInsertId() (int64, error) { return 1, nil }
func (sessionTestResult) RowsAffected() (int64, error) { return 1, nil }

// sessionTestDriver 记录写入次数，用于断言吊销会话时确实补写了 token 黑名单。
type sessionTestDriver struct{ execs int }

func (d *sessionTestDriver) Exec(_ context.Context, _ string, _ any, v any) error {
	d.execs++
	if res, ok := v.(*sql.Result); ok {
		*res = sessionTestResult{}
	}
	return nil
}
func (d *sessionTestDriver) Query(_ context.Context, _ string, _ any, v any) error {
	if rows, ok := v.(*entsql.Rows); ok {
		rows.ColumnScanner = sessionTestScanner{}
	}
	return nil
}
func (d *sessionTestDriver) Tx(context.Context) (dialect.Tx, error) {
	return nil, errors.New("transaction not supported in tests")
}
func (d *sessionTestDriver) Close() error    { return nil }
func (d *sessionTestDriver) Dialect() string { return dialect.MySQL }

func newSessionTestCtx(t *testing.T) (*svc.ServiceContext, *miniredis.Miniredis, *sessionTestDriver) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb, err := redis.NewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType})
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	drv := &sessionTestDriver{}
	client := ent.NewClient(ent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	var cfg config.Config
	cfg.Auth.RefreshExpire = 604800
	return &svc.ServiceContext{Config: cfg, Redis: rdb, EntClient: client}, mr, drv
}

func sessionClaims(uid int64, sid string) map[string]interface{} {
	return map[string]interface{}{"user_id": uid, "username": "admin", "sid": sid}
}

func sessionRequest(method, path string, claims map[string]interface{}, vars map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if claims != nil {
		r = r.WithContext(utils.WithUserCtx(r.Context(), claims))
	}
	if vars != nil {
		r = pathvar.WithVars(r, vars)
	}
	return r
}

type sessionAPIResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func decodeSessionResp(t *testing.T, rec *httptest.ResponseRecorder) sessionAPIResp {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP 状态码 = %d, want 200", rec.Code)
	}
	var resp sessionAPIResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是 JSON: %q (%v)", rec.Body.String(), err)
	}
	return resp
}

func TestSessionListHandler(t *testing.T) {
	svcCtx, _, _ := newSessionTestCtx(t)
	store := authsession.New(svcCtx.Redis, time.Hour)
	ctx := context.Background()
	sidCurrent, sidOther := authsession.NewSID(), authsession.NewSID()
	for _, sid := range []string{sidCurrent, sidOther} {
		if err := store.Create(ctx, 1, sid, "10.0.0.1", "agent", "hash-"+sid); err != nil {
			t.Fatalf("登记会话失败: %v", err)
		}
	}
	// 其它用户的会话不能出现在当前用户的列表里。
	if err := store.Create(ctx, 2, authsession.NewSID(), "10.0.0.9", "agent", "hash-x"); err != nil {
		t.Fatalf("登记会话失败: %v", err)
	}

	rec := httptest.NewRecorder()
	sessionListHandler(svcCtx)(rec, sessionRequest(http.MethodGet, "/admin/sessions", sessionClaims(1, sidCurrent), nil))
	resp := decodeSessionResp(t, rec)
	if resp.Code != 0 {
		t.Fatalf("code = %d, msg = %q", resp.Code, resp.Msg)
	}
	var data types.SessionListResp
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析列表失败: %v, raw=%s", err, resp.Data)
	}
	if len(data.List) != 2 {
		t.Fatalf("列表应只含当前用户的 2 条会话, 实际 %d", len(data.List))
	}
	current := 0
	for _, item := range data.List {
		if item.IsCurrent {
			current++
			if item.SID != sidCurrent {
				t.Fatalf("is_current 标错了会话: %+v", item)
			}
		}
	}
	if current != 1 {
		t.Fatalf("应恰好有 1 条 is_current, 实际 %d", current)
	}
	if strings.Contains(rec.Body.String(), "access_hash") {
		t.Fatal("列表不能泄露 access token 哈希")
	}
}

func TestSessionListHandlerUnauthorizedAndNoRedis(t *testing.T) {
	svcCtx, _, _ := newSessionTestCtx(t)

	rec := httptest.NewRecorder()
	sessionListHandler(svcCtx)(rec, sessionRequest(http.MethodGet, "/admin/sessions", nil, nil))
	if got := decodeSessionResp(t, rec).Code; got != common.ErrUnauthorized {
		t.Fatalf("无用户上下文应返回 %d, 实际 %d", common.ErrUnauthorized, got)
	}

	svcCtx.Redis = nil
	rec = httptest.NewRecorder()
	sessionListHandler(svcCtx)(rec, sessionRequest(http.MethodGet, "/admin/sessions", sessionClaims(1, "sid"), nil))
	resp := decodeSessionResp(t, rec)
	if resp.Code != common.ErrInternalServer || !strings.Contains(resp.Msg, "Redis") {
		t.Fatalf("无 Redis 应给出明确错误, code=%d msg=%q", resp.Code, resp.Msg)
	}
}

func TestSessionRevokeHandler(t *testing.T) {
	svcCtx, mr, drv := newSessionTestCtx(t)
	store := authsession.New(svcCtx.Redis, time.Hour)
	ctx := context.Background()
	sid := authsession.NewSID()
	if err := store.Create(ctx, 1, sid, "10.0.0.1", "agent", "hash-to-blacklist"); err != nil {
		t.Fatalf("登记会话失败: %v", err)
	}

	rec := httptest.NewRecorder()
	req := sessionRequest(http.MethodDelete, "/admin/sessions/"+sid, sessionClaims(1, "another-sid"), map[string]string{"sid": sid})
	sessionRevokeHandler(svcCtx)(rec, req)
	if resp := decodeSessionResp(t, rec); resp.Code != 0 {
		t.Fatalf("吊销失败: code=%d msg=%q", resp.Code, resp.Msg)
	}
	if got, _ := mr.Get("auth:session:revoked:" + sid); got != "1" {
		t.Fatalf("Redis 吊销标记 = %q, want \"1\"", got)
	}
	if revoked, err := authsession.IsRevoked(svcCtx.Redis, sid); err != nil || !revoked {
		t.Fatalf("中间件应判定会话已吊销: revoked=%v err=%v", revoked, err)
	}
	list, err := store.List(ctx, 1)
	if err != nil || len(list) != 0 {
		t.Fatalf("吊销后列表应为空: %v err=%v", list, err)
	}
	if drv.execs == 0 {
		t.Fatal("吊销时应把会话当前 access token 写入黑名单")
	}
}

func TestSessionRevokeHandlerRejectsMissingAndForeignSession(t *testing.T) {
	svcCtx, _, drv := newSessionTestCtx(t)
	store := authsession.New(svcCtx.Redis, time.Hour)
	ctx := context.Background()
	foreign := authsession.NewSID()
	if err := store.Create(ctx, 2, foreign, "10.0.0.2", "agent", "hash"); err != nil {
		t.Fatalf("登记会话失败: %v", err)
	}

	for _, tc := range []struct {
		name  string
		vars  map[string]string
		path  string
		admin int64
	}{
		{"缺少 sid", map[string]string{}, "/admin/sessions/", 1},
		{"未知 sid", map[string]string{"sid": "unknown"}, "/admin/sessions/unknown", 1},
		{"其它用户的会话", map[string]string{"sid": foreign}, "/admin/sessions/" + foreign, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := sessionRequest(http.MethodDelete, tc.path, sessionClaims(tc.admin, "current"), tc.vars)
			sessionRevokeHandler(svcCtx)(rec, req)
			if got := decodeSessionResp(t, rec).Code; got != common.ErrInvalidParams {
				t.Fatalf("应返回 %d, 实际 %d", common.ErrInvalidParams, got)
			}
		})
	}
	if drv.execs != 0 {
		t.Fatalf("失败分支不应写入黑名单, execs=%d", drv.execs)
	}
}
