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

type DepartmentDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDepartmentDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DepartmentDetailLogic {
	return &DepartmentDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DepartmentDetailLogic) DepartmentDetail(req *types.DepartmentDetailReq) (resp *types.DepartmentDetailResp, err error) {
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

	resp = &types.DepartmentDetailResp{
		Id:       dept.ID,
		Name:     dept.Name,
		Sort:     dept.Sort,
		IsActive: dept.IsActive,
		LeaderId: 0,
		ParentId: 0,
	}

	if dept.LeaderID != nil {
		resp.LeaderId = *dept.LeaderID
	}

	if dept.ParentID != nil {
		resp.ParentId = *dept.ParentID
	}

	return resp, nil
}
