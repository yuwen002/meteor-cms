package logic

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/category"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/core/logx"
)

type CategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CategoryLogic {
	return &CategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CategoryList 分类列表（平铺 + parent_name，便于表格直接展示层级）
func (l *CategoryLogic) CategoryList(req *types.CategoryListReq) (*types.CategoryListResp, error) {
	query := l.svcCtx.EntClient.Category.Query()
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		query = query.Where(category.Or(
			category.NameContainsFold(kw),
			category.SlugContainsFold(kw),
		))
	}
	if req.IsActive != nil {
		query = query.Where(category.IsActiveEQ(*req.IsActive))
	}
	rows, err := query.Order(ent.Asc(category.FieldSort), ent.Asc(category.FieldID)).All(l.ctx)
	if err != nil {
		logx.Errorf("list categories failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	names := make(map[int64]string, len(rows))
	for _, c := range rows {
		names[c.ID] = c.Name
	}
	counts, err := categoryArticleCounts(l.ctx, l.svcCtx.EntClient)
	if err != nil {
		return nil, err
	}
	list := make([]types.CategoryItem, 0, len(rows))
	for _, c := range rows {
		item := types.CategoryItem{
			ID:           c.ID,
			Name:         c.Name,
			Slug:         c.Slug,
			Description:  c.Description,
			Sort:         c.Sort,
			IsActive:     c.IsActive,
			ArticleCount: int(counts[c.ID]),
			CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		}
		if c.ParentID != nil {
			item.ParentID = *c.ParentID
			item.ParentName = names[*c.ParentID]
		}
		list = append(list, item)
	}
	return &types.CategoryListResp{List: list}, nil
}

// CategoryCreate 新建分类
func (l *CategoryLogic) CategoryCreate(req *types.CategoryCreateReq) (*types.CommonResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "分类名称不能为空")
	}
	rawSlug := strings.TrimSpace(req.Slug)
	if rawSlug != "" && !IsValidSlug(rawSlug) {
		return nil, common.NewBizError(common.ErrArticleInvalidSlug)
	}
	if err := l.ensureCategoryNameFree(name, 0); err != nil {
		return nil, err
	}
	slug := GenerateSlug(rawSlug, name, time.Now())
	if err := l.ensureCategorySlugFree(slug, 0); err != nil {
		return nil, err
	}
	if req.ParentID > 0 {
		if err := l.ensureParentChainValid(req.ParentID, 0); err != nil {
			return nil, err
		}
	}

	create := l.svcCtx.EntClient.Category.Create().
		SetName(name).
		SetSlug(slug).
		SetDescription(strings.TrimSpace(req.Description)).
		SetSort(req.Sort).
		SetIsActive(true)
	if req.IsActive != nil {
		create = create.SetIsActive(*req.IsActive)
	}
	if req.ParentID > 0 {
		create = create.SetParentID(req.ParentID)
	}
	c, err := create.Save(l.ctx)
	if err != nil {
		logx.Errorf("create category failed: %v", err)
		return nil, common.NewBizError(common.ErrCategoryCreateFail)
	}
	return &types.CommonResp{ID: c.ID, Message: "分类已创建"}, nil
}

// CategoryUpdate 更新分类：slug 变更时在同一事务里留下 301 历史记录
func (l *CategoryLogic) CategoryUpdate(req *types.CategoryUpdateReq) error {
	c, err := l.svcCtx.EntClient.Category.Get(l.ctx, req.ID)
	if err != nil {
		return categoryGetError(err)
	}
	newSlug := c.Slug
	if raw := strings.TrimSpace(req.Slug); raw != "" {
		if !IsValidSlug(raw) {
			return common.NewBizError(common.ErrArticleInvalidSlug)
		}
		if err := l.ensureCategorySlugFree(raw, req.ID); err != nil {
			return err
		}
		newSlug = raw
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		if err := l.ensureCategoryNameFree(name, req.ID); err != nil {
			return err
		}
	}
	if req.ParentID != nil {
		pid := *req.ParentID
		if pid == req.ID {
			return common.NewBizError(common.ErrCategoryParentSelf)
		}
		if pid > 0 {
			if err := l.ensureParentChainValid(pid, req.ID); err != nil {
				return err
			}
		}
	}

	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrCategoryUpdateFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if newSlug != c.Slug {
		if err := upsertSlugHistory(l.ctx, tx, EntityTypeCategory, c.ID, c.Slug); err != nil {
			return common.NewBizError(common.ErrCategoryUpdateFail)
		}
	}

	update := tx.Category.UpdateOneID(c.ID).SetSlug(newSlug)
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
	if req.ParentID != nil {
		if *req.ParentID > 0 {
			update = update.SetParentID(*req.ParentID)
		} else {
			update = update.ClearParent()
		}
	}
	if _, err := update.Save(l.ctx); err != nil {
		logx.Errorf("update category failed: %v", err)
		return common.NewBizError(common.ErrCategoryUpdateFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrCategoryUpdateFail)
	}
	committed = true
	return nil
}

