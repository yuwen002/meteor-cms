package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/articletag"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/core/logx"
)

// categoryArticleCounts 一次 GroupBy 得到各分类的文章数，替代逐条 COUNT 的 N+1。
// 口径与后台文章列表一致：软删除文章由拦截器排除，非超管/非审核角色按 author_id 过滤。
func categoryArticleCounts(ctx context.Context, client *ent.Client) (map[int64]int64, error) {
	scope, err := articleReadScope(ctx)
	if err != nil {
		return nil, err
	}
	q := client.Article.Query().Where(article.CategoryIDNotNil())
	if scope != nil {
		q = q.Where(article.AuthorIDEQ(*scope))
	}
	var rows []struct {
		CategoryID int64 `json:"category_id"`
		Count      int   `json:"count"`
	}
	if err := q.GroupBy(article.FieldCategoryID).Aggregate(ent.Count()).Scan(ctx, &rows); err != nil {
		logx.Errorf("aggregate category article counts failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		out[row.CategoryID] = int64(row.Count)
	}
	return out, nil
}

// tagArticleCounts 按 article_tags 聚合标签文章数。关联表本身不软删除，
// 因此必须显式排除已软删除的文章，再叠加行级作者过滤。
func tagArticleCounts(ctx context.Context, client *ent.Client) (map[int64]int64, error) {
	scope, err := articleReadScope(ctx)
	if err != nil {
		return nil, err
	}
	q := client.ArticleTag.Query().
		Where(articletag.HasArticleWith(article.DeletedAtIsNil()))
	if scope != nil {
		q = q.Where(articletag.HasArticleWith(article.AuthorIDEQ(*scope)))
	}
	var rows []struct {
		TagID int64 `json:"tag_id"`
		Count int   `json:"count"`
	}
	if err := q.GroupBy(articletag.FieldTagID).Aggregate(ent.Count()).Scan(ctx, &rows); err != nil {
		logx.Errorf("aggregate tag article counts failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		out[row.TagID] = int64(row.Count)
	}
	return out, nil
}
