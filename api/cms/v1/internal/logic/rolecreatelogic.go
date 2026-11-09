// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleCreateLogic {
	return &RoleCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleCreateLogic) RoleCreate(req *types.RoleCreateReq) (resp *types.CommonResp, err error) {
	// 1. 检查角色编码是否已存在
	exists, err := l.svcCtx.EntClient.AdminRole.
		Query().
		Where(adminrole.Code(req.Code)).
		Exist(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询角色失败: %v", err)
		logx.Errorf("查询角色失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return nil, common.NewBizError(common.ErrRoleCodeExists)
	}

	// 2. 创建角色
	now := time.Now()
	role, err := l.svcCtx.EntClient.AdminRole.
		Create().
		SetName(req.Name).
		SetCode(req.Code).
		SetDesc(req.Desc).
		SetDataScope(req.DataScope).
		SetIsActive(req.IsActive).
		SetSort(req.Sort).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(l.ctx)
	if err != nil {
		l.Logger.Errorf("创建角色失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 3. 返回成功响应
	return &types.CommonResp{
		ID:      role.ID,
		Message: "创建角色成功",
	}, nil
}
