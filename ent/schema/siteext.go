package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// 站点级扩展模块：友链、评论、邮件订阅。
// 三个模块都通过 SysConfig 的功能开关控制，关闭时公开接口返回空/拒绝写入，
// 后台管理接口始终可用，便于先配置再开放。

// FriendLink 友情链接
type FriendLink struct {
	ent.Schema
}

func (FriendLink) Mixin() []ent.Mixin { return []ent.Mixin{BaseMixin{}} }

func (FriendLink) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "friend_links"},
		entsql.WithComments(true),
		schema.Comment("友情链接表"),
	}
}

func (FriendLink) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Positive().Immutable().Comment("自增主键 ID"),
		field.String("name").MaxLen(100).NotEmpty().Comment("站点名称"),
		field.String("url").MaxLen(512).NotEmpty().Comment("站点地址"),
		field.String("logo").MaxLen(512).Default("").Comment("Logo URL"),
		field.String("description").MaxLen(255).Default("").Comment("站点简介"),
		field.String("link_type").MaxLen(32).Default("friend").Comment("链接类型，取自字典 link_type"),
		field.Int("sort").Default(0).Comment("排序值，越小越靠前"),
		field.Int("status").Default(1).Comment("1 启用 / 0 停用"),
	}
}

func (FriendLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "sort"),
		index.Fields("link_type"),
	}
}

// Comment 文章评论（支持一级回复与审核）
type Comment struct {
	ent.Schema
}

func (Comment) Mixin() []ent.Mixin { return []ent.Mixin{BaseMixin{}} }

func (Comment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "comments"},
		entsql.WithComments(true),
		schema.Comment("文章评论表"),
	}
}

func (Comment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Positive().Immutable().Comment("自增主键 ID"),
		field.Int64("article_id").Positive().Comment("文章 ID"),
		field.Int64("parent_id").Default(0).Comment("父评论 ID，0 表示顶层评论"),
		field.String("author").MaxLen(64).NotEmpty().Comment("显示昵称"),
		field.String("email").MaxLen(128).Default("").Comment("联系邮箱，仅后台可见"),
		field.Text("content").NotEmpty().Comment("评论正文"),
		field.Int("status").Default(0).Comment("0 待审 / 1 通过 / 2 拒绝"),
		field.String("ip").MaxLen(64).Default("").Comment("来源 IP"),
	}
}

func (Comment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("article_id", "status"),
		index.Fields("parent_id"),
		index.Fields("status", "created_at"),
	}
}

// Subscriber 邮件订阅
type Subscriber struct {
	ent.Schema
}

func (Subscriber) Mixin() []ent.Mixin { return []ent.Mixin{BaseMixin{}} }

func (Subscriber) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscribers"},
		entsql.WithComments(true),
		schema.Comment("邮件订阅表"),
	}
}

func (Subscriber) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Positive().Immutable().Comment("自增主键 ID"),
		field.String("email").MaxLen(128).NotEmpty().Unique().Comment("订阅邮箱"),
		field.Int("status").Default(1).Comment("1 订阅中 / 0 已退订"),
		field.String("source").MaxLen(64).Default("site").Comment("订阅来源"),
	}
}

func (Subscriber) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
	}
}
