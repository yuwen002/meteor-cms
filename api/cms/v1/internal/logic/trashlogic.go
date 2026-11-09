package logic

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/articlereview"
	"github.com/yuwen002/meteor-cms/ent/articlerevision"
	"github.com/yuwen002/meteor-cms/ent/articletag"
	"github.com/yuwen002/meteor-cms/ent/category"
	entschema "github.com/yuwen002/meteor-cms/ent/schema"
	"github.com/yuwen002/meteor-cms/ent/slugredirect"
	"github.com/yuwen002/meteor-cms/ent/tag"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/upload"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

type TrashLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTrashLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TrashLogic {
	return &TrashLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func validEntityType(t string) bool {
	return t == EntityTypeArticle || t == EntityTypeCategory || t == EntityTypeTag
}

// TrashList 回收站列表。软删除拦截器默认过滤 deleted_at 非空的行，
// 这里用 SkipSoftDelete 显式读取；三类合并后按删除时间倒序做内存分页。
func (l *TrashLogic) TrashList(req *types.TrashListReq) (*types.TrashListResp, error) {
	kind := strings.TrimSpace(req.EntityType)
	if kind != "" && !validEntityType(kind) {
		return nil, common.NewBizError(common.ErrTrashInvalidType)
	}
	skipCtx := entschema.SkipSoftDelete(l.ctx)
	items := make([]types.TrashItem, 0)

	if kind == "" || kind == EntityTypeArticle {
		q, err := articleQueryVisible(l.ctx,
			l.svcCtx.EntClient.Article.Query().Where(article.DeletedAtNotNil()))
		if err != nil {
			return nil, err
		}
		rows, err := q.All(skipCtx)
		if err != nil {
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		for _, a := range rows {
			items = append(items, types.TrashItem{
				ID: a.ID, EntityType: EntityTypeArticle, Title: a.Title,
				Slug: a.Slug, DeletedAt: FormatOptionalTime(a.DeletedAt),
			})
		}
	}
	if kind == "" || kind == EntityTypeCategory {
		rows, err := l.svcCtx.EntClient.Category.Query().
			Where(category.DeletedAtNotNil()).All(skipCtx)
		if err != nil {
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		for _, c := range rows {
			items = append(items, types.TrashItem{
				ID: c.ID, EntityType: EntityTypeCategory, Title: c.Name,
				Slug: c.Slug, DeletedAt: FormatOptionalTime(c.DeletedAt),
			})
		}
	}
	if kind == "" || kind == EntityTypeTag {
		rows, err := l.svcCtx.EntClient.Tag.Query().
			Where(tag.DeletedAtNotNil()).All(skipCtx)
		if err != nil {
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		for _, t := range rows {
			items = append(items, types.TrashItem{
				ID: t.ID, EntityType: EntityTypeTag, Title: t.Name,
				Slug: t.Slug, DeletedAt: FormatOptionalTime(t.DeletedAt),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].DeletedAt > items[j].DeletedAt })
	page, pageSize := utils.NormalizePagination(req.Page, req.PageSize, 20)
	total := int64(len(items))
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	list := items[start:end]
	if list == nil {
		list = []types.TrashItem{}
	}
	return &types.TrashListResp{
		List: list, Total: total, Page: page, PageSize: pageSize,
	}, nil
}

// TrashRestore 恢复软删除记录。删除时原 slug 被改成一次性占位值并存进历史表，
// 恢复时尝试还原；若该 slug 已被其它内容占用，则生成新 slug 并在消息里说明。
func (l *TrashLogic) TrashRestore(req *types.TrashEntityReq) (*types.CommonResp, error) {
	if !validEntityType(req.EntityType) {
		return nil, common.NewBizError(common.ErrTrashInvalidType)
	}
	skipCtx := entschema.SkipSoftDelete(l.ctx)
	if err := l.ensureInTrash(skipCtx, req.EntityType, req.ID); err != nil {
		return nil, err
	}
	if req.EntityType == EntityTypeArticle {
		a, err := loadArticleForRead(l.ctx, l.svcCtx.EntClient, req.ID, true)
		if err != nil {
			return nil, err
		}
		if err := ensureArticleReadable(l.ctx, a); err != nil {
			return nil, err
		}
	}

	slug, err := l.originalSlug(req.EntityType, req.ID)
	if err != nil {
		return nil, common.NewBizError(common.ErrTrashRestoreFail)
	}
	conflict, err := l.slugOccupied(req.EntityType, req.ID, slug)
	if err != nil {
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	message := "已恢复"
	if conflict {
		slug = TrashSlug(req.ID, time.Now())
		message = "已恢复；原别名已被占用，请手动重新设置别名"
	}

	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return nil, common.NewBizError(common.ErrTrashRestoreFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	switch req.EntityType {
	case EntityTypeArticle:
		if _, err := tx.Article.UpdateOneID(req.ID).ClearDeletedAt().SetSlug(slug).Save(l.ctx); err != nil {
			return nil, common.NewBizError(common.ErrTrashRestoreFail)
		}
	case EntityTypeCategory:
		if _, err := tx.Category.UpdateOneID(req.ID).ClearDeletedAt().SetSlug(slug).Save(l.ctx); err != nil {
			return nil, common.NewBizError(common.ErrTrashRestoreFail)
		}
	case EntityTypeTag:
		if _, err := tx.Tag.UpdateOneID(req.ID).ClearDeletedAt().SetSlug(slug).Save(l.ctx); err != nil {
			return nil, common.NewBizError(common.ErrTrashRestoreFail)
		}
	}
	if _, err := tx.SlugRedirect.Delete().
		Where(slugredirect.EntityTypeEQ(req.EntityType), slugredirect.EntityIDEQ(req.ID)).
		Exec(l.ctx); err != nil {
		return nil, common.NewBizError(common.ErrTrashRestoreFail)
	}
	if err := tx.Commit(); err != nil {
		return nil, common.NewBizError(common.ErrTrashRestoreFail)
	}
	committed = true
	return &types.CommonResp{ID: req.ID, Message: message}, nil
}

// TrashPurge 彻底物理删除：事务内清理所有关联数据与历史别名，
// 文章还会清理不再被任何其它内容引用的封面与正文图片。
func (l *TrashLogic) TrashPurge(req *types.TrashEntityReq) error {
	if !validEntityType(req.EntityType) {
		return common.NewBizError(common.ErrTrashInvalidType)
	}
	skipCtx := entschema.SkipSoftDelete(l.ctx)
	if err := l.ensureInTrash(skipCtx, req.EntityType, req.ID); err != nil {
		return err
	}
	var orphanURLs []string
	if req.EntityType == EntityTypeArticle {
		a, err := l.svcCtx.EntClient.Article.Get(skipCtx, req.ID)
		if err != nil {
			return common.NewBizError(common.ErrTrashNotFound)
		}
		if err := ensureArticleReadable(l.ctx, a); err != nil {
			return err
		}
		orphanURLs = l.collectOrphanURLs(skipCtx, a)
	}

	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrTrashPurgeFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	switch req.EntityType {
	case EntityTypeArticle:
		if _, err := tx.ArticleTag.Delete().
			Where(articletag.ArticleIDEQ(req.ID)).Exec(l.ctx); err != nil {
			return common.NewBizError(common.ErrTrashPurgeFail)
		}
		if _, err := tx.ArticleRevision.Delete().
			Where(articlerevision.ArticleIDEQ(req.ID)).Exec(l.ctx); err != nil {
			return common.NewBizError(common.ErrTrashPurgeFail)
		}
		if _, err := tx.ArticleReview.Delete().
			Where(articlereview.ArticleIDEQ(req.ID)).Exec(l.ctx); err != nil {
			return common.NewBizError(common.ErrTrashPurgeFail)
		}
		if err := tx.Article.DeleteOneID(req.ID).Exec(skipCtx); err != nil {
			return common.NewBizError(common.ErrTrashPurgeFail)
		}
	case EntityTypeCategory:
		if err := tx.Category.DeleteOneID(req.ID).Exec(skipCtx); err != nil {
			return common.NewBizError(common.ErrTrashPurgeFail)
		}
	case EntityTypeTag:
		if _, err := tx.ArticleTag.Delete().
			Where(articletag.TagIDEQ(req.ID)).Exec(l.ctx); err != nil {
			return common.NewBizError(common.ErrTrashPurgeFail)
		}
		if err := tx.Tag.DeleteOneID(req.ID).Exec(skipCtx); err != nil {
			return common.NewBizError(common.ErrTrashPurgeFail)
		}
	}
	if _, err := tx.SlugRedirect.Delete().
		Where(slugredirect.EntityTypeEQ(req.EntityType), slugredirect.EntityIDEQ(req.ID)).
		Exec(l.ctx); err != nil {
		return common.NewBizError(common.ErrTrashPurgeFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrTrashPurgeFail)
	}
	committed = true

	// 文件删除放在事务提交后：文件系统无法回滚，且失败不影响数据库结果。
	root := l.svcCtx.Config.Upload.Defaults().Dir
	for _, u := range orphanURLs {
		if _, err := upload.Delete(root, u); err != nil {
			logx.Errorf("purge orphan upload %s failed: %v", u, err)
		}
	}
	return nil
}

func (l *TrashLogic) ensureInTrash(skipCtx context.Context, entityType string, id int64) error {
	var exists bool
	var err error
	switch entityType {
	case EntityTypeArticle:
		exists, err = l.svcCtx.EntClient.Article.Query().
			Where(article.IDEQ(id), article.DeletedAtNotNil()).Exist(skipCtx)
	case EntityTypeCategory:
		exists, err = l.svcCtx.EntClient.Category.Query().
			Where(category.IDEQ(id), category.DeletedAtNotNil()).Exist(skipCtx)
	case EntityTypeTag:
		exists, err = l.svcCtx.EntClient.Tag.Query().
			Where(tag.IDEQ(id), tag.DeletedAtNotNil()).Exist(skipCtx)
	}
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if !exists {
		return common.NewBizError(common.ErrTrashNotFound)
	}
	return nil
}

// originalSlug 从历史表里找该实体最近一次的真实 slug；找不到时用占位值兜底
func (l *TrashLogic) originalSlug(entityType string, id int64) (string, error) {
	rows, err := l.svcCtx.EntClient.SlugRedirect.Query().
		Where(slugredirect.EntityTypeEQ(entityType), slugredirect.EntityIDEQ(id)).
		Order(ent.Desc(slugredirect.FieldID)).
		All(l.ctx)
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if !IsTrashSlug(r.OldSlug) {
			return r.OldSlug, nil
		}
	}
	return fmt.Sprintf("restored-%d-%d", id, time.Now().UnixNano()), nil
}

func (l *TrashLogic) slugOccupied(entityType string, excludeID int64, slug string) (bool, error) {
	if slug == "" || IsTrashSlug(slug) {
		return false, nil
	}
	switch entityType {
	case EntityTypeArticle:
		return l.svcCtx.EntClient.Article.Query().
			Where(article.SlugEQ(slug), article.IDNEQ(excludeID)).Exist(l.ctx)
	case EntityTypeCategory:
		return l.svcCtx.EntClient.Category.Query().
			Where(category.SlugEQ(slug), category.IDNEQ(excludeID)).Exist(l.ctx)
	case EntityTypeTag:
		return l.svcCtx.EntClient.Tag.Query().
			Where(tag.SlugEQ(slug), tag.IDNEQ(excludeID)).Exist(l.ctx)
	}
	return false, nil
}

// collectOrphanURLs 收集仅被这篇待删文章引用的站内上传文件，避免误删共享资源
func (l *TrashLogic) collectOrphanURLs(skipCtx context.Context, a *ent.Article) []string {
	candidates := make([]string, 0)
	if strings.HasPrefix(a.Cover, "/uploads/") {
		candidates = append(candidates, a.Cover)
	}
	candidates = append(candidates, ExtractMarkdownImageURLs(a.Content)...)
	seen := make(map[string]bool)
	out := make([]string, 0, len(candidates))
	for _, u := range candidates {
		if seen[u] {
			continue
		}
		seen[u] = true
		used, err := l.svcCtx.EntClient.Article.Query().
			Where(
				article.IDNEQ(a.ID),
				article.Or(article.CoverEQ(u), article.ContentContains(u)),
			).Exist(skipCtx)
		if err != nil {
			logx.Errorf("check upload reference %s failed: %v", u, err)
			continue
		}
		if !used {
			out = append(out, u)
		}
	}
	return out
}
