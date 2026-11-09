package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MediaFile 媒体库文件元数据。文件本体仍在磁盘上，这里只记录上传时采集的
// 原始文件名、真实 MIME、尺寸、宽高与上传人，供后台检索与删除引用检查使用。
type MediaFile struct {
	ent.Schema
}

func (MediaFile) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (MediaFile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "media_files"},
		entsql.WithComments(true),
		schema.Comment("媒体库文件元数据表"),
	}
}

func (MediaFile) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable().
			Comment("自增主键 ID"),

		field.String("url").
			MaxLen(512).
			NotEmpty().
			Unique().
			Comment("站内访问路径，如 /uploads/2026/01/xxx.png"),

		field.String("original_name").
			MaxLen(255).
			Default("").
			Comment("上传时的原始文件名"),

		field.String("mime").
			MaxLen(128).
			Default("").
			Comment("由文件内容探测出的 MIME 类型"),

		field.Int64("size").
			Default(0).
			NonNegative().
			Comment("文件大小（字节）"),

		field.Int("width").
			Default(0).
			Comment("图片宽度（像素），非图片为 0"),

		field.Int("height").
			Default(0).
			Comment("图片高度（像素），非图片为 0"),

		field.Int64("uploader_id").
			Optional().
			Nillable().
			Comment("上传人管理员 ID"),
	}
}

func (MediaFile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("uploader_id"),
	}
}
