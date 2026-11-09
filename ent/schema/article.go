package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Article 文章表。正文统一存 Markdown 源码，前台渲染前做转义与白名单转换。
type Article struct {
	ent.Schema
}

func (Article) Mixin() []ent.Mixin {
	return []ent.Mixin{
		BaseMixin{},
		SoftDeleteMixin{},
	}
}

func (Article) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "articles"},
		entsql.WithComments(true),
		schema.Comment("文章表，正文为 Markdown 源码"),
	}
}

func (Article) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable().
			Comment("自增主键 ID"),

		field.String("title").
			MaxLen(255).
			NotEmpty().
			Comment("文章标题"),

		field.String("slug").
			MaxLen(191).
			NotEmpty().
			Unique().
			Comment("URL 友好标识，全局唯一，用于前台文章地址"),

		field.String("summary").
			MaxLen(512).
			Default("").
			Comment("摘要，为空时前台取正文开头"),

		field.String("cover").
			MaxLen(512).
			Default("").
			Comment("封面图 URL，来自 /admin/upload"),

		field.Text("content").
			Optional().
			Comment("正文 Markdown 源码"),

		field.Int("status").
			Default(0).
			Comment("状态：0 草稿 1 已发布 2 已下线"),

		field.Int64("category_id").
			Optional().
			Nillable().
			Comment("所属分类 ID"),

		field.Int64("author_id").
			Optional().
			Nillable().
			Comment("创建人管理员 ID"),

		field.Time("published_at").
			Optional().
			Nillable().
			Comment("发布/定时发布时间；公开可见需满足 status=1 且该时间不晚于当前时间"),

		field.Int64("view_count").
			Default(0).
			NonNegative().
			Comment("浏览量"),

		field.Bool("is_top").
			Default(false).
			Comment("是否置顶"),

		field.String("seo_title").
			MaxLen(255).
			Default("").
			Comment("SEO 标题，为空时取文章标题"),

		field.String("seo_description").
			MaxLen(512).
			Default("").
			Comment("SEO 描述，为空时取摘要"),

		field.String("seo_keywords").
			MaxLen(255).
			Default("").
			Comment("SEO 关键词，逗号分隔"),
	}
}

func (Article) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "published_at"),
		index.Fields("category_id"),
		index.Fields("is_top"),
		// 软删除拦截器会给每个查询追加 deleted_at IS NULL，把它放进联合索引
		// 的最左列，列表查询与排序才能走索引而不是全表 + filesort。
		index.Fields("deleted_at", "status", "published_at", "id"),
		index.Fields("deleted_at", "is_top", "id"),
		index.Fields("deleted_at", "category_id", "status"),
		// 行级数据权限：非超管列表按 author_id 过滤，deleted_at 仍作最左列，
		// 避免补过滤后退化成全表扫描。
		index.Fields("deleted_at", "author_id", "status", "id"),
	}
}

func (Article) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("category", Category.Type).
			Ref("articles").
			Field("category_id").
			Unique().
			Annotations(entsql.OnDelete(entsql.SetNull)),

		edge.To("tags", Tag.Type).
			Through("article_tags", ArticleTag.Type),
	}
}
