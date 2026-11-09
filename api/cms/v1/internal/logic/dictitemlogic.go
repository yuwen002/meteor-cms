// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/sysdictitem"
	"github.com/yuwen002/meteor-cms/internal/common"

	"github.com/zeromicro/go-zero/core/logx"
)

type DictItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDictItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DictItemLogic {
	return &DictItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DictItemLogic) invalidate() {
	if l.svcCtx != nil && l.svcCtx.Dicts != nil {
		l.svcCtx.Dicts.Invalidate()
	}
}

// DictItemList 查询指定字典下的字典项，按 sort、id 升序
func (l *DictItemLogic) DictItemList(req *types.DictItemListReq) (resp *types.DictItemListResp, err error) {
	dictCode := strings.TrimSpace(req.DictCode)
	if dictCode == "" {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}

	query := l.svcCtx.EntClient.SysDictItem.Query().
		Where(sysdictitem.DictCodeEQ(dictCode))
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		query = query.Where(sysdictitem.Or(
			sysdictitem.LabelContainsFold(kw),
			sysdictitem.ValueContainsFold(kw),
		))
	}
	if req.Status != nil {
		query = query.Where(sysdictitem.StatusEQ(*req.Status))
	}

	rows, err := query.
		Order(ent.Asc(sysdictitem.FieldSort), ent.Asc(sysdictitem.FieldID)).
		All(l.ctx)
	if err != nil {
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	list := make([]types.DictDataItem, 0, len(rows))
	for _, it := range rows {
		list = append(list, types.DictDataItem{
			ID: it.ID, DictCode: it.DictCode, Label: it.Label, Value: it.Value,
			Status: it.Status, Sort: it.Sort,
			ParentID: it.ParentID, IsDefault: it.IsDefault, Remark: it.Remark,
		})
	}
	return &types.DictItemListResp{List: list}, nil
}

// DictItemCreate 新增字典项，同一字典下 value 不允许重复；status 显式传 0 表示创建即禁用。
func (l *DictItemLogic) DictItemCreate(req *types.DictItemCreateReq) error {
	dictCode := strings.TrimSpace(req.DictCode)
	label := strings.TrimSpace(req.Label)
	value := strings.TrimSpace(req.Value)
	if dictCode == "" || label == "" || value == "" {
		return common.NewBizError(common.ErrInvalidParams)
	}

	exists, err := l.svcCtx.EntClient.SysDictItem.
		Query().
		Where(
			sysdictitem.DictCodeEQ(dictCode),
			sysdictitem.ValueEQ(value),
		).
		Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrDictItemExists)
	}
	if req.ParentID > 0 {
		parentExists, err := l.svcCtx.EntClient.SysDictItem.Query().
			Where(sysdictitem.IDEQ(req.ParentID), sysdictitem.DictCodeEQ(dictCode)).
			Exist(l.ctx)
		if err != nil {
			return common.NewBizError(common.ErrInternalServer)
		}
		if !parentExists {
			return common.NewBizError(common.ErrInvalidParams)
		}
	}

	status := 1
	if req.Status != nil {
		status = *req.Status
	}
	create := l.svcCtx.EntClient.SysDictItem.Create().
		SetDictCode(dictCode).
		SetLabel(label).
		SetValue(value).
		SetStatus(status).
		SetSort(req.Sort).
		SetParentID(req.ParentID).
		SetIsDefault(req.IsDefault).
		SetRemark(req.Remark)
	if _, err := create.Save(l.ctx); err != nil {
		logx.Errorf("create dict item failed: %v", err)
		return common.NewBizError(common.ErrDictCreateFail)
	}
	l.invalidate()
	return nil
}

// DictItemUpdate 更新字典项；指针字段区分「未传」与「显式清空」。
func (l *DictItemLogic) DictItemUpdate(req *types.DictItemUpdateReq) error {
	it, err := l.svcCtx.EntClient.SysDictItem.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return common.NewBizError(common.ErrDictItemNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}

	update := it.Update()
	if label := strings.TrimSpace(req.Label); label != "" {
		update = update.SetLabel(label)
	}
	if value := strings.TrimSpace(req.Value); value != "" {
		update = update.SetValue(value)
	}
	if req.Status != nil {
		update = update.SetStatus(*req.Status)
	}
	if req.Sort != nil {
		update = update.SetSort(*req.Sort)
	}
	if req.ParentID != nil {
		if *req.ParentID == req.ID {
			return common.NewBizError(common.ErrInvalidParams)
		}
		update = update.SetParentID(*req.ParentID)
	}
	if req.IsDefault != nil {
		update = update.SetIsDefault(*req.IsDefault)
	}
	if req.Remark != nil {
		update = update.SetRemark(*req.Remark)
	}
	if _, err := update.Save(l.ctx); err != nil {
		return common.NewBizError(common.ErrDictUpdateFail)
	}
	l.invalidate()
	return nil
}

// DictItemDelete 删除字典项；存在子项时拒绝删除，避免产生悬空层级。
func (l *DictItemLogic) DictItemDelete(req *types.DictItemDetailReq) error {
	it, err := l.svcCtx.EntClient.SysDictItem.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return common.NewBizError(common.ErrDictItemNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}
	children, err := l.svcCtx.EntClient.SysDictItem.Query().
		Where(sysdictitem.ParentIDEQ(req.ID)).
		Count(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if children > 0 {
		return common.NewBizError(common.ErrPermissionInUse)
	}
	if err := l.svcCtx.EntClient.SysDictItem.DeleteOneID(it.ID).Exec(l.ctx); err != nil {
		return common.NewBizError(common.ErrDictDeleteFail)
	}
	l.invalidate()
	return nil
}
