package logic

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/articlereview"
	"github.com/yuwen002/meteor-cms/ent/articlerevision"
	"github.com/yuwen002/meteor-cms/ent/category"
	"github.com/yuwen002/meteor-cms/ent/tag"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

type ArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArticleLogic {
	return &ArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ensureOwner 数据权限：超级管理员可管理全部文章，其余用户只能操作自己创建的文章。
// 用 claims 里的 is_super 判断，避免每个写操作再查一次用户表。
func (l *ArticleLogic) ensureOwner(a *ent.Article) error {
	if utils.IsSuperAdmin(l.ctx) {
		return nil
	}
	userID, ok := utils.CurrentUserID(l.ctx)
	if !ok || a.AuthorID == nil || *a.AuthorID != userID {
		return common.NewBizError(common.ErrArticleForbidden)
	}
	return nil
}

// ArticleList 后台文章列表：支持关键字、状态、分类、标签筛选与分页
func (l *ArticleLogic) ArticleList(req *types.ArticleListReq) (*types.ArticleListResp, error) {
	query, err := articleQueryVisible(l.ctx, l.svcCtx.EntClient.Article.Query())
	if err != nil {
		return nil, err
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		query = query.Where(article.Or(
			article.TitleContainsFold(kw),
			article.SummaryContainsFold(kw),
		))
	}
	if req.Status != nil {
		query = query.Where(article.StatusEQ(*req.Status))
	}
	if req.CategoryID > 0 {
		query = query.Where(article.CategoryIDEQ(req.CategoryID))
	}
	if req.TagID > 0 {
		query = query.Where(article.HasTagsWith(tag.IDEQ(req.TagID)))
	}

	total, err := query.Clone().Count(l.ctx)
	if err != nil {
		logx.Errorf("count articles failed: %v", err)
		return nil, common.NewBizError(common.ErrArticleListFail)
	}

	page, pageSize := utils.NormalizePagination(req.Page, req.PageSize, 20)
	offset, limit := utils.NormalizePage(req.Page, req.PageSize, 20)
	rows, err := query.
		WithCategory().
		WithTags().
		Order(articleListOrder(req.Sort, req.Order)...).
		Limit(limit).
		Offset(offset).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list articles failed: %v", err)
		return nil, common.NewBizError(common.ErrArticleListFail)
	}

	names := lookupAuthorNames(l.ctx, l.svcCtx.EntClient, articleAuthorIDs(rows))
	now := time.Now()
	list := make([]types.ArticleItem, 0, len(rows))
	for _, a := range rows {
		list = append(list, articleToItem(a, names, now))
	}
	return &types.ArticleListResp{
		List: list, Total: int64(total), Page: page, PageSize: pageSize,
	}, nil
}

// ArticleDetail 文章详情，含 Markdown 正文与 SEO 字段
func (l *ArticleLogic) ArticleDetail(req *types.ArticleIDReq) (*types.ArticleDetailResp, error) {
	a, err := l.svcCtx.EntClient.Article.Query().
		Where(article.IDEQ(req.ID)).
		WithCategory().
		WithTags().
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrArticleNotFound)
		}
		logx.Errorf("get article failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if err := ensureArticleReadable(l.ctx, a); err != nil {
		return nil, err
	}
	names := lookupAuthorNames(l.ctx, l.svcCtx.EntClient, articleAuthorIDs([]*ent.Article{a}))
	return &types.ArticleDetailResp{
		ArticleItem:    articleToItem(a, names, time.Now()),
		Content:        a.Content,
		SeoTitle:       a.SeoTitle,
		SeoDescription: a.SeoDescription,
		SeoKeywords:    a.SeoKeywords,
	}, nil
}

