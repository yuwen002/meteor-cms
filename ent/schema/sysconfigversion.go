package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SysConfigVersion 记录配置值的每一次变更，支持审计追溯与按版本回滚。
// 不做软删除：变更历史属于审计证据，只能随系统保留策略清理。
type SysConfigVersion struct {
	ent.Schema
}

func (SysConfigVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (SysConfigVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Comment("自增主键 ID").
			Unique().
			Positive().
			Immutable(),

		field.Int64("config_id").
			Comment("关联配置项 ID"),

		field.String("config_key").
			MaxLen(128).
			Comment("配置键快照，配置项被删除后仍可追溯"),

		field.Text("old_value").
			Optional().
			Comment("变更前的值（密文配置存密文）"),

		field.Text("new_value").
			Optional().
			Comment("变更后的值（密文配置存密文）"),

		field.Int64("operator_id").
			Default(0).
			Comment("操作人 ID"),

		field.String("operator_name").
			MaxLen(64).
			Optional().
			Comment("操作人账号"),
	}
}

func (SysConfigVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_id", "created_at"),
		index.Fields("config_key"),
	}
}

func (SysConfigVersion) Edges() []ent.Edge {
	return nil
}
