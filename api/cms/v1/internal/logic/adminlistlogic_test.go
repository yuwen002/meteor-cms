package logic

import (
	"testing"

	"github.com/yuwen002/meteor-cms/internal/pkg/datascope"
)

// TestDeptFilterAllowed 固化「按部门筛选」的防越权边界：
// 用户传的 dept_id 必须仍启用，且落在自己数据可见范围内。
func TestDeptFilterAllowed(t *testing.T) {
	super := &datascope.Filter{Restricted: false, UserID: 1}
	limited := &datascope.Filter{Restricted: true, UserID: 2, DeptIDs: []int64{10, 11}}
	empty := &datascope.Filter{Restricted: true, UserID: 3}

	for _, tc := range []struct {
		name       string
		scope      *datascope.Filter
		deptActive bool
		deptID     int64
		want       bool
	}{
		{"超管可筛选任意启用部门", super, true, 99, true},
		{"受限用户筛选可见部门", limited, true, 11, true},
		{"受限用户筛选不可见部门", limited, true, 99, false},
		{"可见范围为空", empty, true, 10, false},
		{"已停用/已删除部门", super, false, 10, false},
		{"非正部门ID", super, true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := deptFilterAllowed(tc.scope, tc.deptActive, tc.deptID); got != tc.want {
				t.Fatalf("deptFilterAllowed() = %v, want %v", got, tc.want)
			}
		})
	}
}