// ArticleCreate 新建文章，一律以草稿状态落库，发布必须走发布/审核接口
func (l *ArticleLogic) ArticleCreate(req *types.ArticleCreateReq) (*types.CommonResp, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "文章标题不能为空")
	}
	rawSlug := strings.TrimSpace(req.Slug)
	if rawSlug != "" && !IsValidSlug(rawSlug) {
		return nil, common.NewBizError(common.ErrArticleInvalidSlug)
	}
	slug := GenerateSlug(rawSlug, title, time.Now())
	if err := l.ensureArticleSlugFree(slug, 0); err != nil {
		return nil, err
	}
	tagIDs, err := l.validateArticleTaxonomy(req.CategoryID, req.TagIDs)
	if err != nil {
		return nil, err
	}

	create := l.svcCtx.EntClient.Article.Create().
		SetTitle(title).
		SetSlug(slug).
		SetSummary(strings.TrimSpace(req.Summary)).
		SetCover(strings.TrimSpace(req.Cover)).
		SetContent(req.Content).
		SetStatus(ArticleStatusDraft).
		SetIsTop(req.IsTop).
		SetSeoTitle(strings.TrimSpace(req.SeoTitle)).
		SetSeoDescription(strings.TrimSpace(req.SeoDescription)).
		SetSeoKeywords(strings.TrimSpace(req.SeoKeywords))
	if req.CategoryID > 0 {
		create = create.SetCategoryID(req.CategoryID)
	}
	if userID, ok := utils.CurrentUserID(l.ctx); ok {
		create = create.SetAuthorID(userID)
	}
	if len(tagIDs) > 0 {
		create = create.AddTagIDs(tagIDs...)
	}

	a, err := create.Save(l.ctx)
	if err != nil {
		logx.Errorf("create article failed: %v", err)
		return nil, common.NewBizError(common.ErrArticleCreateFail)
	}
	return &types.CommonResp{ID: a.ID, Message: "文章已保存为草稿"}, nil
}

// ArticleUpdate 编辑文章：事务内依次完成「修订快照 → 旧 slug 留痕 → 整单更新」，
// 避免旧实现里多步写入中途失败导致数据不一致。
func (l *ArticleLogic) ArticleUpdate(req *types.ArticleUpdateReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if err := l.ensureOwner(a); err != nil {
		return err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return common.NewBizErrorWithMsg(common.ErrInvalidParams, "文章标题不能为空")
	}

	slug := a.Slug
	if raw := strings.TrimSpace(req.Slug); raw != "" {
		if !IsValidSlug(raw) {
			return common.NewBizError(common.ErrArticleInvalidSlug)
		}
		slug = raw
		if err := l.ensureArticleSlugFree(slug, req.ID); err != nil {
			return err
		}
	}
	tagIDs, err := l.validateArticleTaxonomy(req.CategoryID, req.TagIDs)
	if err != nil {
		return err
	}

	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrArticleUpdateFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := l.saveRevision(tx, a, "编辑前自动存档"); err != nil {
		logx.Errorf("save article revision failed: %v", err)
		return common.NewBizError(common.ErrArticleUpdateFail)
	}
	if slug != a.Slug {
		if err := upsertSlugHistory(l.ctx, tx, EntityTypeArticle, a.ID, a.Slug); err != nil {
			logx.Errorf("record article slug history failed: %v", err)
			return common.NewBizError(common.ErrArticleUpdateFail)
		}
	}

	update := tx.Article.UpdateOneID(a.ID).
		SetTitle(title).
		SetSlug(slug).
		SetSummary(strings.TrimSpace(req.Summary)).
		SetCover(strings.TrimSpace(req.Cover)).
		SetContent(req.Content).
		SetIsTop(req.IsTop).
		SetSeoTitle(strings.TrimSpace(req.SeoTitle)).
		SetSeoDescription(strings.TrimSpace(req.SeoDescription)).
		SetSeoKeywords(strings.TrimSpace(req.SeoKeywords))
	if req.CategoryID > 0 {
		update = update.SetCategoryID(req.CategoryID)
	} else {
		update = update.ClearCategory()
	}
	update = update.ClearTags()
	if len(tagIDs) > 0 {
		update = update.AddTagIDs(tagIDs...)
	}
	if _, err := update.Save(l.ctx); err != nil {
		logx.Errorf("update article failed: %v", err)
		return common.NewBizError(common.ErrArticleUpdateFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrArticleUpdateFail)
	}
	committed = true
	return nil
}

