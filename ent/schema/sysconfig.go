package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SysConfig 系统配置表，以键值对形式存放站点级参数
type SysConfig struct {
	ent.Schema
}

func (SysConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
		schema.Comment("系统配置表，以键值对形式存放站点级参数"),
	}
}

func (SysConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (SysConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Comment("自增主键 ID").
			Unique().
			Positive().
			Immutable(),

		field.String("config_key").
			MaxLen(128).
			NotEmpty().
			Unique().
			Comment("配置键，全局唯一，如 site_name"),

		field.String("config_value").
			MaxLen(2048).
			Optional().
			Comment("配置值"),

		field.String("name").
			MaxLen(128).
			NotEmpty().
			Comment("配置名称（展示用）"),

		field.String("remark").
			MaxLen(512).
			Optional().
			Comment("备注说明"),

		field.Bool("is_system").
			Default(false).
			Comment("是否系统内置：内置配置禁止删除，避免误删导致功能异常"),

		field.String("config_group").
			MaxLen(64).
			Default("general").
			Comment("配置分组，如 site/seo/mail/security，用于分组渲染"),

		field.String("value_type").
			MaxLen(32).
			Default("string").
			Comment("值类型：string/int/bool/json/url/image/text，用于前端控件与服务端校验"),

		field.String("validation").
			MaxLen(512).
			Optional().
			Comment("校验规则：正则或内置规则名，保存前校验"),

		field.Int("sort").
			Default(0).
			Comment("同组内排序，从小到大"),

		field.Int("status").
			Default(1).
			Comment("状态：1 启用 0 禁用，禁用项不参与运行时读取"),

		field.Bool("is_secret").
			Default(false).
			Comment("是否敏感值：加密存储、列表脱敏、审计快照过滤"),
	}
}

func (SysConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_key").Unique(),
		index.Fields("config_group"),
		index.Fields("status"),
	}
}

func (SysConfig) Edges() []ent.Edge {
	return nil
}
