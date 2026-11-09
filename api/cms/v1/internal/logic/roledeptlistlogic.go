// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminroledept"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleDeptListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleDeptListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDeptListLogic {
	return &RoleDeptListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleDeptListLogic) RoleDeptList(req *types.RoleDetailReq) (resp *types.RoleDeptResp, err error) {
	// 1. 角色必须存在
	exists, err := l.svcCtx.EntClient.AdminRole.
		Query().
		Where(adminrole.ID(req.ID)).
		Exist(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询角色失败，ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if !exists {
		return nil, common.NewBizError(common.ErrRoleNotFound)
	}

	// 2. 查询该角色已绑定的部门
	roleDepts, err := l.svcCtx.EntClient.AdminRoleDept.
		Query().
		Where(adminroledept.RoleID(req.ID)).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询角色部门失败，角色ID: %d, 错误: %v", req.ID, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	deptIDs := make([]int64, 0, len(roleDepts))
	for _, rd := range roleDepts {
		deptIDs = append(deptIDs, rd.DeptID)
	}

	return &types.RoleDeptResp{DeptIDs: deptIDs}, nil
}
