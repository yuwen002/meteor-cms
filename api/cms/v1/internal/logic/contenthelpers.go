package logic

import (
	"context"
	"strings"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/ent/slugredirect"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

// upsertSlugHistory 在 slug 变更或软删除前记录旧地址，供 301 与回收站恢复使用。
// (entity_type, old_slug) 有唯一索引：slug 被重新分配给别的实体时，同键历史记录
// 会被覆盖，因此先删再插，避免唯一键冲突导致整个事务失败。
func upsertSlugHistory(ctx context.Context, tx *ent.Tx, entityType string, entityID int64, oldSlug string) error {
	if oldSlug == "" || IsTrashSlug(oldSlug) {
		return nil
	}
	if _, err := tx.SlugRedirect.Delete().
		Where(slugredirect.EntityTypeEQ(entityType), slugredirect.OldSlugEQ(oldSlug)).
		Exec(ctx); err != nil {
		return err
	}
	return tx.SlugRedirect.Create().
		SetEntityType(entityType).
		SetEntityID(entityID).
		SetOldSlug(oldSlug).
		Exec(ctx)
}

// articleAuthorIDs 收集文章列表里出现的作者 ID（去重）
func articleAuthorIDs(rows []*ent.Article) []int64 {
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	for _, a := range rows {
		if a.AuthorID != nil && !seen[*a.AuthorID] {
			seen[*a.AuthorID] = true
			ids = append(ids, *a.AuthorID)
		}
	}
	return ids
}

// lookupAuthorNames 批量查作者显示名，查询失败时降级为空名而不是让整个请求失败
func lookupAuthorNames(ctx context.Context, client *ent.Client, ids []int64) map[int64]string {
	out := make(map[int64]string)
	if len(ids) == 0 {
		return out
	}
	users, err := client.AdminUser.Query().Where(adminuser.IDIn(ids...)).All(ctx)
	if err != nil {
		logx.Errorf("lookup article authors failed: %v", err)
		return out
	}
	for _, u := range users {
		out[u.ID] = FirstNonEmpty(u.Nickname, u.Username)
	}
	return out
}

func authorNameOf(names map[int64]string, a *ent.Article) string {
	if a.AuthorID == nil {
		return ""
	}
	return names[*a.AuthorID]
}

// descendantIDsFrom 在已构造好的 children 映射上做 BFS，带环检测，避免脏数据造成死循环。
func descendantIDsFrom(children map[int64][]int64, rootID int64) []int64 {
	result := []int64{rootID}
	seen := map[int64]bool{rootID: true}
	queue := []int64{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			if !seen[child] {
				seen[child] = true
				result = append(result, child)
				queue = append(queue, child)
			}
		}
	}
	return result
}

// descendantCategoryIDs 返回 rootID 及其所有子孙分类 ID，用于前台按父分类筛选时
// 一并展示子分类下的文章。一次取全量分类在内存里做 BFS，避免递归查询。
func descendantCategoryIDs(ctx context.Context, client *ent.Client, rootID int64) ([]int64, error) {
	all, err := client.Category.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	children := make(map[int64][]int64)
	for _, c := range all {
		if c.ParentID != nil {
			children[*c.ParentID] = append(children[*c.ParentID], c.ID)
		}
	}
	return descendantIDsFrom(children, rootID), nil
}

const (
	ReviewActionSubmit  = 1
	ReviewActionApprove = 2
	ReviewActionReject  = 3
)

func ReviewActionText(action int) string {
	switch action {
	case ReviewActionSubmit:
		return "提交审核"
	case ReviewActionApprove:
		return "审核通过"
	case ReviewActionReject:
		return "驳回"
	default:
		return "未知"
	}
}

// currentOperator 取当前登录管理员的 ID 与名称快照。
// 名称冗余存进审核记录，管理员之后改名或离职也不会让历史记录失去可读性。
func currentOperator(ctx context.Context, client *ent.Client) (int64, string) {
	uid, ok := utils.CurrentUserID(ctx)
	if !ok || uid <= 0 {
		return 0, "系统"
	}
	u, err := client.AdminUser.Get(ctx, uid)
	if err != nil {
		logx.Errorf("load current operator %d failed: %v", uid, err)
		return uid, ""
	}
	if u.Nickname != "" {
		return uid, u.Nickname
	}
	return uid, u.Username
}

// writeReview 必须在状态变更的同一个事务里调用：状态改了但记录没写成功时整体回滚。
func writeReview(ctx context.Context, tx *ent.Tx, articleID int64, action int, remark string, operatorID int64, operatorName string) error {
	return tx.ArticleReview.Create().
		SetArticleID(articleID).
		SetAction(action).
		SetRemark(strings.TrimSpace(remark)).
		SetOperatorID(operatorID).
		SetOperatorName(operatorName).
		Exec(ctx)
}
