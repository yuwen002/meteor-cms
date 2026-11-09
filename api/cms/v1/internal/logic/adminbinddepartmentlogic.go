// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/ent/department"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminBindDepartmentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBindDepartmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBindDepartmentLogic {
	return &AdminBindDepartmentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminBindDepartmentLogic) AdminBindDepartment(req *types.AdminBindDeptReq) (resp *types.CommonResp, err error) {
	// 1️⃣ 校验管理员是否存在
	user, err := l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.IDEQ(req.Id)).
		Only(l.ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		logx.Errorf("查询管理员信息失败，ID: %d, 错误: %v", req.Id, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 2️⃣ 校验部门是否存在
	exist, err := l.svcCtx.EntClient.Department.
		Query().
		Where(department.IDEQ(req.DeptId)).
		Exist(l.ctx)

	if err != nil {
		l.Errorf("查询部门信息失败, 部门ID: %d, 错误: %v", req.DeptId, err)
		return nil, err
	}

	if !exist {
		return nil, common.NewBizError(common.ErrDepartmentNotFound)
	}

	// 3️⃣ 绑定部门
	err = user.
		Update().
		SetDeptID(req.DeptId).
		Exec(l.ctx)

	if err != nil {
		l.Errorf("更新管理员部门信息失败, 管理员ID: %d, 部门ID: %d, 错误: %v", req.Id, req.DeptId, err)
		return nil, err
	}

	return &types.CommonResp{
		ID:      0,
		Message: "绑定成功",
	}, nil
}
