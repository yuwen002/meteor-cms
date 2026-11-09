// Package contentbundle 提供 CMS 内容的可移植 JSON 快照（导出/导入/备份/恢复）。
//
// 设计约定：
//   - 以 slug 作为跨库稳定标识，不导出自增 ID；
//   - 密文配置（is_secret=true）不导出，避免密文与业务数据一起扩散；
//   - 导入是幂等的：已存在按 slug 跳过，传 overwrite=true 时覆盖文章字段；
//   - 整个导入在单事务内完成，任一条失败全部回滚。
package contentbundle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/articletag"
	"github.com/yuwen002/meteor-cms/ent/category"
	"github.com/yuwen002/meteor-cms/ent/sysconfig"
	"github.com/yuwen002/meteor-cms/ent/sysdict"
	"github.com/yuwen002/meteor-cms/ent/sysdictitem"
	"github.com/yuwen002/meteor-cms/ent/tag"
)

// Version 内容包格式版本，格式不兼容时必须递增。
const Version = "1.0"

// MaxBytes 导入时的最大请求体，防止超大 JSON 打爆内存。
const MaxBytes int64 = 32 << 20

const maxItems = 10000

type Category struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	ParentSlug  string `json:"parent_slug,omitempty"`
	Sort        int    `json:"sort"`
	IsActive    bool   `json:"is_active"`
}

type Tag struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Sort        int    `json:"sort"`
	IsActive    bool   `json:"is_active"`
}

type Article struct {
	Title          string   `json:"title"`
	Slug           string   `json:"slug"`
	Summary        string   `json:"summary,omitempty"`
	Cover          string   `json:"cover,omitempty"`
	Content        string   `json:"content,omitempty"`
	Status         int      `json:"status"`
	CategorySlug   string   `json:"category_slug,omitempty"`
	TagSlugs       []string `json:"tag_slugs,omitempty"`
	PublishedAt    string   `json:"published_at,omitempty"`
	IsTop          bool     `json:"is_top"`
	ViewCount      int64    `json:"view_count"`
	SeoTitle       string   `json:"seo_title,omitempty"`
	SeoDescription string   `json:"seo_description,omitempty"`
	SeoKeywords    string   `json:"seo_keywords,omitempty"`
}

type Config struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	Remark     string `json:"remark,omitempty"`
	Group      string `json:"group,omitempty"`
	ValueType  string `json:"value_type,omitempty"`
	Validation string `json:"validation,omitempty"`
	Sort       int    `json:"sort"`
	Status     int    `json:"status"`
	IsSystem   bool   `json:"is_system"`
}

