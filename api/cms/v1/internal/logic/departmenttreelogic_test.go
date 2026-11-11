package logic

import (
	"testing"

	"github.com/yuwen002/meteor-cms/ent"
)

func deptParentPtr(id int64) *int64 { return &id }

// TestBuildDepartmentTreeMultiLevel 覆盖原有的值拷贝缺陷：
// 父先于子、孙最后遍历时，三级及以上部门不能静默丢失。
func TestBuildDepartmentTreeMultiLevel(t *testing.T) {
	list := []*ent.Department{
		{ID: 1, Name: "root", Sort: 1, IsActive: true},
		{ID: 2, Name: "child", Sort: 1, IsActive: true, ParentID: deptParentPtr(1)},
		{ID: 3, Name: "grandchild", Sort: 1, IsActive: true, ParentID: deptParentPtr(2)},
		{ID: 4, Name: "great-grandchild", Sort: 1, IsActive: true, ParentID: deptParentPtr(3)},
	}

	tree := buildDepartmentTree(list)
	if len(tree) != 1 {
		t.Fatalf("应只有 1 个根节点, 实际 %d", len(tree))
	}
	node := tree[0]
	for _, want := range []string{"child", "grandchild", "great-grandchild"} {
		if len(node.Children) != 1 {
			t.Fatalf("层级 %s 丢失: %s 的 Children = %d", want, node.Name, len(node.Children))
		}
		node = &node.Children[0]
		if node.Name != want {
			t.Fatalf("层级名称 = %q, want %q", node.Name, want)
		}
	}
}

// TestBuildDepartmentTreeMultipleRootsAndOrphan 多根、父级缺失节点与空列表。
func TestBuildDepartmentTreeMultipleRootsAndOrphan(t *testing.T) {
	list := []*ent.Department{
		{ID: 10, Name: "root-a", Sort: 2, IsActive: true},
		{ID: 20, Name: "root-b", Sort: 1, IsActive: true},
		{ID: 30, Name: "child-of-a", Sort: 1, IsActive: true, ParentID: deptParentPtr(10)},
		{ID: 40, Name: "orphan", Sort: 1, IsActive: true, ParentID: deptParentPtr(999)},
	}

	tree := buildDepartmentTree(list)
	if len(tree) != 2 {
		t.Fatalf("应有 2 个根节点, 实际 %d", len(tree))
	}
	// 根节点顺序按输入顺序，不因 sort 或挂载顺序变化
	if tree[0].Name != "root-a" || tree[1].Name != "root-b" {
		t.Fatalf("根节点顺序不符: %q, %q", tree[0].Name, tree[1].Name)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Name != "child-of-a" {
		t.Fatalf("root-a 的子节点不符: %+v", tree[0].Children)
	}
	if len(tree[1].Children) != 0 {
		t.Fatalf("root-b 不应有子节点: %+v", tree[1].Children)
	}
	if got := buildDepartmentTree(nil); len(got) != 0 {
		t.Fatalf("空输入应返回空树, 实际 %d", len(got))
	}
}