// ArticleDelete 软删除：事务内记录原 slug 快照（供恢复）、把 slug 改成一次性值
// （释放唯一索引），再写 deleted_at。刻意不用 DeleteOneID：软删除 hook 内部会用
// 全局 client 执行 UPDATE，绕过事务，回滚时无法撤销。
func (l *ArticleLogic) ArticleDelete(req *types.ArticleIDReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if err := l.ensureOwner(a); err != nil {
		return err
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrArticleDeleteFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now := time.Now()
	if err := upsertSlugHistory(l.ctx, tx, EntityTypeArticle, a.ID, a.Slug); err != nil {
		return common.NewBizError(common.ErrArticleDeleteFail)
	}
	if _, err := tx.Article.UpdateOneID(a.ID).
		SetSlug(TrashSlug(a.ID, now)).
		SetDeletedAt(now).
		Save(l.ctx); err != nil {
		logx.Errorf("soft delete article failed: %v", err)
		return common.NewBizError(common.ErrArticleDeleteFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrArticleDeleteFail)
	}
	committed = true
	return nil
}

// ArticlePublish 发布/定时发布。待审核文章必须走审核通过接口。
// 重新发布已下线文章且未指定时间时保留首发时间，避免丢失内容原始发布时间。
func (l *ArticleLogic) ArticlePublish(req *types.ArticlePublishReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if err := l.ensureOwner(a); err != nil {
		return err
	}
	if a.Status == ArticleStatusPending {
		return common.NewBizError(common.ErrArticleApproveNeeded)
	}
	if strings.TrimSpace(a.Content) == "" {
		return common.NewBizErrorWithMsg(common.ErrInvalidParams, "正文为空，不能发布")
	}
	publishAt, err := ParsePublishTime(req.PublishedAt, time.Now())
	if err != nil {
		return common.NewBizError(common.ErrArticleInvalidTime)
	}
	if strings.TrimSpace(req.PublishedAt) == "" &&
		a.Status == ArticleStatusOffline && a.PublishedAt != nil {
		publishAt = *a.PublishedAt
	}
	if _, err := a.Update().
		SetStatus(ArticleStatusPublished).
		SetPublishedAt(publishAt).
		Save(l.ctx); err != nil {
		logx.Errorf("publish article failed: %v", err)
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	return nil
}

// ArticleWithdraw 撤回（下线）文章，保留内容供重新编辑后再发布
func (l *ArticleLogic) ArticleWithdraw(req *types.ArticleIDReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if err := l.ensureOwner(a); err != nil {
		return err
	}
	if _, err := a.Update().SetStatus(ArticleStatusOffline).Save(l.ctx); err != nil {
		logx.Errorf("withdraw article failed: %v", err)
		return common.NewBizError(common.ErrArticleWithdrawFail)
	}
	return nil
}

func (l *ArticleLogic) ArticleToggleTop(req *types.ArticleIDReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if err := l.ensureOwner(a); err != nil {
		return err
	}
	if _, err := a.Update().SetIsTop(!a.IsTop).Save(l.ctx); err != nil {
		logx.Errorf("toggle article top failed: %v", err)
		return common.NewBizError(common.ErrArticleSetTopFail)
	}
	return nil
}

// ArticleSubmit 草稿 -> 待审核，并在同一事务里留一条审核记录
func (l *ArticleLogic) ArticleSubmit(req *types.ArticleIDReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if err := l.ensureOwner(a); err != nil {
		return err
	}
	if a.Status != ArticleStatusDraft {
		return common.NewBizErrorWithMsg(common.ErrInvalidParams, "只有草稿可以提交审核")
	}
	if strings.TrimSpace(a.Content) == "" {
		return common.NewBizErrorWithMsg(common.ErrInvalidParams, "正文为空，不能提交审核")
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.Article.UpdateOneID(a.ID).SetStatus(ArticleStatusPending).Save(l.ctx); err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	uid, name := currentOperator(l.ctx, l.svcCtx.EntClient)
	if err := writeReview(l.ctx, tx, a.ID, ReviewActionSubmit, "", uid, name); err != nil {
		logx.Errorf("write article review (submit) failed: %v", err)
		return common.NewBizError(common.ErrInternalServer)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	committed = true
	return nil
}

// ArticleApprove 待审核 -> 已发布。该接口的 API 权限码为 cms:article:approve，
// 权限中间件已保证调用者具备审核权限（超管放行）。
func (l *ArticleLogic) ArticleApprove(req *types.ArticlePublishReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if a.Status != ArticleStatusPending {
		return common.NewBizError(common.ErrArticleNotPending)
	}
	publishAt, err := ParsePublishTime(req.PublishedAt, time.Now())
	if err != nil {
		return common.NewBizError(common.ErrArticleInvalidTime)
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.Article.UpdateOneID(a.ID).
		SetStatus(ArticleStatusPublished).
		SetPublishedAt(publishAt).
		Save(l.ctx); err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	uid, name := currentOperator(l.ctx, l.svcCtx.EntClient)
	if err := writeReview(l.ctx, tx, a.ID, ReviewActionApprove, "", uid, name); err != nil {
		logx.Errorf("write article review (approve) failed: %v", err)
		return common.NewBizError(common.ErrInternalServer)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	committed = true
	return nil
}

// ArticleReject 待审核 -> 草稿，驳回原因写入审核记录
func (l *ArticleLogic) ArticleReject(req *types.ArticleRejectReq) error {
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if a.Status != ArticleStatusPending {
		return common.NewBizError(common.ErrArticleNotPending)
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.Article.UpdateOneID(a.ID).SetStatus(ArticleStatusDraft).Save(l.ctx); err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	uid, name := currentOperator(l.ctx, l.svcCtx.EntClient)
	if err := writeReview(l.ctx, tx, a.ID, ReviewActionReject, req.Remark, uid, name); err != nil {
		logx.Errorf("write article review (reject) failed: %v", err)
		return common.NewBizError(common.ErrInternalServer)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrArticlePublishFail)
	}
	committed = true
	return nil
}

// ArticleReviewList 返回最近 50 条审核动作，供编辑页展示审核轨迹
func (l *ArticleLogic) ArticleReviewList(req *types.ArticleIDReq) (*types.ArticleReviewListResp, error) {
	if err := ensureArticleIDReadable(l.ctx, l.svcCtx.EntClient, req.ID, false); err != nil {
		return nil, err
	}
	rows, err := l.svcCtx.EntClient.ArticleReview.Query().
		Where(articlereview.ArticleIDEQ(req.ID)).
		Order(ent.Desc(articlereview.FieldID)).
		Limit(50).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list article reviews failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	list := make([]types.ArticleReviewItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, types.ArticleReviewItem{
			ID:           r.ID,
			ArticleID:    r.ArticleID,
			Action:       r.Action,
			ActionText:   ReviewActionText(r.Action),
			Remark:       r.Remark,
			OperatorID:   r.OperatorID,
			OperatorName: r.OperatorName,
			CreatedAt:    FormatOptionalTime(&r.CreatedAt),
		})
	}
	return &types.ArticleReviewListResp{List: list}, nil
}

// ---------- 修订历史 ----------

func (l *ArticleLogic) ArticleRevisionList(req *types.ArticleIDReq) (*types.ArticleRevisionListResp, error) {
	if err := ensureArticleIDReadable(l.ctx, l.svcCtx.EntClient, req.ID, false); err != nil {
		return nil, err
	}
	rows, err := l.svcCtx.EntClient.ArticleRevision.Query().
		Where(articlerevision.ArticleIDEQ(req.ID)).
		Order(ent.Desc(articlerevision.FieldID)).
		Limit(50).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list article revisions failed: %v", err)
		return nil, common.NewBizError(common.ErrRevisionListFail)
	}
	list := make([]types.ArticleRevisionItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, types.ArticleRevisionItem{
			ID:        r.ID,
			ArticleID: r.ArticleID,
			Title:     r.Title,
			Remark:    r.Remark,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &types.ArticleRevisionListResp{List: list}, nil
}

func (l *ArticleLogic) ArticleRevisionDetail(req *types.ArticleRevisionIDReq) (*types.ArticleRevisionDetailResp, error) {
	if err := ensureArticleIDReadable(l.ctx, l.svcCtx.EntClient, req.ID, false); err != nil {
		return nil, err
	}
	r, err := l.svcCtx.EntClient.ArticleRevision.Query().
		Where(articlerevision.IDEQ(req.RevisionID), articlerevision.ArticleIDEQ(req.ID)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrRevisionNotFound)
		}
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	return &types.ArticleRevisionDetailResp{
		ArticleRevisionItem: types.ArticleRevisionItem{
			ID: r.ID, ArticleID: r.ArticleID, Title: r.Title,
			Remark: r.Remark, CreatedAt: r.CreatedAt.Format(time.RFC3339),
		},
		Summary: r.Summary,
		Cover:   r.Cover,
		Content: r.Content,
	}, nil
}

// ArticleRevisionRestore 把历史版本写回文章，恢复前先把当前内容再存一版，保证可逆
func (l *ArticleLogic) ArticleRevisionRestore(req *types.ArticleRevisionIDReq) error {
	r, err := l.svcCtx.EntClient.ArticleRevision.Query().
		Where(articlerevision.IDEQ(req.RevisionID), articlerevision.ArticleIDEQ(req.ID)).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return common.NewBizError(common.ErrRevisionNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}
	a, err := l.svcCtx.EntClient.Article.Get(l.ctx, req.ID)
	if err != nil {
		return articleGetError(err)
	}
	if err := l.ensureOwner(a); err != nil {
		return err
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrRevisionRestoreFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := l.saveRevision(tx, a, "恢复历史版本前自动存档"); err != nil {
		return common.NewBizError(common.ErrRevisionRestoreFail)
	}
	if _, err := tx.Article.UpdateOneID(a.ID).
		SetTitle(r.Title).SetSummary(r.Summary).SetCover(r.Cover).SetContent(r.Content).
		Save(l.ctx); err != nil {
		return common.NewBizError(common.ErrRevisionRestoreFail)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrRevisionRestoreFail)
	}
	committed = true
	return nil
}

func (l *ArticleLogic) saveRevision(tx *ent.Tx, a *ent.Article, remark string) error {
	create := tx.ArticleRevision.Create().
		SetArticleID(a.ID).
		SetTitle(a.Title).
		SetSummary(a.Summary).
		SetCover(a.Cover).
		SetContent(a.Content).
		SetRemark(remark)
	if userID, ok := utils.CurrentUserID(l.ctx); ok {
		create = create.SetEditorID(userID)
	}
	return create.Exec(l.ctx)
}

func articleGetError(err error) error {
	if ent.IsNotFound(err) {
		return common.NewBizError(common.ErrArticleNotFound)
	}
	logx.Errorf("get article failed: %v", err)
	return common.NewBizError(common.ErrInternalServer)
}

// articleListOrder 排序白名单：用户输入只用于挑选预定义字段，不拼进 SQL。
// 未指定时保持「置顶优先 + 最新创建」的默认顺序。
func articleListOrder(sortBy, order string) []article.OrderOption {
	allowed := map[string]string{
		"published_at": article.FieldPublishedAt,
		"view_count":   article.FieldViewCount,
		"updated_at":   article.FieldUpdatedAt,
		"created_at":   article.FieldCreatedAt,
		"id":           article.FieldID,
	}
	field, ok := allowed[strings.ToLower(strings.TrimSpace(sortBy))]
	if !ok {
		return []article.OrderOption{
			article.OrderOption(ent.Desc(article.FieldIsTop)),
			article.OrderOption(ent.Desc(article.FieldID)),
		}
	}
	fn := ent.Desc
	if strings.EqualFold(strings.TrimSpace(order), "asc") {
		fn = ent.Asc
	}
	return []article.OrderOption{
		article.OrderOption(fn(field)),
		article.OrderOption(ent.Desc(article.FieldID)),
	}
}

const maxBatchArticles = 100

func normalizeBatchIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "请至少选择一篇文章")
	}
	if len(out) > maxBatchArticles {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "单次最多操作 100 篇文章")
	}
	return out, nil
}

// batchArticles 一次取回目标文章并逐条做数据权限校验，避免批量接口绕过 ensureOwner。
func (l *ArticleLogic) batchArticles(req *types.ArticleBatchReq) ([]*ent.Article, error) {
	ids, err := normalizeBatchIDs(req.IDs)
	if err != nil {
		return nil, err
	}
	rows, err := l.svcCtx.EntClient.Article.Query().Where(article.IDIn(ids...)).All(l.ctx)
	if err != nil {
		logx.Errorf("batch load articles failed: %v", err)
		return nil, common.NewBizError(common.ErrArticleListFail)
	}
	for _, a := range rows {
		if err := l.ensureOwner(a); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// ArticleBatchOffline 批量下线：只处理已发布文章，其余状态原样保留。
func (l *ArticleLogic) ArticleBatchOffline(req *types.ArticleBatchReq) (*types.ArticleBatchResp, error) {
	rows, err := l.batchArticles(req)
	if err != nil {
		return nil, err
	}
	targets := make([]*ent.Article, 0, len(rows))
	for _, a := range rows {
		if a.Status == ArticleStatusPublished {
			targets = append(targets, a)
		}
	}
	if len(targets) == 0 {
		return &types.ArticleBatchResp{Count: 0}, nil
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return nil, common.NewBizError(common.ErrArticleWithdrawFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, a := range targets {
		if _, err := tx.Article.UpdateOneID(a.ID).
			SetStatus(ArticleStatusOffline).Save(l.ctx); err != nil {
			logx.Errorf("batch offline article %d failed: %v", a.ID, err)
			return nil, common.NewBizError(common.ErrArticleWithdrawFail)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, common.NewBizError(common.ErrArticleWithdrawFail)
	}
	committed = true
	return &types.ArticleBatchResp{Count: len(targets)}, nil
}

// ArticleBatchDelete 批量软删除：与单条删除一致，事务内留痕 + 释放 slug + 写 deleted_at。
func (l *ArticleLogic) ArticleBatchDelete(req *types.ArticleBatchReq) (*types.ArticleBatchResp, error) {
	rows, err := l.batchArticles(req)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &types.ArticleBatchResp{Count: 0}, nil
	}
	cancel := newTxContext(&l.ctx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(l.ctx)
	if err != nil {
		return nil, common.NewBizError(common.ErrArticleDeleteFail)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now := time.Now()
	for _, a := range rows {
		if err := upsertSlugHistory(l.ctx, tx, EntityTypeArticle, a.ID, a.Slug); err != nil {
			return nil, common.NewBizError(common.ErrArticleDeleteFail)
		}
		if _, err := tx.Article.UpdateOneID(a.ID).
			SetSlug(TrashSlug(a.ID, now)).
			SetDeletedAt(now).
			Save(l.ctx); err != nil {
			logx.Errorf("batch soft delete article %d failed: %v", a.ID, err)
			return nil, common.NewBizError(common.ErrArticleDeleteFail)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, common.NewBizError(common.ErrArticleDeleteFail)
	}
	committed = true
	return &types.ArticleBatchResp{Count: len(rows)}, nil
}

func articleToItem(a *ent.Article, names map[int64]string, now time.Time) types.ArticleItem {
	item := types.ArticleItem{
		ID:          a.ID,
		Title:       a.Title,
		Slug:        a.Slug,
		Summary:     a.Summary,
		Cover:       a.Cover,
		Status:      a.Status,
		StatusText:  ArticleDisplayStatus(a.Status, a.PublishedAt, now),
		AuthorName:  authorNameOf(names, a),
		TagIDs:      make([]int64, 0, len(a.Edges.Tags)),
		TagNames:    make([]string, 0, len(a.Edges.Tags)),
		IsTop:       a.IsTop,
		ViewCount:   a.ViewCount,
		PublishedAt: FormatOptionalTime(a.PublishedAt),
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.Format(time.RFC3339),
	}
	if a.CategoryID != nil {
		item.CategoryID = *a.CategoryID
	}
	if a.Edges.Category != nil {
		item.CategoryName = a.Edges.Category.Name
	}
	for _, tg := range a.Edges.Tags {
		item.TagIDs = append(item.TagIDs, tg.ID)
		item.TagNames = append(item.TagNames, tg.Name)
	}
	return item
}

// ensureArticleSlugFree 校验 slug 未被其它文章占用（软删除的文章已被拦截器过滤）
func (l *ArticleLogic) ensureArticleSlugFree(slug string, excludeID int64) error {
	query := l.svcCtx.EntClient.Article.Query().Where(article.SlugEQ(slug))
	if excludeID > 0 {
		query = query.Where(article.IDNEQ(excludeID))
	}
	exists, err := query.Exist(l.ctx)
	if err != nil {
		logx.Errorf("check article slug failed: %v", err)
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrArticleSlugExists)
	}
	return nil
}

// validateArticleTaxonomy 校验分类与标签存在且启用，并返回去重后的标签 ID 列表
func (l *ArticleLogic) validateArticleTaxonomy(categoryID int64, tagIDs []int64) ([]int64, error) {
	if categoryID > 0 {
		ok, err := l.svcCtx.EntClient.Category.Query().
			Where(category.IDEQ(categoryID), category.IsActiveEQ(true)).
			Exist(l.ctx)
		if err != nil {
			logx.Errorf("check article category failed: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if !ok {
			return nil, common.NewBizError(common.ErrArticleCategoryInvalid)
		}
	}

	unique := make([]int64, 0, len(tagIDs))
	seen := make(map[int64]bool, len(tagIDs))
	for _, id := range tagIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return unique, nil
	}
	count, err := l.svcCtx.EntClient.Tag.Query().
		Where(tag.IDIn(unique...), tag.IsActiveEQ(true)).
		Count(l.ctx)
	if err != nil {
		logx.Errorf("check article tags failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if count != len(unique) {
		return nil, common.NewBizError(common.ErrArticleTagInvalid)
	}
	return unique, nil
}
