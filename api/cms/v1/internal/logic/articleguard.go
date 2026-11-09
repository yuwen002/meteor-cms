package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	entschema "github.com/yuwen002/meteor-cms/ent/schema"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/permcheck"
	"github.com/yuwen002/meteor-cms/internal/utils"
)

// articleReviewPermissions 持有任一权限的用户可以跨作者查看文章。
// 审核员必须能看到别人的待审文章，否则审核流无法工作；权限码与 internal/seed
// 中登记的按钮/API 权限保持一致。
// 只放行真正需要审他人文章的 approve/reject；cms:article:review 仅表示
// 「查看审核记录」，普通作者也会用它查看自己文章的轨迹，不能据此放开跨作者读取。
var articleReviewPermissions = []string{
	"cms:article:approve", "cms:article:reject",
}

// articleReadScope 返回当前用户可见文章的作者 ID；nil 表示不做行级过滤
// （超级管理员或审核角色）。
//
// 权限集合直接复用中间件注入 ctx 的快照，既避免每次请求再查三张表，也保证
// 「接口鉴权」与「行级判定」使用同一份结论。缺少快照时 fail-closed 只放行本人文章。
func articleReadScope(ctx context.Context) (*int64, error) {
	if utils.IsSuperAdmin(ctx) {
		return nil, nil
	}
	userID, ok := utils.CurrentUserID(ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}
	perms, ok := permcheck.PermissionsFrom(ctx)
	if ok && permcheck.HasAny(perms, articleReviewPermissions...) {
		return nil, nil
	}
	return &userID, nil
}

// articleQueryVisible 给文章列表（含其 Count 克隆）叠加行级过滤。
func articleQueryVisible(ctx context.Context, q *ent.ArticleQuery) (*ent.ArticleQuery, error) {
	scope, err := articleReadScope(ctx)
	if err != nil {
		return nil, err
	}
	if scope != nil {
		q = q.Where(article.AuthorIDEQ(*scope))
	}
	return q, nil
}

// ensureArticleReadable 校验单篇文章是否允许当前用户读取。
func ensureArticleReadable(ctx context.Context, a *ent.Article) error {
	scope, err := articleReadScope(ctx)
	if err != nil {
		return err
	}
	if scope != nil && (a.AuthorID == nil || *a.AuthorID != *scope) {
		return common.NewBizError(common.ErrArticleReadForbidden)
	}
	return nil
}

// loadArticleForRead 按 ID 读取文章用于权限判定；includeDeleted 供回收站场景使用。
func loadArticleForRead(ctx context.Context, client *ent.Client, id int64, includeDeleted bool) (*ent.Article, error) {
	queryCtx := ctx
	if includeDeleted {
		queryCtx = entschema.SkipSoftDelete(ctx)
	}
	a, err := client.Article.Query().
		Where(article.IDEQ(id)).
		Only(queryCtx)
	if err != nil {
		return nil, articleGetError(err)
	}
	return a, nil
}

// ensureArticleIDReadable 按文章 ID 校验读权限，供审核记录、修订历史等
// 只有 article_id 而没有文章实体的接口复用。
func ensureArticleIDReadable(ctx context.Context, client *ent.Client, id int64, includeDeleted bool) error {
	a, err := loadArticleForRead(ctx, client, id, includeDeleted)
	if err != nil {
		return err
	}
	return ensureArticleReadable(ctx, a)
}
