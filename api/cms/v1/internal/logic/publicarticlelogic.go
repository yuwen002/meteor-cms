package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/articletag"
	"github.com/yuwen002/meteor-cms/ent/category"
	"github.com/yuwen002/meteor-cms/ent/tag"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

// viewDedupMem 是 Redis 不可用时的进程内浏览量去重表（key -> 过期 Unix 秒）。
// viewDedupCleanAt 记录上次惰性清理时间，避免这张表随访问 IP 无限增长。
var (
	viewDedupMem     sync.Map
	viewDedupCleanAt atomic.Int64
)

const (
	viewDedupSeconds         = 86400
	viewDedupCleanupInterval = int64(3600)
)

type PublicArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublicArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublicArticleLogic {
	return &PublicArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// visibleQuery 公开可见条件：状态已发布且发布时间已到。
// 定时发布只需在发布时把 published_at 设为未来时间，到点后本查询自动放行。
func (l *PublicArticleLogic) visibleQuery() *ent.ArticleQuery {
	return l.svcCtx.EntClient.Article.Query().
		Where(
			article.StatusEQ(ArticleStatusPublished),
			// published_at 是 datetime(0)，MySQL 会把含纳秒的 time.Now() 四舍五入到下一秒；
			// 严格用 now 判断会让刚发布的文章在 1 秒内对前台不可见，这里与
			// IsPubliclyVisible 保持一致，容忍 1 秒存储精度误差。
			article.PublishedAtLTE(time.Now().Add(publishTimeTolerance)),
		)
}

// cleanupViewDedupMem 每小时惰性清理一次过期项。sync.Map 本身不回收，
// 不清理会让「Redis 降级模式」下的进程内存随访客 IP 单调增长。
func cleanupViewDedupMem(now int64) {
	last := viewDedupCleanAt.Load()
	if now-last < viewDedupCleanupInterval {
		return
	}
	if !viewDedupCleanAt.CompareAndSwap(last, now) {
		return
	}
	viewDedupMem.Range(func(k, v interface{}) bool {
		if expire, ok := v.(int64); ok && expire <= now {
			viewDedupMem.Delete(k)
		}
		return true
	})
}

func (l *PublicArticleLogic) viewDedupKey(articleID int64) string {
	ip := utils.GetClientIP(l.ctx)
	if ip == "" {
		ip = "unknown"
	}
	sum := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("article:view:%d:%s", articleID, hex.EncodeToString(sum[:8]))
}

// acquireViewDedup 同一 IP 在 24 小时内对同一篇文章只放行一次。
// 优先用 Redis SET NX EX（多实例共享），Redis 不可用时降级到进程内表。
func (l *PublicArticleLogic) acquireViewDedup(key string) bool {
	if l.svcCtx.Redis != nil {
		ok, err := l.svcCtx.Redis.SetnxEx(key, "1", viewDedupSeconds)
		if err == nil {
			return ok
		}
		logx.Errorf("article view dedup via Redis failed, fallback to memory: %v", err)
	}
	now := time.Now().Unix()
	cleanupViewDedupMem(now)
	if v, loaded := viewDedupMem.LoadOrStore(key, now+viewDedupSeconds); loaded {
		if expire, ok := v.(int64); ok && expire > now {
			return false
		}
	}
	viewDedupMem.Store(key, now+viewDedupSeconds)
	return true
}

// releaseViewDedup 落库失败时释放去重占位，否则该访客 24 小时内再也不会被计数。
func (l *PublicArticleLogic) releaseViewDedup(key string) {
	if l.svcCtx.Redis != nil {
		if _, err := l.svcCtx.Redis.Del(key); err != nil {
			logx.Errorf("release article view dedup key failed: %v", err)
		}
	}
	viewDedupMem.Delete(key)
}

// countViewOnce 用 SQL 原子自增替代「读旧值 + 写回」，避免并发访问丢计数。
func (l *PublicArticleLogic) countViewOnce(a *ent.Article) {
	key := l.viewDedupKey(a.ID)
	if !l.acquireViewDedup(key) {
		return
	}
	if err := l.svcCtx.EntClient.Article.UpdateOneID(a.ID).
		AddViewCount(1).Exec(l.ctx); err != nil {
		logx.Errorf("increase article view count failed: %v", err)
		l.releaseViewDedup(key)
		return
	}
	a.ViewCount++
}

// PublicArticleList 前台文章列表，不返回正文
func (l *PublicArticleLogic) PublicArticleList(req *types.PublicArticleListReq) (*types.PublicArticleListResp, error) {
	page, pageSize := utils.NormalizePagination(req.Page, req.PageSize, 10)
	offset, limit := utils.NormalizePage(req.Page, req.PageSize, 10)
	empty := &types.PublicArticleListResp{
		List:     []types.PublicArticleItem{},
		Total:    0,
		Page:     page,
		PageSize: pageSize,
	}

	query := l.visibleQuery()
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		// 全文检索：标题、摘要、正文一起匹配。正文列为 TEXT，MySQL 的
		// LIKE '%kw%' 无法走 B-Tree 索引；数据量上来后应改用 FULLTEXT 或外部搜索引擎。
		query = query.Where(article.Or(
			article.TitleContainsFold(kw),
			article.SummaryContainsFold(kw),
			article.ContentContainsFold(kw),
		))
	}
	if slug := strings.TrimSpace(req.CategorySlug); slug != "" {
		c, err := l.svcCtx.EntClient.Category.Query().
			Where(category.SlugEQ(slug), category.IsActiveEQ(true)).
			Only(l.ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return empty, nil
			}
			logx.Errorf("query public category failed: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		// 分类是树形结构：选父分类时一并展示所有子孙分类下的文章
		ids, err := descendantCategoryIDs(l.ctx, l.svcCtx.EntClient, c.ID)
		if err != nil {
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		query = query.Where(article.CategoryIDIn(ids...))
	}
	if slug := strings.TrimSpace(req.TagSlug); slug != "" {
		query = query.Where(article.HasTagsWith(tag.SlugEQ(slug)))
	}

	total, err := query.Clone().Count(l.ctx)
	if err != nil {
		logx.Errorf("count public articles failed: %v", err)
		return nil, common.NewBizError(common.ErrArticleListFail)
	}
	rows, err := query.
		WithCategory().
		WithTags().
		Order(ent.Desc(article.FieldIsTop), ent.Desc(article.FieldPublishedAt), ent.Desc(article.FieldID)).
		Limit(limit).
		Offset(offset).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list public articles failed: %v", err)
		return nil, common.NewBizError(common.ErrArticleListFail)
	}

	names := lookupAuthorNames(l.ctx, l.svcCtx.EntClient, articleAuthorIDs(rows))
	list := make([]types.PublicArticleItem, 0, len(rows))
	for _, a := range rows {
		list = append(list, publicArticleToItem(a, names))
	}
	return &types.PublicArticleListResp{
		List: list, Total: int64(total), Page: page, PageSize: pageSize,
	}, nil
}

// PublicArticleDetail 前台文章详情，按 slug 访问；返回正文、相关文章与上下篇
func (l *PublicArticleLogic) PublicArticleDetail(req *types.PublicArticleDetailReq) (*types.PublicArticleDetailResp, error) {
	a, err := l.visibleQuery().
		Where(article.SlugEQ(strings.TrimSpace(req.Slug))).
		WithCategory().
		WithTags().
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrArticleNotFound)
		}
		logx.Errorf("get public article failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	l.countViewOnce(a)

	// 相关文章 / 上下篇可能属于别的作者，必须把它们一起纳入批量查名，
	// 否则这些条目的 author_name 恒为空。
	related := l.relatedArticles(a)
	prev, next := l.neighborArticles(a)
	all := make([]*ent.Article, 0, 2+len(related)+2)
	all = append(all, a)
	all = append(all, related...)
	if prev != nil {
		all = append(all, prev)
	}
	if next != nil {
		all = append(all, next)
	}
	names := lookupAuthorNames(l.ctx, l.svcCtx.EntClient, articleAuthorIDs(all))

	base := strings.TrimRight(l.svcCtx.Config.WebBaseURL, "/")
	resp := &types.PublicArticleDetailResp{
		PublicArticleItem: publicArticleToItem(a, names),
		Content:           a.Content,
		SeoTitle:          a.SeoTitle,
		SeoDescription:    a.SeoDescription,
		SeoKeywords:       a.SeoKeywords,
		CanonicalURL:      base + "/articles/" + a.Slug,
		Related:           make([]types.PublicArticleItem, 0, len(related)),
	}
	for _, r := range related {
		resp.Related = append(resp.Related, publicArticleToItem(r, names))
	}
	if prev != nil {
		item := publicArticleToItem(prev, names)
		resp.Prev = &item
	}
	if next != nil {
		item := publicArticleToItem(next, names)
		resp.Next = &item
	}
	return resp, nil
}