type DictItem struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Remark      string `json:"remark,omitempty"`
	ParentValue string `json:"parent_value,omitempty"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	IsDefault   bool   `json:"is_default"`
}

type Dict struct {
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Group       string     `json:"group,omitempty"`
	Status      int        `json:"status"`
	Sort        int        `json:"sort"`
	IsSystem    bool       `json:"is_system"`
	Items       []DictItem `json:"items,omitempty"`
}

// Bundle 是一次完整的内容快照。
type Bundle struct {
	Version    string     `json:"version"`
	ExportedAt string     `json:"exported_at"`
	Categories []Category `json:"categories"`
	Tags       []Tag      `json:"tags"`
	Articles   []Article  `json:"articles"`
	Configs    []Config   `json:"configs"`
	Dicts      []Dict     `json:"dicts"`
}

// Result 汇总导入结果，便于前端展示与运维核对。
type Result struct {
	CategoriesCreated int `json:"categories_created"`
	CategoriesSkipped int `json:"categories_skipped"`
	TagsCreated       int `json:"tags_created"`
	TagsSkipped       int `json:"tags_skipped"`
	ArticlesCreated   int `json:"articles_created"`
	ArticlesUpdated   int `json:"articles_updated"`
	ArticlesSkipped   int `json:"articles_skipped"`
	ConfigsCreated    int `json:"configs_created"`
	ConfigsSkipped    int `json:"configs_skipped"`
	DictsCreated      int `json:"dicts_created"`
	DictsSkipped      int `json:"dicts_skipped"`
	DictItemsCreated  int `json:"dict_items_created"`
	DictItemsSkipped  int `json:"dict_items_skipped"`
}

// Validate 拒绝格式错误 / 重复 slug / 超大内容包，避免半途失败留下脏数据。
func (b *Bundle) Validate() error {
	if b == nil {
		return errors.New("内容包为空")
	}
	if b.Version != Version {
		return fmt.Errorf("不支持的内容包版本 %q（期望 %q）", b.Version, Version)
	}
	if len(b.Categories)+len(b.Tags)+len(b.Articles)+len(b.Configs)+len(b.Dicts) > maxItems*5 {
		return errors.New("内容包条目过多")
	}
	check := func(items []string, label string) error {
		seen := make(map[string]bool, len(items))
		for _, v := range items {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("%s 存在空 slug", label)
			}
			if seen[v] {
				return fmt.Errorf("%s slug 重复: %s", label, v)
			}
			seen[v] = true
		}
		return nil
	}
	catSlugs := make([]string, 0, len(b.Categories))
	for _, c := range b.Categories {
		catSlugs = append(catSlugs, c.Slug)
	}
	if err := check(catSlugs, "分类"); err != nil {
		return err
	}
	tagSlugs := make([]string, 0, len(b.Tags))
	for _, t := range b.Tags {
		tagSlugs = append(tagSlugs, t.Slug)
	}
	if err := check(tagSlugs, "标签"); err != nil {
		return err
	}
	artSlugs := make([]string, 0, len(b.Articles))
	for _, a := range b.Articles {
		if strings.TrimSpace(a.Title) == "" {
			return fmt.Errorf("文章 %s 标题为空", a.Slug)
		}
		artSlugs = append(artSlugs, a.Slug)
	}
	return check(artSlugs, "文章")
}

// Decode 从有界 Reader 解析内容包。
func Decode(r io.Reader) (*Bundle, error) {
	var b Bundle
	dec := json.NewDecoder(io.LimitReader(r, MaxBytes))
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("解析内容包失败: %w", err)
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &b, nil
}

// Write 以缩进 JSON 输出内容包，便于人工审阅与版本管理。
func Write(w io.Writer, b *Bundle) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(b)
}

func timeString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

// Export 读取全部内容（软删除行自动排除），生成可移植快照。
func Export(ctx context.Context, client *ent.Client) (*Bundle, error) {
	b := &Bundle{
		Version:    Version,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Categories: []Category{}, Tags: []Tag{}, Articles: []Article{},
		Configs: []Config{}, Dicts: []Dict{},
	}

	cats, err := client.Category.Query().Order(ent.Asc(category.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	catSlugByID := make(map[int64]string, len(cats))
	for _, c := range cats {
		parent := ""
		if c.ParentID != nil {
			parent = catSlugByID[*c.ParentID]
		}
		catSlugByID[c.ID] = c.Slug
		b.Categories = append(b.Categories, Category{
			Name: c.Name, Slug: c.Slug, Description: c.Description,
			ParentSlug: parent, Sort: c.Sort, IsActive: c.IsActive,
		})
	}

	tags, err := client.Tag.Query().Order(ent.Asc(tag.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	tagSlugByID := make(map[int64]string, len(tags))
	for _, t := range tags {
		tagSlugByID[t.ID] = t.Slug
		b.Tags = append(b.Tags, Tag{
			Name: t.Name, Slug: t.Slug, Description: t.Description,
			Sort: t.Sort, IsActive: t.IsActive,
		})
	}

	links, err := client.ArticleTag.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	articleTags := make(map[int64][]string)
	for _, l := range links {
		if slug, ok := tagSlugByID[l.TagID]; ok {
			articleTags[l.ArticleID] = append(articleTags[l.ArticleID], slug)
		}
	}

	articles, err := client.Article.Query().Order(ent.Asc(article.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range articles {
		catSlug := ""
		if a.CategoryID != nil {
			catSlug = catSlugByID[*a.CategoryID]
		}
		b.Articles = append(b.Articles, Article{
			Title: a.Title, Slug: a.Slug, Summary: a.Summary, Cover: a.Cover,
			Content: a.Content, Status: a.Status, CategorySlug: catSlug,
			TagSlugs: articleTags[a.ID], PublishedAt: timeString(a.PublishedAt),
			IsTop: a.IsTop, ViewCount: a.ViewCount,
			SeoTitle: a.SeoTitle, SeoDescription: a.SeoDescription, SeoKeywords: a.SeoKeywords,
		})
	}

	// 密文配置不导出：密文本身也是敏感资产，且跨实例密钥不同无法解密
	configs, err := client.SysConfig.Query().
		Where(sysconfig.IsSecretEQ(false)).
		Order(ent.Asc(sysconfig.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range configs {
		b.Configs = append(b.Configs, Config{
			Key: c.ConfigKey, Name: c.Name, Value: c.ConfigValue, Remark: c.Remark,
			Group: c.ConfigGroup, ValueType: c.ValueType, Validation: c.Validation,
			Sort: c.Sort, Status: c.Status, IsSystem: c.IsSystem,
		})
	}

	dicts, err := client.SysDict.Query().Order(ent.Asc(sysdict.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	items, err := client.SysDictItem.Query().
		Order(ent.Asc(sysdictitem.FieldSort), ent.Asc(sysdictitem.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	itemsByCode := make(map[string][]*ent.SysDictItem)
	for _, it := range items {
		itemsByCode[it.DictCode] = append(itemsByCode[it.DictCode], it)
	}
	valueByID := make(map[int64]string, len(items))
	for _, it := range items {
		valueByID[it.ID] = it.Value
	}
	for _, d := range dicts {
		out := Dict{
			Code: d.Code, Name: d.Name, Description: d.Description, Group: d.DictGroup,
			Status: d.Status, Sort: d.Sort, IsSystem: d.IsSystem, Items: []DictItem{},
		}
		for _, it := range itemsByCode[d.Code] {
			parent := ""
			if it.ParentID != 0 {
				parent = valueByID[it.ParentID]
			}
			out.Items = append(out.Items, DictItem{
				Label: it.Label, Value: it.Value, Remark: it.Remark, ParentValue: parent,
				Sort: it.Sort, Status: it.Status, IsDefault: it.IsDefault,
			})
		}
		b.Dicts = append(b.Dicts, out)
	}
	return b, nil
}

// Import 在单事务内导入内容包；overwrite=true 时覆盖同 slug 文章。
func Import(ctx context.Context, client *ent.Client, b *Bundle, overwrite bool) (*Result, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	res := &Result{}

	catIDs := make(map[string]int64, len(b.Categories))
	for _, c := range b.Categories {
		existing, err := tx.Category.Query().Where(category.SlugEQ(c.Slug)).Only(ctx)
		if err == nil {
			catIDs[c.Slug] = existing.ID
			res.CategoriesSkipped++
			continue
		}
		if !ent.IsNotFound(err) {
			return nil, err
		}
		created, err := tx.Category.Create().
			SetName(c.Name).SetSlug(c.Slug).SetDescription(c.Description).
			SetSort(c.Sort).SetIsActive(c.IsActive).Save(ctx)
		if err != nil {
			return nil, err
		}
		catIDs[c.Slug] = created.ID
		res.CategoriesCreated++
	}
	// 父分类可能排在子分类之后，必须两遍处理
	for _, c := range b.Categories {
		if c.ParentSlug == "" {
			continue
		}
		parentID, ok := catIDs[c.ParentSlug]
		if !ok {
			continue
		}
		if _, err := tx.Category.UpdateOneID(catIDs[c.Slug]).SetParentID(parentID).Save(ctx); err != nil {
			return nil, err
		}
	}

	tagIDs := make(map[string]int64, len(b.Tags))
	for _, t := range b.Tags {
		existing, err := tx.Tag.Query().Where(tag.SlugEQ(t.Slug)).Only(ctx)
		if err == nil {
			tagIDs[t.Slug] = existing.ID
			res.TagsSkipped++
			continue
		}
		if !ent.IsNotFound(err) {
			return nil, err
		}
		created, err := tx.Tag.Create().
			SetName(t.Name).SetSlug(t.Slug).SetDescription(t.Description).
			SetSort(t.Sort).SetIsActive(t.IsActive).Save(ctx)
		if err != nil {
			return nil, err
		}
		tagIDs[t.Slug] = created.ID
		res.TagsCreated++
	}

	for _, a := range b.Articles {
		categoryID := int64(0)
		if a.CategorySlug != "" {
			categoryID = catIDs[a.CategorySlug]
		}
		published, err := parseTime(a.PublishedAt)
		if err != nil {
			return nil, fmt.Errorf("文章 %s 的 published_at 非法: %w", a.Slug, err)
		}
		existing, err := tx.Article.Query().Where(article.SlugEQ(a.Slug)).Only(ctx)
		var articleID int64
		switch {
		case err == nil:
			articleID = existing.ID
			if !overwrite {
				res.ArticlesSkipped++
				continue
			}
			upd := tx.Article.UpdateOneID(articleID).
				SetTitle(a.Title).SetSummary(a.Summary).SetCover(a.Cover).SetContent(a.Content).
				SetStatus(a.Status).SetIsTop(a.IsTop).SetViewCount(a.ViewCount).
				SetSeoTitle(a.SeoTitle).SetSeoDescription(a.SeoDescription).SetSeoKeywords(a.SeoKeywords)
			if categoryID > 0 {
				upd = upd.SetCategoryID(categoryID)
			} else {
				upd = upd.ClearCategoryID()
			}
			if !published.IsZero() {
				upd = upd.SetPublishedAt(published)
			} else {
				upd = upd.ClearPublishedAt()
			}
			if _, err := upd.Save(ctx); err != nil {
				return nil, err
			}
			res.ArticlesUpdated++
		case ent.IsNotFound(err):
			create := tx.Article.Create().
				SetTitle(a.Title).SetSlug(a.Slug).SetSummary(a.Summary).SetCover(a.Cover).
				SetContent(a.Content).SetStatus(a.Status).SetIsTop(a.IsTop).
				SetViewCount(a.ViewCount).SetSeoTitle(a.SeoTitle).
				SetSeoDescription(a.SeoDescription).SetSeoKeywords(a.SeoKeywords)
			if categoryID > 0 {
				create = create.SetCategoryID(categoryID)
			}
			if !published.IsZero() {
				create = create.SetPublishedAt(published)
			}
			created, err := create.Save(ctx)
			if err != nil {
				return nil, err
			}
			articleID = created.ID
			res.ArticlesCreated++
		default:
			return nil, err
		}

		// 重建标签关联，保证导入结果与快照一致
		if _, err := tx.ArticleTag.Delete().Where(articletag.ArticleIDEQ(articleID)).Exec(ctx); err != nil {
			return nil, err
		}
		for _, slug := range a.TagSlugs {
			tid, ok := tagIDs[slug]
			if !ok {
				continue
			}
			if _, err := tx.ArticleTag.Create().SetArticleID(articleID).SetTagID(tid).Save(ctx); err != nil {
				return nil, err
			}
		}
	}

	for _, c := range b.Configs {
		exists, err := tx.SysConfig.Query().Where(sysconfig.ConfigKeyEQ(c.Key)).Exist(ctx)
		if err != nil {
			return nil, err
		}
		if exists {
			res.ConfigsSkipped++
			continue
		}
		group := c.Group
		if group == "" {
			group = "general"
		}
		valueType := c.ValueType
		if valueType == "" {
			valueType = "string"
		}
		if _, err := tx.SysConfig.Create().
			SetConfigKey(c.Key).SetConfigValue(c.Value).SetName(c.Name).SetRemark(c.Remark).
			SetConfigGroup(group).SetValueType(valueType).SetValidation(c.Validation).
			SetSort(c.Sort).SetStatus(c.Status).SetIsSystem(c.IsSystem).
			Save(ctx); err != nil {
			return nil, err
		}
		res.ConfigsCreated++
	}

	for _, d := range b.Dicts {
		existing, err := tx.SysDict.Query().Where(sysdict.CodeEQ(d.Code)).Only(ctx)
		if err == nil {
			res.DictsSkipped++
			_ = existing
		} else {
			if !ent.IsNotFound(err) {
				return nil, err
			}
			if _, err := tx.SysDict.Create().
				SetCode(d.Code).SetName(d.Name).SetDescription(d.Description).
				SetDictGroup(d.Group).SetStatus(d.Status).SetSort(d.Sort).
				SetIsSystem(d.IsSystem).Save(ctx); err != nil {
				return nil, err
			}
			res.DictsCreated++
		}
		itemIDs := make(map[string]int64, len(d.Items))
		for _, it := range d.Items {
			found, err := tx.SysDictItem.Query().
				Where(sysdictitem.DictCodeEQ(d.Code), sysdictitem.ValueEQ(it.Value)).
				Only(ctx)
			if err == nil {
				itemIDs[it.Value] = found.ID
				res.DictItemsSkipped++
				continue
			}
			if !ent.IsNotFound(err) {
				return nil, err
			}
			created, err := tx.SysDictItem.Create().
				SetDictCode(d.Code).SetLabel(it.Label).SetValue(it.Value).SetRemark(it.Remark).
				SetSort(it.Sort).SetStatus(it.Status).SetIsDefault(it.IsDefault).
				Save(ctx)
			if err != nil {
				return nil, err
			}
			itemIDs[it.Value] = created.ID
			res.DictItemsCreated++
		}
		for _, it := range d.Items {
			if it.ParentValue == "" {
				continue
			}
			parentID, ok := itemIDs[it.ParentValue]
			if !ok {
				continue
			}
			if _, err := tx.SysDictItem.UpdateOneID(itemIDs[it.Value]).SetParentID(parentID).Save(ctx); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return res, nil
}
