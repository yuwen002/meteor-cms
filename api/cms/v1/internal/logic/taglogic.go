package logic

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/tag"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/core/logx"
)

type TagLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TagLogic {
	return &TagLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// TagList 标签列表，附带已关联（未删除）文章数
func (l *TagLogic) TagList(req *types.TagListReq) (*types.TagListResp, error) {
	query := l.svcCtx.EntClient.Tag.Query()
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		query = query.Where(tag.Or(
			tag.NameContainsFold(kw),
			tag.SlugContainsFold(kw),
		))
	}
	if req.IsActive != nil {
		query = query.Where(tag.IsActiveEQ(*req.IsActive))
	}
	rows, err := query.Order(ent.Asc(tag.FieldSort), ent.Asc(tag.FieldID)).All(l.ctx)
	if err != nil {
		logx.Errorf("list tags failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	counts, err := tagArticleCounts(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		return nil, err
	}
	list := make([]types.TagItem, 0, len(rows))
	for _, tg := range rows {
		list = append(list, types.TagItem{
			ID:           tg.ID,
			Name:         tg.Name,
			Slug:         tg.Slug,
			Description:  tg.Description,
			Sort:         tg.Sort,
			IsActive:     tg.IsActive,
			ArticleCount: int(counts[tg.ID]),
			CreatedAt:    tg.CreatedAt.Format(time.RFC3339),
		})
	}
	return &types.TagListResp{List: list}, nil
}

// TagCreate 新建标签
func (l *TagLogic) TagCreate(req *types.TagCreateReq) (*types.CommonResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "标签名称不能为空")
	}
	rawSlug := strings.TrimSpace(req.Slug)
	if rawSlug != "" && !IsValidSlug(rawSlug) {
		return nil, common.NewBizError(common.ErrArticleInvalidSlug)
	}
	if err := l.ensureTagNameFree(name, 0); err != nil {
		return nil, err
	}
	slug := GenerateSlug(rawSlug, name, time.Now())
	if err := l.ensureTagSlugFree(slug, 0); err != nil {
		return nil, err
	}

	create := l.svcCtx.EntClient.Tag.Create().
		SetName(name).
		SetSlug(slug).
		SetDescription(strings.TrimSpace(req.Description)).
		SetSort(req.Sort).
		SetIsActive(true)
	if req.IsActive != nil {
		create = create.SetIsActive(*req.IsActive)
	}
	tg, err := create.Save(l.ctx)
	if err != nil {
		logx.Errorf("create tag failed: %v", err)
		return nil, common.NewBizError(common.ErrTagCreateFail)
	}
	return &types.CommonResp{ID: tg.ID, Message: "标签已创建"}, nil
}

// TagUpdate 更新标签：slug 变更时在同一事务里留下 301 历史记录
func (l *TagLogic) TagUpdate(req *types.TagUpdateReq) error {
	tg, err := l.svcCtx.EntClient.Tag.Get(l.ctx, req.ID)
	if err != nil {
		return tagGetError(err)
	}
	newSlug := tg.Slug
	if raw := strings.TrimSpace(req.Slug); raw != "" {
		if !IsValidSlug(raw) {
			return common.NewBizError(common.ErrArticleInvalidSlug)
		}
		if err := l.ensureTagSlugFree(raw, req.ID); err != nil {
			return err
		}
		newSlug = raw
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		if err := l.ensureTagNameFree(name, req.ID); err != nil {
			return err
		}
	}

	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrTagUpdateFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if newSlug != tg.Slug {
		if err := upsertSlugHistory(l.ctx, tx, EntityTypeTag, tg.ID, tg.Slug); err != nil {
			return common.NewBizError(common.ErrTagUpdateFail)
		}
	}
	update := tx.Tag.UpdateOneID(tg.ID).SetSlug(newSlug)
	if name := strings.TrimSpace(req.Name); name != "" {
		update = update.SetName(name)
	}
	if req.Description != "" {
		update = update.SetDescription(strings.TrimSpace(req.Description))
	}
	if req.Sort != nil {
		update = update.SetSort(*req.Sort)
	}
	if req.IsActive != nil {
		update = update.SetIsActive(*req.IsActive)
	}
	if _, err := update.Save(l.ctx); err != nil {
		logx.Errorf("update tag failed: %v", err)
		return common.NewBizError(common.ErrTagUpdateFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrTagUpdateFail)
	}
	committed = true
	return nil
}

// TagDelete 软删除标签：事务内记录原 slug、清理关联行、释放唯一索引并写 deleted_at
func (l *TagLogic) TagDelete(req *types.TagIDReq) error {
	tg, err := l.svcCtx.EntClient.Tag.Get(l.ctx, req.ID)
	if err != nil {
		return tagGetError(err)
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrTagDeleteFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now := time.Now()
	if err := upsertSlugHistory(l.ctx, tx, EntityTypeTag, tg.ID, tg.Slug); err != nil {
		return common.NewBizError(common.ErrTagDeleteFail)
	}
	if _, err := tx.Tag.UpdateOneID(tg.ID).
		SetSlug(TrashSlug(tg.ID, now)).
		SetDeletedAt(now).
		ClearArticles().
		Save(l.ctx); err != nil {
		logx.Errorf("soft delete tag failed: %v", err)
		return common.NewBizError(common.ErrTagDeleteFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrTagDeleteFail)
	}
	committed = true
	return nil
}

func (l *TagLogic) ensureTagNameFree(name string, excludeID int64) error {
	query := l.svcCtx.EntClient.Tag.Query().Where(tag.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(tag.IDNEQ(excludeID))
	}
	exists, err := query.Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrTagNameExists)
	}
	return nil
}

func (l *TagLogic) ensureTagSlugFree(slug string, excludeID int64) error {
	query := l.svcCtx.EntClient.Tag.Query().Where(tag.SlugEQ(slug))
	if excludeID > 0 {
		query = query.Where(tag.IDNEQ(excludeID))
	}
	exists, err := query.Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrTagSlugExists)
	}
	return nil
}

func tagGetError(err error) error {
	if ent.IsNotFound(err) {
		return common.NewBizError(common.ErrTagNotFound)
	}
	logx.Errorf("get tag failed: %v", err)
	return common.NewBizError(common.ErrInternalServer)
}
