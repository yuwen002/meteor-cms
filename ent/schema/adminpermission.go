package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AdminPermission 后台权限表
type AdminPermission struct {
	ent.Schema
}

func (AdminPermission) Mixin() []ent.Mixin {
	return []ent.Mixin{
		BaseMixin{},
		SoftDeleteMixin{},
	}
}

func (AdminPermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "admin_permissions",
		},
		entsql.WithComments(true),
		schema.Comment("后台权限表（菜单/按钮/接口）"),
	}
}

func (AdminPermission) Fields() []ent.Field {
	return []ent.Field{
		// 基础字段
		field.Int64("id").
			Unique().
			Immutable().
			Comment("主键ID"),

		field.String("name").
			NotEmpty().
			Comment("权限名称，如 用户管理 / 新增用户"),

		field.Int64("parent_id").
			Optional().
			Nillable().
			Comment("父级ID（用于菜单树）"),

		field.Int("type").
			Default(1).
			Comment("权限类型：1 菜单 2 按钮 3 API"),

		// 前端菜单相关
		field.String("path").
			Default("").
			Comment("前端路由路径，仅菜单(type=1)使用"),

		field.String("component").
			Default("").
			Comment("前端组件路径，如 views/user/list.vue"),

		field.String("icon").
			Default("").
			Comment("菜单图标"),

		// API 权限相关
		field.String("method").
			Default("").
			Comment("API 方法：GET/POST/PUT/DELETE，仅 type=3 使用"),

		field.String("api_path").
			Default("").
			Comment("API 路径，如 /admin/user/list，仅 type=3 使用"),

		// 权限标识（最重要）
		field.String("permission").
			Default("").
			Comment("权限标识，如 system:user:list"),

		// 控制项
		field.Bool("is_active").
			Default(true).
			Comment("是否启用"),

		field.Int("sort").
			Default(0).
			Comment("排序"),
	}
}

// Indexes 给权限记录加业务唯一约束，使并发迁移无法插入重复权限。
// 去重键与 UpsertPermissions 的判重条件完全一致：菜单/按钮的 method、api_path 为空串，
// 等价于按 (type, permission) 唯一；API 权限则允许同一权限码绑定多条不同接口。
// 升级前 cmd/migrate 会先执行 DedupePermissions 清理历史重复数据，否则建索引会失败。
func (AdminPermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type", "permission", "method", "api_path").Unique(),
	}
}

func (AdminPermission) Edges() []ent.Edge {
	return []ent.Edge{
		// 权限树
		edge.From("parent", AdminPermission.Type).
			Ref("children").
			Field("parent_id").
			Unique().
			Annotations(
				entsql.OnDelete(entsql.SetNull),
			),

		edge.To("children", AdminPermission.Type),

		// 角色-权限（反向多对多）
		edge.From("roles", AdminRole.Type).
			Ref("permissions"),
	}
}
