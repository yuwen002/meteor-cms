// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/department"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type DepartmentDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDepartmentDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DepartmentDeleteLogic {
	return &DepartmentDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DepartmentDeleteLogic) DepartmentDelete(req *types.DepartmentDeleteReq) (resp *types.CommonResp, err error) {
	// 1️⃣ 校验是否存在
	exist, err := l.svcCtx.EntClient.Department.
		Query().
		Where(department.IDEQ(req.Id)).
		Exist(l.ctx)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, common.NewBizError(common.ErrDepartmentNotFound)
	}

	// 2️⃣ 校验是否有子部门
	hasChild, err := l.svcCtx.EntClient.Department.
		Query().
		Where(department.ParentIDEQ(req.Id)).
		Exist(l.ctx)
	if err != nil {
		return nil, err
	}
	if hasChild {
		return nil, common.NewBizError(common.ErrDepartmentHasChildren)
	}

	// 3️⃣ 软删除（触发 SoftDeleteMixin）
	err = l.svcCtx.EntClient.Department.
		DeleteOneID(req.Id).
		Exec(l.ctx)

	if err != nil {
		return nil, err
	}

	return &types.CommonResp{
		ID:      req.Id,
		Message: "删除成功",
	}, nil
}
