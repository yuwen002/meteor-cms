package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LoginLog 登录日志表，记录登录成功与失败，用于账号安全审计
type LoginLog struct {
	ent.Schema
}

// Annotations 表和字段的额外注解
func (LoginLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
		schema.Comment("登录日志表，记录登录成功与失败尝试"),
	}
}

// Fields 定义 LoginLog 实体字段
func (LoginLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Comment("自增主键 ID").
			Unique().
			Positive().
			Immutable(),
		field.Int64("user_id").
			Optional().
			Comment("管理员ID，登录失败且账号不存在时为空"),
		field.String("username").
			MaxLen(50).
			Comment("尝试登录的账号"),
		field.String("ip").
			MaxLen(45).
			Optional().
			Comment("来源IP，兼容 IPv6"),
		field.String("user_agent").
			MaxLen(512).
			Optional().
			Comment("客户端标识"),
		field.Int("status").
			Default(0).
			Comment("登录结果：0 成功，1 失败"),
		field.String("fail_reason").
			MaxLen(255).
			Optional().
			Comment("失败原因"),
		field.Time("login_at").
			Default(time.Now).
			Comment("登录时间"),
		field.String("request_id").
			MaxLen(64).
			Optional().
			Comment("请求链路标识，用于串联登录日志与操作日志"),
	}
}

// Mixin 复用创建时间、更新时间等通用字段。
// 注意：登录日志刻意不继承 SoftDeleteMixin，保证记录不可被软删除/篡改，满足审计完整性。
func (LoginLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		BaseMixin{},
	}
}

// Indexes 定义索引
func (LoginLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("username"),
		index.Fields("login_at"),
		index.Fields("request_id"),
		index.Fields("user_id", "login_at"),
		index.Fields("status", "login_at"),
	}
}

// Edges 暂不定义关联，避免级联删除把日志一起删掉
func (LoginLog) Edges() []ent.Edge {
	return nil
}
