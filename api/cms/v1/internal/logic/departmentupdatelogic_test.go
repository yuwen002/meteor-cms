package logic

import "testing"

// TestCreatesCycle 覆盖「把部门挂到自己的后代下」的检测，以及库里已有脏环的防御。
func TestCreatesCycle(t *testing.T) {
	// 层级: 1(root) → 2 → 3；4 是独立根；5↔6 是库里已存在的脏环
	parentOf := map[int64]*int64{
		1: nil,
		2: deptParentPtr(1),
		3: deptParentPtr(2),
		4: nil,
		5: deptParentPtr(6),
		6: deptParentPtr(5),
	}

	for _, tc := range []struct {
		name                string
		deptID, newParentID int64
		want                bool
	}{
		{"挂到自己", 2, 2, true},
		{"挂到直接子级", 1, 2, true},
		{"挂到深层后代", 1, 3, true},
		{"合法跨分支移动", 4, 1, false},
		{"移到根级", 2, 0, false},
		{"父链本身成环的脏数据", 4, 5, true},
		{"未知父级不成环", 4, 999, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := createsCycle(tc.deptID, tc.newParentID, parentOf); got != tc.want {
				t.Fatalf("createsCycle(%d, %d) = %v, want %v", tc.deptID, tc.newParentID, got, tc.want)
			}
		})
	}
}

// TestComputeLevels 根级为 1，跨父移动后子树层级整体跟着变化；
// 已有脏环也必须能终止计算。
func TestComputeLevels(t *testing.T) {
	// 移动前: 1(root) → 2 → 3；4 是独立根
	parentOf := map[int64]*int64{
		1: nil,
		2: deptParentPtr(1),
		3: deptParentPtr(2),
		4: nil,
	}
	before := computeLevels(parentOf)
	for id, want := range map[int64]int{1: 1, 2: 2, 3: 3, 4: 1} {
		if before[id] != want {
			t.Fatalf("移动前部门 %d 层级 = %d, want %d", id, before[id], want)
		}
	}

	// 把 1 挂到 4 下：1/2/3 依次为 2/3/4
	moved := map[int64]*int64{
		1: deptParentPtr(4),
		2: deptParentPtr(1),
		3: deptParentPtr(2),
		4: nil,
	}
	after := computeLevels(moved)
	for id, want := range map[int64]int{1: 2, 2: 3, 3: 4, 4: 1} {
		if after[id] != want {
			t.Fatalf("移动后部门 %d 层级 = %d, want %d", id, after[id], want)
		}
	}

	cycle := computeLevels(map[int64]*int64{
		5: deptParentPtr(6),
		6: deptParentPtr(5),
	})
	if len(cycle) != 2 {
		t.Fatalf("脏环应终止计算并给出结果: %v", cycle)
	}
}

// TestDeactivationBlocked 停用只在「启用 → 停用」且仍有启用子部门时拦截；
// 部门被管理员 / 角色引用时仍允许停用（停用即收权），重新启用也不受影响。
func TestDeactivationBlocked(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		currentActive, nextActive, hasActiveChild bool
		want                                      bool
	}{
		{"启用子部门未处理时禁止停用", true, false, true, true},
		{"无启用子部门可停用", true, false, false, false},
		{"原本停用不受子部门影响", false, false, true, false},
		{"保持启用不受子部门影响", true, true, true, false},
		{"重新启用不受子部门影响", false, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := deactivationBlocked(tc.currentActive, tc.nextActive, tc.hasActiveChild)
			if got != tc.want {
				t.Fatalf("deactivationBlocked() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDescendantsOf 重算范围必须恰好是被移动部门及其后代，且脏环不死循环。
func TestDescendantsOf(t *testing.T) {
	parentOf := map[int64]*int64{
		1: nil,
		2: deptParentPtr(1),
		3: deptParentPtr(2),
		4: deptParentPtr(1),
		5: nil,
	}
	got := descendantsOf(parentOf, 1)
	for id, want := range map[int64]bool{1: true, 2: true, 3: true, 4: true, 5: false} {
		if got[id] != want {
			t.Fatalf("descendantsOf(1) 包含 %d = %v, want %v", id, got[id], want)
		}
	}
	if cycle := descendantsOf(map[int64]*int64{6: deptParentPtr(7), 7: deptParentPtr(6)}, 6); len(cycle) != 2 {
		t.Fatalf("脏环应终止 BFS: %v", cycle)
	}
}
