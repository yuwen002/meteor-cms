package datascope

import (
	"testing"

	"github.com/yuwen002/meteor-cms/ent"
)

func pi64(i int64) *int64 { return &i }

func TestFilter_AllowsUser(t *testing.T) {
	// 1. 非受限（超管/无范围限制）→ 全部放行
	unrestricted := &Filter{Restricted: false}
	if !unrestricted.AllowsUser(5, pi64(99)) {
		t.Error("非受限范围应放行任意用户")
	}

	// 2. 仅本人
	selfOnly := &Filter{Restricted: true, OnlySelf: true, UserID: 1}
	if !selfOnly.AllowsUser(1, nil) {
		t.Error("本人应被放行")
	}
	if selfOnly.AllowsUser(2, nil) {
		t.Error("仅本人模式下他人应被拒绝")
	}

	// 3. 部门范围
	dept := &Filter{Restricted: true, DeptIDs: []int64{10, 20}}
	if !dept.AllowsUser(1, pi64(20)) {
		t.Error("本部门用户应被放行")
	}
	if dept.AllowsUser(1, pi64(30)) {
		t.Error("非本部门用户应被拒绝")
	}
	if dept.AllowsUser(1, nil) {
		t.Error("部门为空的记录应被拒绝(fail-closed)")
	}

	// 4. 受限但无任何范围（解析兜底）→ 全部拒绝
	empty := &Filter{Restricted: true}
	if empty.AllowsUser(1, pi64(10)) {
		t.Error("受限但无范围时应全部拒绝(fail-closed)")
	}
}

// TestValidScope 写入口用它拦住 0/负数/越界值，避免脏 data_scope 让整个 Resolve 失败。
func TestValidScope(t *testing.T) {
	for scope := ScopeAll; scope <= ScopeCustom; scope++ {
		if !ValidScope(scope) {
			t.Fatalf("合法 data_scope %d 被拒绝", scope)
		}
	}
	for _, scope := range []int{-1, 0, ScopeCustom + 1, 99} {
		if ValidScope(scope) {
			t.Fatalf("非法 data_scope %d 被放行", scope)
		}
	}
}

// TestActiveDepartments 停用部门不能再通过旧的 dept_id / admin_role_depts 引用继续放权。
func TestActiveDepartments(t *testing.T) {
	got := ActiveDepartments([]*ent.Department{
		{ID: 1, IsActive: true},
		{ID: 2, IsActive: false},
		nil,
		{ID: 3, IsActive: true},
	})
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("只应保留启用部门: %+v", got)
	}
	if empty := ActiveDepartments(nil); len(empty) != 0 {
		t.Fatalf("空输入应返回空列表, 实际 %d", len(empty))
	}
}
