package datascope

import (
	"testing"

	"github.com/yuwen002/meteor-cms/ent"
)

// dept 快速构造部门实体（仅使用 tree 需要的字段）
func dept(id int64, parent int64) *ent.Department {
	var p *int64
	if parent > 0 {
		v := parent
		p = &v
	}
	return &ent.Department{ID: id, ParentID: p}
}

func TestCollectDescendantIDs(t *testing.T) {
	departments := []*ent.Department{
		dept(1, 0),
		dept(2, 1),
		dept(3, 1),
		dept(4, 2),
		dept(5, 4),
		dept(9, 0), // 另一棵子树
	}

	got := CollectDescendantIDs(departments, []int64{1})
	want := map[int64]bool{2: true, 3: true, 4: true, 5: true}
	if len(got) != len(want) {
		t.Fatalf("子孙部门数量不符: got %v, want %v", got, want)
	}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("不应包含部门 %d", id)
		}
	}

	// 叶子节点没有子孙
	if got := CollectDescendantIDs(departments, []int64{5}); len(got) != 0 {
		t.Fatalf("叶子节点应无子孙, got %v", got)
	}

	// 不跨树
	if got := CollectDescendantIDs(departments, []int64{9}); len(got) != 0 {
		t.Fatalf("孤立节点应无子孙, got %v", got)
	}
}

// TestCollectDescendantIDsWithCycle 脏数据形成环时不能死循环
func TestCollectDescendantIDsWithCycle(t *testing.T) {
	departments := []*ent.Department{
		dept(1, 3), // 环：1 -> 2 -> 3 -> 1
		dept(2, 1),
		dept(3, 2),
	}

	got := CollectDescendantIDs(departments, []int64{1})
	if len(got) != 2 {
		t.Fatalf("成环时应只返回去重后的子孙节点, got %v", got)
	}
}

func TestCollectDescendantIDsEmptyRoots(t *testing.T) {
	if got := CollectDescendantIDs(nil, nil); got != nil {
		t.Fatalf("无根节点应返回 nil, got %v", got)
	}
}

func TestBuildChildrenIndex(t *testing.T) {
	departments := []*ent.Department{dept(1, 0), dept(2, 1), dept(3, 1), dept(4, 2)}
	index := BuildChildrenIndex(departments)

	if len(index[1]) != 2 {
		t.Fatalf("部门1应有2个子部门, got %v", index[1])
	}
	if len(index[2]) != 1 {
		t.Fatalf("部门2应有1个子部门, got %v", index[2])
	}
	// parent_id 为 0 或 null 的节点是根节点，不应出现在 value 侧的最小 key 中
	if _, ok := index[0]; ok {
		t.Fatal("根节点不应作为子节点索引出现")
	}
}
