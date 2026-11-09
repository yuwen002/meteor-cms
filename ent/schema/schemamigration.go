package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SchemaMigration 记录每次 cmd/migrate 成功应用的 schema 版本。
// API 启动时读取最新版本并校验，避免「忘跑迁移 → 启动看似成功、运行时才报 SQL 错误」。
// 这张表不使用 BaseMixin：迁移历史只追加，不需要软删除与业务更新时间。
type SchemaMigration struct {
	ent.Schema
}

func (SchemaMigration) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "schema_migrations"},
		entsql.WithComments(true),
		schema.Comment("数据库 schema 迁移版本记录"),
	}
}

func (SchemaMigration) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Positive().Immutable().Comment("自增主键 ID"),
		field.String("version").MaxLen(64).Unique().Comment("schema 版本号"),
		field.String("description").MaxLen(255).Default("").Comment("本次变更说明"),
		field.String("checksum").MaxLen(64).Default("").Comment("期望表结构摘要（SHA-256）"),
		field.Int64("duration_ms").Default(0).Comment("本次迁移耗时（毫秒）"),
		field.Time("applied_at").Default(time.Now).Immutable().Comment("应用时间"),
	}
}

func (SchemaMigration) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("applied_at"),
	}
}
