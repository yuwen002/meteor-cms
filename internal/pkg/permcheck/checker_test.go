package permcheck

import (
	"context"
	"testing"
)

func TestHasAndHasAny(t *testing.T) {
	perms := []string{"cms:article:list", "cms:article:review"}
	if !Has(perms, "cms:article:list") {
		t.Fatal("应命中已有权限")
	}
	if Has(perms, "cms:article:approve") {
		t.Fatal("不应命中未授权权限")
	}
	if !HasAny(perms, "cms:article:approve", "cms:article:review") {
		t.Fatal("HasAny 应命中任一候选")
	}
	if HasAny(perms, "cms:article:approve", "cms:article:reject") {
		t.Fatal("HasAny 不应凭空放行")
	}
}

func TestPermissionsContext(t *testing.T) {
	if _, ok := PermissionsFrom(context.Background()); ok {
		t.Fatal("未注入时不应存在权限快照")
	}
	want := []string{"cms:article:list"}
	ctx := WithPermissions(context.Background(), want)
	got, ok := PermissionsFrom(ctx)
	if !ok || len(got) != 1 || got[0] != want[0] {
		t.Fatalf("权限快照未按预期注入: %v ok=%v", got, ok)
	}
}

func TestContainsAll(t *testing.T) {
	for _, tc := range []struct {
		name      string
		own       []string
		requested []string
		want      bool
	}{
		{"子集", []string{"a", "b"}, []string{"a"}, true},
		{"全等", []string{"a", "b"}, []string{"a", "b"}, true},
		{"包含未授权项", []string{"a", "b"}, []string{"a", "c"}, false},
		{"空请求（清空权限）", []string{"a"}, nil, true},
		{"自身为空但有请求", nil, []string{"a"}, false},
		{"双方为空", nil, nil, true},
		{"重复请求项", []string{"a"}, []string{"a", "a"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ContainsAll(tc.own, tc.requested); got != tc.want {
				t.Fatalf("ContainsAll(%v, %v) = %v, want %v", tc.own, tc.requested, got, tc.want)
			}
		})
	}
}

func TestInvalidateChangesCacheKeyVersions(t *testing.T) {
	c := &Checker{userVersions: make(map[int64]int64)}
	g0, u0 := c.generation.Load(), c.userVersion(7)
	c.InvalidateUser(9)
	if c.userVersion(7) != u0 {
		t.Fatal("失效其他用户不应影响该用户版本")
	}
	c.InvalidateUser(7)
	if c.userVersion(7) != u0+1 {
		t.Fatal("单用户失效应递增该用户版本")
	}
	if c.generation.Load() != g0 {
		t.Fatal("单用户失效不应改变全局版本")
	}
	c.InvalidateAll()
	if c.generation.Load() != g0+1 {
		t.Fatal("全局失效应递增全局版本")
	}
}
