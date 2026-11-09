package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	"github.com/alicebob/miniredis/v2"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/config"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const jwtTestSecret = "jwt-middleware-test-secret"

// emptyColumnScanner 模拟「查询不到任何行」的结果集。
type emptyColumnScanner struct{}

func (emptyColumnScanner) Close() error                            { return nil }
func (emptyColumnScanner) ColumnTypes() ([]*sql.ColumnType, error) { return nil, nil }
func (emptyColumnScanner) Columns() ([]string, error)              { return []string{"id"}, nil }
func (emptyColumnScanner) Err() error                              { return nil }
func (emptyColumnScanner) Next() bool                              { return false }
func (emptyColumnScanner) NextResultSet() bool                     { return false }
func (emptyColumnScanner) Scan(...any) error                       { return nil }

// emptyDBDriver 让 ent 的 token 黑名单查询恒为空，使中间件测试不依赖真实 MySQL。
// 仅用于测试：所有查询都返回空结果，不执行任何 SQL。
type emptyDBDriver struct{}

func (emptyDBDriver) Exec(context.Context, string, any, any) error { return nil }
func (emptyDBDriver) Query(_ context.Context, _ string, _ any, v any) error {
	if rows, ok := v.(*dialectsql.Rows); ok {
		rows.ColumnScanner = emptyColumnScanner{}
	}
	return nil
}
func (emptyDBDriver) Tx(context.Context) (dialect.Tx, error) {
	return nil, errors.New("transaction not supported in tests")
}
func (emptyDBDriver) Close() error    { return nil }
func (emptyDBDriver) Dialect() string { return dialect.MySQL }

type jwtTestEnv struct {
	mini *miniredis.Miniredis
	rdb  *redis.Redis
	do   func(string) *httptest.ResponseRecorder
}

func newJwtTestEnv(t *testing.T, rdb *redis.Redis, mini *miniredis.Miniredis) *jwtTestEnv {
	t.Helper()
	var cfg config.Config
	cfg.Auth.AccessSecret = jwtTestSecret
	db := ent.NewClient(ent.Driver(emptyDBDriver{}))
	t.Cleanup(func() { _ = db.Close() })

	handler := NewJwtMiddleware(&cfg, db, rdb)(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("PASSED"))
	})
	do := func(token string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		handler.ServeHTTP(rec, req)
		return rec
	}
	return &jwtTestEnv{mini: mini, rdb: rdb, do: do}
}

func newJwtTestEnvWithRedis(t *testing.T) *jwtTestEnv {
	t.Helper()
	mini := miniredis.RunT(t)
	rdb, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: redis.NodeType})
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	return newJwtTestEnv(t, rdb, mini)
}

func signTestToken(t *testing.T, userID, tv int64, withTV bool) string {
	t.Helper()
	payload := map[string]interface{}{
		"user_id":  userID,
		"username": "admin",
	}
	if withTV {
		payload["tv"] = tv
	}
	tok, err := utils.GenerateToken(jwtTestSecret, 3600, payload)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return tok
}

func assertPassed(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(rec.Body.String(), "PASSED") {
		t.Fatalf("期望放行, 实际 code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func assertRevoked(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是 JSON: %q (%v)", rec.Body.String(), err)
	}
	if resp.Code != common.ErrTokenRevoked {
		t.Fatalf("期望 code=%d(已吊销), 实际 %d, body=%q", common.ErrTokenRevoked, resp.Code, rec.Body.String())
	}
}

// TestJwtMiddleware_RealKick 把手工「真实踢人」验证固化成回归测试：
// 踢人前旧 token 可用 → Bump 真写 Redis auth:tv:1=1 → 同一旧 token 返回 2012。
func TestJwtMiddleware_RealKick(t *testing.T) {
	env := newJwtTestEnvWithRedis(t)
	tok := signTestToken(t, 1, 0, true)

	assertPassed(t, env.do(tok))

	v, err := tokenversion.Bump(context.Background(), env.rdb, 1)
	if err != nil || v != 1 {
		t.Fatalf("Bump = (%d, %v), want (1, nil)", v, err)
	}
	if got, _ := env.mini.Get("auth:tv:1"); got != "1" {
		t.Fatalf("Redis auth:tv:1 = %q, want \"1\"", got)
	}

	assertRevoked(t, env.do(tok))
}

// TestJwtMiddleware_NewTokenAfterKick 踢人后按新版本签发的 token 应正常放行（不是一律封杀）。
func TestJwtMiddleware_NewTokenAfterKick(t *testing.T) {
	env := newJwtTestEnvWithRedis(t)
	if _, err := tokenversion.Bump(context.Background(), env.rdb, 1); err != nil {
		t.Fatal(err)
	}
	assertPassed(t, env.do(signTestToken(t, 1, 1, true)))
}

// TestJwtMiddleware_LegacyTokenWithoutTV 缺失 tv 的旧令牌在版本 >0 后同样必须被吊销。
func TestJwtMiddleware_LegacyTokenWithoutTV(t *testing.T) {
	env := newJwtTestEnvWithRedis(t)
	if _, err := tokenversion.Bump(context.Background(), env.rdb, 1); err != nil {
		t.Fatal(err)
	}
	assertRevoked(t, env.do(signTestToken(t, 1, 0, false)))
}

// TestJwtMiddleware_OtherUserUnaffected 踢 A 不误伤 B（版本 key 按 uid 隔离）。
func TestJwtMiddleware_OtherUserUnaffected(t *testing.T) {
	env := newJwtTestEnvWithRedis(t)
	if _, err := tokenversion.Bump(context.Background(), env.rdb, 1); err != nil {
		t.Fatal(err)
	}
	assertPassed(t, env.do(signTestToken(t, 2, 0, true)))
}

// TestJwtMiddleware_NoRedisDegrade 未配置 Redis 时不校验版本，降级放行。
func TestJwtMiddleware_NoRedisDegrade(t *testing.T) {
	env := newJwtTestEnv(t, nil, nil)
	assertPassed(t, env.do(signTestToken(t, 1, 0, true)))
}

// TestJwtMiddleware_RedisErrorDegrade Redis 中版本值非法时降级放行，避免误杀。
func TestJwtMiddleware_RedisErrorDegrade(t *testing.T) {
	env := newJwtTestEnvWithRedis(t)
	if err := env.mini.Set("auth:tv:1", "not-a-number"); err != nil {
		t.Fatal(err)
	}
	assertPassed(t, env.do(signTestToken(t, 1, 0, true)))
}
