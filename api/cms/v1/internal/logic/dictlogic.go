// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/sysdict"
	"github.com/yuwen002/meteor-cms/ent/sysdictitem"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type DictLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDictLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DictLogic {
	return &DictLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DictLogic) invalidate() {
	if l.svcCtx != nil && l.svcCtx.Dicts != nil {
		l.svcCtx.Dicts.Invalidate()
	}
}

// DictList 分页查询字典类型，支持按编码/名称模糊搜索、状态与分组过滤
func (l *DictLogic) DictList(req *types.DictListReq) (resp *types.DictListResp, err error) {
	query := l.svcCtx.EntClient.SysDict.Query()
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		query = query.Where(sysdict.Or(
			sysdict.CodeContainsFold(kw),
			sysdict.NameContainsFold(kw),
		))
	}
	if req.Status != nil {
		query = query.Where(sysdict.StatusEQ(*req.Status))
	}
	if g := strings.TrimSpace(req.Group); g != "" {
		query = query.Where(sysdict.DictGroupEQ(g))
	}

	total, err := query.Clone().Count(l.ctx)
	if err != nil {
		logx.Errorf("count dict failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	rows, err := query.
		Order(ent.Asc(sysdict.FieldSort), ent.Asc(sysdict.FieldID)).
		Limit(int(pageSize)).
		Offset(int((page - 1) * pageSize)).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list dict failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	list := make([]types.DictItem, 0, len(rows))
	for _, d := range rows {
		list = append(list, types.DictItem{
			ID: d.ID, Code: d.Code, Name: d.Name, Description: d.Description,
			Status: d.Status, Sort: d.Sort,
			DictGroup: d.DictGroup, IsSystem: d.IsSystem,
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
		})
	}
	return &types.DictListResp{List: list, Total: int64(total)}, nil
}

// DictCreate 新增字典类型，编码全局唯一；status 不传默认 1，显式传 0 表示创建即禁用。
func (l *DictLogic) DictCreate(req *types.DictCreateReq) error {
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		return common.NewBizError(common.ErrInvalidParams)
	}
	exists, err := l.svcCtx.EntClient.SysDict.
		Query().
		Where(sysdict.CodeEQ(code)).
		Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrDictCodeExists)
	}

	status := 1
	if req.Status != nil {
		status = *req.Status
	}
	group := strings.TrimSpace(req.DictGroup)
	if group == "" {
		group = "general"
	}
	if _, err := l.svcCtx.EntClient.SysDict.Create().
		SetCode(code).
		SetName(name).
		SetDescription(req.Description).
		SetStatus(status).
		SetSort(req.Sort).
		SetDictGroup(group).
		SetIsSystem(false).
		Save(l.ctx); err != nil {
		logx.Errorf("create dict failed: %v", err)
		return common.NewBizError(common.ErrDictCreateFail)
	}
	l.invalidate()
	return nil
}

// DictUpdate 更新字典类型；指针字段区分「未传」与「显式清空」。
func (l *DictLogic) DictUpdate(req *types.DictUpdateReq) error {
	d, err := l.svcCtx.EntClient.SysDict.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return common.NewBizError(common.ErrDictNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}

	update := d.Update()
	if name := strings.TrimSpace(req.Name); name != "" {
		update = update.SetName(name)
	}
	if req.Description != nil {
		update = update.SetDescription(*req.Description)
	}
	if req.Status != nil {
		update = update.SetStatus(*req.Status)
	}
	if req.Sort != nil {
		update = update.SetSort(*req.Sort)
	}
	if req.DictGroup != nil {
		update = update.SetDictGroup(strings.TrimSpace(*req.DictGroup))
	}
	if _, err := update.Save(l.ctx); err != nil {
		logx.Errorf("update dict failed: %v", err)
		return common.NewBizError(common.ErrDictUpdateFail)
	}
	l.invalidate()
	return nil
}

// DictDelete 在同一事务内级联删除字典项与字典类型；内置字典禁止删除。
func (l *DictLogic) DictDelete(req *types.DictDetailReq) error {
	d, err := l.svcCtx.EntClient.SysDict.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return common.NewBizError(common.ErrDictNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}
	if d.IsSystem {
		return common.NewBizError(common.ErrPermissionInUse)
	}

	txCtx := &l.ctx
	cancel := newTxContext(txCtx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(*txCtx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.SysDictItem.Delete().
		Where(sysdictitem.DictCodeEQ(d.Code)).
		Exec(*txCtx); err != nil {
		logx.Errorf("delete dict items failed: %v", err)
		return common.NewBizError(common.ErrDictDeleteFail)
	}
	if err := tx.SysDict.DeleteOneID(req.ID).Exec(*txCtx); err != nil {
		logx.Errorf("delete dict failed: %v", err)
		return common.NewBizError(common.ErrDictDeleteFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed = true
	l.invalidate()
	return nil
}
