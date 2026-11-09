package datascope

import (
	"github.com/yuwen002/meteor-cms/ent"
)

// BuildChildrenIndex 构建父部门到子部门列表的索引
func BuildChildrenIndex(departments []*ent.Department) map[int64][]int64 {
	index := make(map[int64][]int64, len(departments))
	for _, dept := range departments {
		if dept.ParentID == nil || *dept.ParentID == 0 {
			continue
		}
		index[*dept.ParentID] = append(index[*dept.ParentID], dept.ID)
	}
	return index
}

// CollectDescendantIDs 收集 roots 的全部子孙部门 ID（不含 roots 自身）。
//
// 采用广度优先遍历并对已入队节点去重，避免部门表存在脏数据形成环时死循环。
func CollectDescendantIDs(departments []*ent.Department, roots []int64) []int64 {
	if len(roots) == 0 {
		return nil
	}

	index := BuildChildrenIndex(departments)
	visited := make(map[int64]struct{}, len(departments))
	queue := make([]int64, 0, len(roots))
	result := make([]int64, 0, len(departments))

	for _, root := range roots {
		visited[root] = struct{}{}
		queue = append(queue, root)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, child := range index[current] {
			if _, ok := visited[child]; ok {
				continue
			}
			visited[child] = struct{}{}
			result = append(result, child)
			queue = append(queue, child)
		}
	}

	return result
}
