package logic

import (
	"context"
	"testing"
)

// TestGrantAllowed 固化「防越权提升」的判定边界：提权链的核心回归网。
func TestGrantAllowed(t *testing.T) {
	for _, tc := range []struct {
		name        string
		isSuper     bool
		hasSnapshot bool
		own         []string
		requested   []string
		want        bool
	}{
		{"超管可授予任意权限", true, false, nil, []string{"system:role:add"}, true},
		{"超管即使无快照也放行", true, false, nil, []string{"a", "b"}, true},
		{"无权限快照一律拒绝", false, false, []string{"a"}, []string{"a"}, false},
		{"授予自身权限子集", false, true, []string{"a", "b"}, []string{"a"}, true},
		{"授予自身权限全等", false, true, []string{"a", "b"}, []string{"a", "b"}, true},
		{"授予超出自身的权限（提权）", false, true, []string{"a"}, []string{"a", "b"}, false},
		{"自身无权限却尝试授权", false, true, nil, []string{"a"}, false},
		{"清空角色权限属收权操作", false, true, []string{"a"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := grantAllowed(tc.isSuper, tc.hasSnapshot, tc.own, tc.requested); got != tc.want {
				t.Fatalf("grantAllowed() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEnsurePermissionGrantAllowedEmptyList 空数组表示清空角色的全部权限，必须放行。
func TestEnsurePermissionGrantAllowedEmptyList(t *testing.T) {
	if err := ensurePermissionGrantAllowed(context.Background(), nil, nil); err != nil {
		t.Fatalf("清空权限不应报错: %v", err)
	}
	if err := ensurePermissionGrantAllowed(context.Background(), nil, []int64{}); err != nil {
		t.Fatalf("清空权限不应报错: %v", err)
	}
}
