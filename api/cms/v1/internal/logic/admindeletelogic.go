// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuserrole"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteLogic {
	return &AdminDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteLogic) AdminDelete(req *types.DeleteAdminReq) (resp *types.CommonResp, err error) {
	// 检查用户是否存在
	target, err := l.svcCtx.EntClient.AdminUser.Get(l.ctx, req.Id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrAdminUserNotFound)
		}
		l.Logger.Errorf("查询管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	// 不能删除自己
	if currentUserID, ok := utils.CurrentUserID(l.ctx); ok && currentUserID == req.Id {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "不能删除自己")
	}

	// 超级管理员不能被删除
	if target.IsSuper {
		return nil, common.NewBizErrorWithMsg(common.ErrPermissionDenied, "超级管理员不可删除")
	}

	// 软删除用户与清理角色关联必须同事务：admin_user_roles 没有软删除字段，
	// 残留行不会被拦截器过滤，会让角色删除接口一直误判「角色正在被使用」。
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		l.Logger.Errorf("开启删除管理员事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.AdminUserRole.
		Delete().
		Where(adminuserrole.UserID(req.Id)).
		Exec(l.ctx); err != nil {
		l.Logger.Errorf("清理管理员角色关联失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if _, err := tx.AdminUser.UpdateOneID(req.Id).
		SetDeletedAt(time.Now()).
		Save(l.ctx); err != nil {
		l.Logger.Errorf("删除管理员失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if err := tx.Commit(); err != nil {
		l.Logger.Errorf("提交删除管理员事务失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	committed = true

	// 关联已清理，同时让该用户的权限快照失效
	l.svcCtx.PermissionChecker.InvalidateUser(req.Id)

	// 删除后吊销该用户所有会话（全局踢人）
	if _, berr := tokenversion.Bump(l.ctx, l.svcCtx.Redis, req.Id); berr != nil {
		l.Logger.Errorf("吊销用户旧会话失败: user_id=%d err=%v", req.Id, berr)
	}

	return &types.CommonResp{
		ID:      req.Id,
		Message: "删除成功",
	}, nil
}
