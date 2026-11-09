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

	// 校验父级是否存在（如果 parent_id != 0）
	if req.ParentId != 0 {
		exist, err := l.svcCtx.EntClient.Department.
			Query().
			Where(department.IDEQ(req.ParentId)).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("查询父部门失败: %v", err)
			return nil, common.NewBizError(common.ErrDepartmentParentNotExist)
		}
		if !exist {
			return nil, common.NewBizError(common.ErrDepartmentParentNotExist)
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

	// 4️⃣ 执行更新
	err = l.svcCtx.EntClient.Department.
		UpdateOneID(req.Id).
		SetName(req.Name).
		SetParentID(req.ParentId).
		SetSort(req.Sort).
		SetIsActive(req.IsActive).
		SetNillableLeaderID(&req.LeaderId).
		Exec(l.ctx)

	if err != nil {
		return nil, err
	}

	return &types.CommonResp{
		ID:      0,
		Message: "更新成功",
	}, nil
}
