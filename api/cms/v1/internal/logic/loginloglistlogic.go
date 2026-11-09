// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/loginlog"
	"github.com/yuwen002/meteor-cms/ent/predicate"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogListLogic {
	return &LoginLogListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogListLogic) LoginLogList(req *types.LoginLogListReq) (resp *types.LoginLogListResp, err error) {
	offset, pageSize := utils.NormalizePage(req.Page, req.PageSize, 20)
	page := offset/pageSize + 1

	query := l.svcCtx.EntClient.LoginLog.Query()

	conds := make([]predicate.LoginLog, 0, 6)
	// 数据权限：超管或持有 system:log:view_all 的审计员可看全量，其余人只能看自己；
	// user_id 仅对全量可见者生效，避免越权检索他人日志。
	uid, scoped, err := logScopeUserID(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		l.Logger.Errorf("判定日志可见范围失败: %v", err)
		return nil, err
	}
	if scoped {
		conds = append(conds, loginlog.UserIDEQ(uid))
	} else if req.UserID > 0 {
		conds = append(conds, loginlog.UserIDEQ(req.UserID))
	}
	if req.Username != "" {
		conds = append(conds, loginlog.UsernameContainsFold(req.Username))
	}
	// Status 为指针：0（成功）与「未筛选」必须区分开
	if req.Status != nil {
		conds = append(conds, loginlog.StatusEQ(*req.Status))
	}
	if start, ok := parseLogTime(req.StartTime); ok {
		conds = append(conds, loginlog.LoginAtGTE(start))
	}
	if end, ok := parseLogEndTime(req.EndTime); ok {
		conds = append(conds, loginlog.LoginAtLTE(end))
	}
	if len(conds) > 0 {
		query = query.Where(loginlog.And(conds...))
	}

	total, err := query.Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询登录日志总数失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	list, err := query.
		Order(ent.Desc(loginlog.FieldLoginAt)).
		Offset(offset).
		Limit(pageSize).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询登录日志失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	items := make([]types.LoginLogItem, 0, len(list))
	for _, item := range list {
		items = append(items, loginLogItem(item))
	}

	return &types.LoginLogListResp{
		Total:    int64(total),
		Page:     page,
		PageSize: pageSize,
		List:     items,
	}, nil
}