// relatedArticles 同分类的其它已发布文章，不足 5 篇时用最新文章补齐
func (l *PublicArticleLogic) relatedArticles(a *ent.Article) []*ent.Article {
	seen := map[int64]bool{a.ID: true}
	result := make([]*ent.Article, 0, 5)
	collect := func(rows []*ent.Article) {
		for _, r := range rows {
			if seen[r.ID] || len(result) >= 5 {
				continue
			}
			seen[r.ID] = true
			result = append(result, r)
		}
	}
	if a.CategoryID != nil {
		rows, err := l.visibleQuery().
			Where(article.CategoryIDEQ(*a.CategoryID), article.IDNEQ(a.ID)).
			WithCategory().WithTags().
			Order(ent.Desc(article.FieldPublishedAt)).Limit(5).All(l.ctx)
		if err == nil {
			collect(rows)
		}
	}
	if len(result) < 5 {
		rows, err := l.visibleQuery().
			Where(article.IDNEQ(a.ID)).
			WithCategory().WithTags().
			Order(ent.Desc(article.FieldPublishedAt)).Limit(5).All(l.ctx)
		if err == nil {
			collect(rows)
		}
	}
	return result
}

// neighborArticles 按发布时间取相邻的前一篇（更早）与后一篇（更晚）
func (l *PublicArticleLogic) neighborArticles(a *ent.Article) (*ent.Article, *ent.Article) {
	if a.PublishedAt == nil {
		return nil, nil
	}
	var prev, next *ent.Article
	older, err := l.visibleQuery().
		Where(article.PublishedAtLT(*a.PublishedAt)).
		WithCategory().WithTags().
		Order(ent.Desc(article.FieldPublishedAt), ent.Desc(article.FieldID)).
		Limit(1).All(l.ctx)
	if err == nil && len(older) > 0 {
		prev = older[0]
	}
	newer, err := l.visibleQuery().
		Where(article.PublishedAtGT(*a.PublishedAt)).
		WithCategory().WithTags().
		Order(ent.Asc(article.FieldPublishedAt), ent.Asc(article.FieldID)).
		Limit(1).All(l.ctx)
	if err == nil && len(newer) > 0 {
		next = newer[0]
	}
	return prev, next
}

