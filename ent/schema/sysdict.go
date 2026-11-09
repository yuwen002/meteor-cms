package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SysDict 数据字典类型表（如 user_status、article_type）
type SysDict struct {
	ent.Schema
}

func (SysDict) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
		schema.Comment("数据字典类型表，用于维护系统中可枚举的业务字典"),
	}
}

func (SysDict) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (SysDict) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Comment("自增主键 ID").
			Unique().
			Positive().
			Immutable(),

		field.String("code").
			MaxLen(64).
			NotEmpty().
			Unique().
			Comment("字典编码，全局唯一，如 user_status"),

		field.String("name").
			MaxLen(128).
			NotEmpty().
			Comment("字典名称"),

		field.String("description").
			MaxLen(512).
			Optional().
			Comment("字典描述"),

		field.Int("status").
			Default(1).
			Comment("状态：1 启用 0 禁用"),

		field.Int("sort").
			Default(0).
			Comment("排序，从小到大"),

		field.String("dict_group").
			MaxLen(64).
			Default("general").
			Comment("字典分组，便于按业务域归类"),

		field.Bool("is_system").
			Default(false).
			Comment("是否系统内置：内置字典禁止删除"),
	}
}

func (SysDict) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique(),
		index.Fields("status"),
		index.Fields("dict_group"),
	}
}

func (SysDict) Edges() []ent.Edge {
	return nil
}
