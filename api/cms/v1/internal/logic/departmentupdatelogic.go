// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/department"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type DepartmentUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDepartmentUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DepartmentUpdateLogic {
	return &DepartmentUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DepartmentUpdateLogic) DepartmentUpdate(req *types.DepartmentUpdateReq) (resp *types.CommonResp, err error) {
	// 校验部门是否存在
	dept, err := l.svcCtx.EntClient.Department.
		Query().
		Where(department.IDEQ(req.Id)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrDepartmentNotFound)
		}
		return nil, err
	}

	// 不能把自己设为自己的父级
	if req.ParentId == dept.ID {
		return nil, common.NewBizError(common.ErrDepartmentSetSelfAsParent)
	}

	// 一次读取全表，同时校验父级存在/启用与「不能挂到自己的后代下」。
	// 成环会让整段子树没有根节点、从树中消失，数据范围的可见集合也会失真。
	all, err := l.svcCtx.EntClient.Department.Query().All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询部门层级失败: %v", err)
		return nil, common.NewBizError(common.ErrDepartmentUpdateFail)
	}
	parentOf := make(map[int64]*int64, len(all))
	parentExists, parentActive := false, false
	for _, d := range all {
		parentOf[d.ID] = d.ParentID
		if d.ID == req.ParentId {
			parentExists, parentActive = true, d.IsActive
		}
	}
	if req.ParentId > 0 {
		if !parentExists {
			return nil, common.NewBizError(common.ErrDepartmentParentNotExist)
		}
		if !parentActive {
			return nil, common.NewBizError(common.ErrDepartmentParentNotActive)
		}
		if createsCycle(req.Id, req.ParentId, parentOf) {
			return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "不能将部门挂到自己的子部门下")
		}
	}

	// 停用本身是收权手段（旧引用不再参与范围解析），但若还有启用的子部门，
	// 整条子树会沿停用父级被截断、在数据范围中整体消失且没有任何提示。
	// 与「新增 / 改父级时父级必须启用」保持对称：先处理子部门再停用父级。
	hasActiveChild := false
	if dept.IsActive && !req.IsActive {
		hasActiveChild, err = l.svcCtx.EntClient.Department.
			Query().
			Where(department.ParentIDEQ(req.Id), department.IsActiveEQ(true)).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("查询启用的子部门失败: %v", err)
			return nil, common.NewBizError(common.ErrDepartmentUpdateFail)
		}
	}
	if deactivationBlocked(dept.IsActive, req.IsActive, hasActiveChild) {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "该部门下仍有启用的子部门，请先调整或停用子部门")
	}

	// 父级变化时 level 必须连同子树一起重算，否则层级展示与统计会长期失真。
	var currentParent int64
	if dept.ParentID != nil {
		currentParent = *dept.ParentID
	}
	parentChanged := currentParent != req.ParentId

	// 如果部门名称有变化，检查新名称是否已存在
	if req.Name != dept.Name {
		exists, err := l.svcCtx.EntClient.Department.
			Query().
			Where(
				department.And(
					department.NameEQ(req.Name),
					department.IDNEQ(req.Id), // 排除当前部门
				),
			).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("检查部门名称是否存在失败: %v", err)
			return nil, common.NewBizError(common.ErrDepartmentUpdateFail)
		}
		if exists {
			return nil, common.NewBizError(common.ErrDepartmentNameExists)
		}
	}

	// 4️⃣ 事务内执行更新；根级统一用 NULL 表示（自引用外键下写 0 会失败或产生两种根表示）
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		l.Logger.Errorf("开启事务失败: %v", err)
		return nil, common.NewBizError(common.ErrDepartmentUpdateFail)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				l.Logger.Errorf("事务回滚失败: %v", rbErr)
			}
		}
	}()

	update := tx.Department.
		UpdateOneID(req.Id).
		SetName(req.Name).
		SetSort(req.Sort).
		SetIsActive(req.IsActive)
	if req.ParentId > 0 {
		update.SetParentID(req.ParentId)
	} else {
		update.ClearParentID()
	}
	if req.LeaderId > 0 {
		update.SetLeaderID(req.LeaderId)
	} else {
		update.ClearLeaderID()
	}

	if err = update.Exec(l.ctx); err != nil {
		l.Logger.Errorf("更新部门失败: %v", err)
		return nil, common.NewBizError(common.ErrDepartmentUpdateFail)
	}

	if parentChanged {
		parentOfNew := make(map[int64]*int64, len(parentOf))
		for id, p := range parentOf {
			parentOfNew[id] = p
		}
		if req.ParentId > 0 {
			pid := req.ParentId
			parentOfNew[req.Id] = &pid
		} else {
			parentOfNew[req.Id] = nil
		}

		levels := computeLevels(parentOfNew)
		affected := descendantsOf(parentOfNew, req.Id)
		for _, d := range all {
			if !affected[d.ID] || levels[d.ID] == d.Level {
				continue
			}
			if err = tx.Department.UpdateOneID(d.ID).SetLevel(levels[d.ID]).Exec(l.ctx); err != nil {
				l.Logger.Errorf("重算部门层级失败，ID: %d, 错误: %v", d.ID, err)
				return nil, common.NewBizError(common.ErrDepartmentUpdateFail)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		l.Logger.Errorf("提交事务失败: %v", err)
		return nil, common.NewBizError(common.ErrDepartmentUpdateFail)
	}

	return &types.CommonResp{
		ID:      0,
		Message: "更新成功",
	}, nil
}

// deactivationBlocked 判定「启用 → 停用」是否需要拦截。
// 仅拦截仍挂有启用子部门的情况；部门被管理员 / 角色引用时允许停用，
// 因为停用本身就是「旧引用不再放权」的收权手段。
func deactivationBlocked(currentActive, nextActive, hasActiveChild bool) bool {
	return currentActive && !nextActive && hasActiveChild
}

// createsCycle 判断把 deptID 挂到 newParentID 下是否会形成环。
// parentOf 是「部门 ID → 父部门 ID」的快照；库里已存在的脏环也一并视为不可用。
func createsCycle(deptID, newParentID int64, parentOf map[int64]*int64) bool {
	if deptID == newParentID {
		return true
	}
	seen := make(map[int64]struct{}, len(parentOf))
	cur := newParentID
	for cur > 0 {
		if cur == deptID {
			return true
		}
		if _, ok := seen[cur]; ok {
			return true
		}
		seen[cur] = struct{}{}
		parent, ok := parentOf[cur]
		if !ok || parent == nil || *parent == 0 {
			return false
		}
		cur = *parent
	}
	return false
}

// computeLevels 按 parentOf 快照计算每个部门的层级，根级为 1。
// 遇到环或父级不在快照中时停止上溯，保证脏数据不会造成死循环。
func computeLevels(parentOf map[int64]*int64) map[int64]int {
	levels := make(map[int64]int, len(parentOf))
	for id := range parentOf {
		depth, cur := 1, id
		seen := make(map[int64]bool, len(parentOf))
		for !seen[cur] {
			seen[cur] = true
			p := parentOf[cur]
			if p == nil || *p == 0 {
				break
			}
			cur = *p
			depth++
		}
		levels[id] = depth
	}
	return levels
}

// descendantsOf 返回 root 及其全部后代 ID，用于把 level 重算限定在被移动的子树内；
// out 去重同时保证库里已有的脏环不会让 BFS 无限扩张。
func descendantsOf(parentOf map[int64]*int64, root int64) map[int64]bool {
	out := map[int64]bool{root: true}
	queue := []int64{root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for id, p := range parentOf {
			if p != nil && *p == cur && !out[id] {
				out[id] = true
				queue = append(queue, id)
			}
		}
	}
	return out
}
