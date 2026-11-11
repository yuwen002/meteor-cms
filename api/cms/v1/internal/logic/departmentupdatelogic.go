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

	// 4️⃣ 执行更新；根级统一用 NULL 表示（自引用外键下写 0 会失败或产生两种根表示）
	update := l.svcCtx.EntClient.Department.
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
		return nil, err
	}

	return &types.CommonResp{
		ID:      0,
		Message: "更新成功",
	}, nil
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
