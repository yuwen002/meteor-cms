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