// CategoryDelete 软删除空分类：事务内记录原 slug 快照、释放唯一索引、写 deleted_at
func (l *CategoryLogic) CategoryDelete(req *types.CategoryIDReq) error {
	c, err := l.svcCtx.EntClient.Category.Get(l.ctx, req.ID)
	if err != nil {
		return categoryGetError(err)
	}
	hasChildren, err := l.svcCtx.EntClient.Category.Query().
		Where(category.ParentIDEQ(c.ID)).Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if hasChildren {
		return common.NewBizError(common.ErrCategoryHasChildren)
	}
	hasArticles, err := l.svcCtx.EntClient.Article.Query().
		Where(article.CategoryIDEQ(c.ID)).Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if hasArticles {
		return common.NewBizError(common.ErrCategoryHasArticles)
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrCategoryDeleteFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now := time.Now()
	if err := upsertSlugHistory(l.ctx, tx, EntityTypeCategory, c.ID, c.Slug); err != nil {
		return common.NewBizError(common.ErrCategoryDeleteFail)
	}
	if _, err := tx.Category.UpdateOneID(c.ID).
		SetSlug(TrashSlug(c.ID, now)).
		SetDeletedAt(now).
		Save(l.ctx); err != nil {
		return common.NewBizError(common.ErrCategoryDeleteFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrCategoryDeleteFail)
	}
	committed = true
	return nil
}

// ensureParentChainValid 沿父级链向上走，校验父级存在且不会把自己卷进环里
func (l *CategoryLogic) ensureParentChainValid(parentID, selfID int64) error {
	seen := make(map[int64]bool)
	cur := parentID
	for cur > 0 {
		if cur == selfID && selfID > 0 {
			return common.NewBizError(common.ErrCategoryParentNested)
		}
		if seen[cur] {
			return common.NewBizError(common.ErrCategoryParentNested)
		}
		seen[cur] = true
		parent, err := l.svcCtx.EntClient.Category.Get(l.ctx, cur)
		if err != nil {
			if ent.IsNotFound(err) {
				return common.NewBizError(common.ErrCategoryNotFound)
			}
			return common.NewBizError(common.ErrInternalServer)
		}
		if parent.ParentID == nil {
			return nil
		}
		cur = *parent.ParentID
	}
	return nil
}

func (l *CategoryLogic) ensureCategoryNameFree(name string, excludeID int64) error {
	query := l.svcCtx.EntClient.Category.Query().Where(category.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(category.IDNEQ(excludeID))
	}
	exists, err := query.Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrCategoryNameExists)
	}
	return nil
}

func (l *CategoryLogic) ensureCategorySlugFree(slug string, excludeID int64) error {
	query := l.svcCtx.EntClient.Category.Query().Where(category.SlugEQ(slug))
	if excludeID > 0 {
		query = query.Where(category.IDNEQ(excludeID))
	}
	exists, err := query.Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrCategorySlugExists)
	}
	return nil
}

func categoryGetError(err error) error {
	if ent.IsNotFound(err) {
		return common.NewBizError(common.ErrCategoryNotFound)
	}
	logx.Errorf("get category failed: %v", err)
	return common.NewBizError(common.ErrInternalServer)
}
