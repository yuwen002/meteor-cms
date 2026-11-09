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

type DepartmentCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDepartmentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DepartmentCreateLogic {
	return &DepartmentCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DepartmentCreateLogic) DepartmentCreate(req *types.DepartmentCreateReq) (resp *types.CommonResp, err error) {
	var parentID *int64
	level := 1 // 默认为一级部门

	// 检查部门名称是否已存在
	exists, err := l.svcCtx.EntClient.Department.
		Query().
		Where(department.NameEQ(req.Name)).
		Exist(l.ctx)
	if err != nil {
		l.Logger.Errorf("检查部门名称是否存在失败: %v", err)
		return nil, common.NewBizError(common.ErrDepartmentCreateFail)
	}
	if exists {
		return nil, common.NewBizError(common.ErrDepartmentNameExists)
	}

	// 处理父级部门
	if req.ParentId > 0 {
		// 检查父部门是否存在
		exists, err := l.svcCtx.EntClient.Department.
			Query().
			Where(department.IDEQ(req.ParentId)).
			Exist(l.ctx)
		if err != nil {
			l.Logger.Errorf("查询父部门失败: %v", err)
			return nil, common.NewBizError(common.ErrDepartmentParentNotExist)
		}
		if !exists {
			return nil, common.NewBizError(common.ErrDepartmentParentNotExist)
		}

		// 获取父部门的层级并加1
		parent, err := l.svcCtx.EntClient.Department.Get(l.ctx, req.ParentId)
		if err != nil {
			l.Logger.Errorf("获取父部门信息失败: %v", err)
			return nil, common.NewBizError(common.ErrDepartmentParentNotExist)
		}

		// 检查父部门是否启用
		if !parent.IsActive {
			return nil, common.NewBizError(common.ErrDepartmentParentNotActive)
		}

		level = parent.Level + 1
		parentID = &req.ParentId
	}

	// 创建部门
	create := l.svcCtx.EntClient.Department.Create().
		SetName(req.Name).
		SetLevel(level).
		SetSort(req.Sort).
		SetIsActive(req.IsActive)

	// 设置父部门ID（如果有）
	if parentID != nil {
		create = create.SetParentID(*parentID)
	}

	// 设置负责人ID（如果有）
	if req.LeaderId > 0 {
		// 设置部门负责人
		create = create.SetLeaderID(req.LeaderId)
	}

	// 创建部门
	dept, err := create.Save(l.ctx)
	if err != nil {
		logx.Errorf("创建部门失败: %v", err)
		return nil, common.NewBizError(common.ErrDepartmentCreateFail)
	}

	// 如果指定了管理员，更新管理员部门关系
	if req.LeaderId > 0 {
		_, err = l.svcCtx.EntClient.AdminUser.UpdateOneID(req.LeaderId).
			SetDepartmentID(dept.ID).
			Save(l.ctx)
		if err != nil {
			// 记录错误但不中断流程，因为部门已经创建成功
			logx.Errorf("更新管理员部门关系失败: %v", err)
		}
	}

	return &types.CommonResp{
		ID:      dept.ID,
		Message: "创建成功",
	}, nil
}