// PublicTaxonomy 前台分类与标签导航，只返回至少有一篇公开文章的条目。
//
// 计数改为「固定 4 条 SQL」：全量分类（建树）、启用分类、按 category_id 聚合的
// 公开文章数、按 tag_id 聚合的公开文章数。旧实现对每个分类/标签各执行一次
// COUNT + 一次全表分类扫描，分类数增长时呈 N+1 劣化。
func (l *PublicArticleLogic) PublicTaxonomy() (*types.PublicTaxonomyResp, error) {
	allCats, err := l.svcCtx.EntClient.Category.Query().All(l.ctx)
	if err != nil {
		logx.Errorf("list all categories failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	children := make(map[int64][]int64, len(allCats))
	for _, c := range allCats {
		if c.ParentID != nil {
			children[*c.ParentID] = append(children[*c.ParentID], c.ID)
		}
	}

	cats, err := l.svcCtx.EntClient.Category.Query().
		Where(category.IsActiveEQ(true)).
		Order(ent.Asc(category.FieldSort), ent.Asc(category.FieldID)).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list public categories failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	var catCounts []struct {
		CategoryID int64 `json:"category_id"`
		Count      int   `json:"count"`
	}
	if err := l.visibleQuery().
		Where(article.CategoryIDNotNil()).
		GroupBy(article.FieldCategoryID).
		Aggregate(ent.Count()).
		Scan(l.ctx, &catCounts); err != nil {
		logx.Errorf("aggregate public category counts failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	directCounts := make(map[int64]int64, len(catCounts))
	for _, row := range catCounts {
		directCounts[row.CategoryID] = int64(row.Count)
	}

	catItems := make([]types.PublicCategoryItem, 0, len(cats))
	for _, c := range cats {
		var total int64
		for _, id := range descendantIDsFrom(children, c.ID) {
			total += directCounts[id]
		}
		if total == 0 {
			continue
		}
		item := types.PublicCategoryItem{
			ID: c.ID, Name: c.Name, Slug: c.Slug, ArticleCount: total,
		}
		if c.ParentID != nil {
			item.ParentID = *c.ParentID
		}
		catItems = append(catItems, item)
	}

	tags, err := l.svcCtx.EntClient.Tag.Query().
		Where(tag.IsActiveEQ(true)).
		Order(ent.Asc(tag.FieldSort), ent.Asc(tag.FieldID)).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list public tags failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	var tagCounts []struct {
		TagID int64 `json:"tag_id"`
		Count int   `json:"count"`
	}
	if err := l.svcCtx.EntClient.ArticleTag.Query().
		Where(articletag.HasArticleWith(
			article.StatusEQ(ArticleStatusPublished),
			// published_at 是 datetime(0)，MySQL 会把含纳秒的 time.Now() 四舍五入到下一秒；
			// 严格用 now 判断会让刚发布的文章在 1 秒内对前台不可见，这里与
			// IsPubliclyVisible 保持一致，容忍 1 秒存储精度误差。
			article.PublishedAtLTE(time.Now().Add(publishTimeTolerance)),
			article.DeletedAtIsNil(),
		)).
		GroupBy(articletag.FieldTagID).
		Aggregate(ent.Count()).
		Scan(l.ctx, &tagCounts); err != nil {
		logx.Errorf("aggregate public tag counts failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	tagDirect := make(map[int64]int64, len(tagCounts))
	for _, row := range tagCounts {
		tagDirect[row.TagID] = int64(row.Count)
	}

	tagItems := make([]types.PublicTagItem, 0, len(tags))
	for _, tg := range tags {
		n := tagDirect[tg.ID]
		if n == 0 {
			continue
		}
		tagItems = append(tagItems, types.PublicTagItem{
			ID: tg.ID, Name: tg.Name, Slug: tg.Slug, ArticleCount: n,
		})
	}

	return &types.PublicTaxonomyResp{Categories: catItems, Tags: tagItems}, nil
}

// PublicFeed 供 RSS / sitemap 使用，返回最新的公开文章与站点根地址
func (l *PublicArticleLogic) PublicFeed(limit int) ([]*ent.Article, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := l.visibleQuery().
		Order(ent.Desc(article.FieldPublishedAt)).
		Limit(limit).
		All(l.ctx)
	if err != nil {
		return nil, "", err
	}
	return rows, strings.TrimRight(l.svcCtx.Config.WebBaseURL, "/"), nil
}

func publicArticleToItem(a *ent.Article, names map[int64]string) types.PublicArticleItem {
	item := types.PublicArticleItem{
		ID:          a.ID,
		Title:       a.Title,
		Slug:        a.Slug,
		Summary:     a.Summary,
		Cover:       a.Cover,
		AuthorName:  authorNameOf(names, a),
		TagNames:    make([]string, 0, len(a.Edges.Tags)),
		IsTop:       a.IsTop,
		ViewCount:   a.ViewCount,
		PublishedAt: FormatOptionalTime(a.PublishedAt),
	}
	if a.CategoryID != nil {
		item.CategoryID = *a.CategoryID
	}
	if a.Edges.Category != nil {
		item.CategoryName = a.Edges.Category.Name
		item.CategorySlug = a.Edges.Category.Slug
	}
	for _, tg := range a.Edges.Tags {
		item.TagNames = append(item.TagNames, tg.Name)
	}
	return item
}
