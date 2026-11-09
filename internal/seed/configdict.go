package seed

import (
	"context"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/sysconfig"
	"github.com/yuwen002/meteor-cms/ent/sysdict"
	"github.com/yuwen002/meteor-cms/ent/sysdictitem"
)

type configSeed struct {
	key, name, value, group, valueType, validation, remark string
	sort                                                   int
	secret                                                 bool
}

// defaultConfigs 是站点运行所依赖的内置配置键。
// 种子只在键不存在时插入，已存在时只补齐 is_system 标记，绝不覆盖运营已改的值。
var defaultConfigs = []configSeed{
	{key: "site_name", name: "站点名称", value: "Meteor CMS", group: "site", valueType: "string", sort: 10, remark: "用于 RSS 标题、页面标题与前台展示"},
	{key: "site_logo", name: "站点 Logo", value: "", group: "site", valueType: "url", sort: 20, validation: `^https?://|^/`, remark: "留空则显示文字站名"},
	{key: "site_description", name: "站点描述", value: "基于 Go + Vue 的轻量内容管理系统", group: "site", valueType: "text", sort: 30, remark: "RSS description 与首页 meta"},
	{key: "site_keywords", name: "站点关键词", value: "", group: "site", valueType: "string", sort: 40},
	{key: "site_icp", name: "备案号", value: "", group: "site", valueType: "string", sort: 50},
	{key: "site_footer", name: "页脚文本", value: "© {year} Meteor CMS", group: "site", valueType: "text", sort: 60, remark: "支持 {year}（当年）、{site_name} / {site}（站名）占位符，输出时展开；直接写死年份则原样展示"},
	{key: "site_language", name: "站点语言", value: "zh-CN", group: "site", valueType: "string", sort: 70, remark: "RSS language 与 html lang"},
	{key: "site_url", name: "站点公开地址", value: "", group: "site", valueType: "url", sort: 80, validation: `^https?://`, remark: "RSS / sitemap / robots 绝对地址；留空用配置文件 WebBaseURL"},
	{key: "seo_title_template", name: "页面标题模板", value: "{title} - {site_name}", group: "seo", valueType: "string", sort: 10, remark: "支持 {title}、{site_name}、{site}"},
	{key: "seo_default_description", name: "默认 SEO 描述", value: "", group: "seo", valueType: "text", sort: 20},
	{key: "seo_robots_disallow", name: "robots 禁止路径", value: "/admin", group: "seo", valueType: "text", sort: 30, remark: "逗号或换行分隔"},
	{key: "seo_robots_sitemap", name: "robots 声明 Sitemap", value: "true", group: "seo", valueType: "bool", sort: 40},
	{key: "mail_smtp_password", name: "SMTP 密码", value: "", group: "mail", valueType: "string", sort: 10, secret: true, remark: "加密存储，列表不回显明文"},
	{key: "security_login_captcha", name: "启用登录验证码", value: "true", group: "security", valueType: "bool", sort: 10},
	{key: "theme_preset", name: "主题预设", value: "light", group: "theme", valueType: "string", sort: 10},
	{key: "theme_primary_color", name: "主题主色", value: "#409EFF", group: "theme", valueType: "string", sort: 20, validation: `^#[0-9A-Fa-f]{6}$`},
	{key: "feature_comment_enabled", name: "启用评论", value: "false", group: "feature", valueType: "bool", sort: 10, remark: "关闭时公开评论接口拒绝写入"},
	{key: "feature_comment_review", name: "评论需审核", value: "true", group: "feature", valueType: "bool", sort: 20, remark: "开启后新评论为待审状态"},
	{key: "feature_link_enabled", name: "启用友链", value: "true", group: "feature", valueType: "bool", sort: 30},
	{key: "feature_subscription_enabled", name: "启用邮件订阅", value: "true", group: "feature", valueType: "bool", sort: 40},
	{key: "feature_registration_enabled", name: "启用注册", value: "false", group: "feature", valueType: "bool", sort: 50},
}

type dictItemSeed struct {
	label, value string
	sort         int
	def          bool
	parentValue  string
}

type dictSeed struct {
	code, name, group, description string
	items                          []dictItemSeed
}

var defaultDicts = []dictSeed{
	{code: "article_source", name: "文章来源", group: "content", description: "文章来源类型", items: []dictItemSeed{
		{label: "原创", value: "original", sort: 10, def: true},
		{label: "转载", value: "reprint", sort: 20},
		{label: "翻译", value: "translation", sort: 30},
	}},
	{code: "article_priority", name: "文章优先级", group: "content", description: "推荐/置顶级别", items: []dictItemSeed{
		{label: "普通", value: "normal", sort: 10, def: true},
		{label: "推荐", value: "recommended", sort: 20},
		{label: "置顶", value: "top", sort: 30},
	}},
	{code: "link_type", name: "友链分类", group: "site", description: "友情链接分组", items: []dictItemSeed{
		{label: "友情链接", value: "friend", sort: 10, def: true},
		{label: "合作伙伴", value: "partner", sort: 20},
		{label: "赞助商", value: "sponsor", sort: 30},
	}},
	{code: "theme_preset", name: "主题预设", group: "site", description: "前台主题", items: []dictItemSeed{
		{label: "浅色", value: "light", sort: 10, def: true},
		{label: "深色", value: "dark", sort: 20},
		{label: "跟随系统", value: "auto", sort: 30},
	}},
	{code: "locale", name: "语言", group: "site", description: "站点可用语言", items: []dictItemSeed{
		{label: "简体中文", value: "zh-CN", sort: 10, def: true},
		{label: "English", value: "en-US", sort: 20},
	}},
	{code: "yes_no", name: "是否", group: "system", description: "通用布尔枚举", items: []dictItemSeed{
		{label: "是", value: "1", sort: 10, def: true},
		{label: "否", value: "0", sort: 20},
	}},
	{code: "config_value_type", name: "配置值类型", group: "system", description: "配置管理页值类型下拉", items: []dictItemSeed{
		{label: "字符串", value: "string", sort: 10, def: true},
		{label: "多行文本", value: "text", sort: 20},
		{label: "整数", value: "int", sort: 30},
		{label: "布尔", value: "bool", sort: 40},
		{label: "JSON", value: "json", sort: 50},
		{label: "URL", value: "url", sort: 60},
		{label: "图片", value: "image", sort: 70},
	}},
	{code: "dict_group", name: "字典分组", group: "system", description: "字典管理页分组下拉", items: []dictItemSeed{
		{label: "内容", value: "content", sort: 10},
		{label: "站点", value: "site", sort: 20},
		{label: "系统", value: "system", sort: 30},
	}},
	{code: "config_group", name: "配置分组", group: "system", description: "配置管理页分组", items: []dictItemSeed{
		{label: "站点", value: "site", sort: 10},
		{label: "SEO", value: "seo", sort: 20},
		{label: "邮件", value: "mail", sort: 30},
		{label: "安全", value: "security", sort: 40},
		{label: "主题", value: "theme", sort: 50},
		{label: "功能", value: "feature", sort: 60},
	}},
}

// legacyDisplayValues 记录历史上写入过的种子默认值。项目由 go-meteor-cms
// 更名为 meteor-cms、以及页脚改为 {year} 年份模板后，已有库里的站名 / 页脚
// 仍是旧值，而种子按设计不覆盖已有值。这里做一次性迁移：仅当当前值精确等于
// 任一历史默认值时才写成当前种子值，运营自定义过的站名、页脚完全不受影响。
// 一个键可对应多个历史值，覆盖「更名前」「更名后但未模板化」的存量库。
var legacyDisplayValues = map[string][]string{
	"site_name":   {"Go Meteor CMS"},
	"site_footer": {"© 2026 Go Meteor CMS", "© 2026 Meteor CMS"},
}

func renamedDisplayValue(key, current, seedValue string) (string, bool) {
	if current == seedValue {
		return "", false
	}
	for _, legacy := range legacyDisplayValues[key] {
		if current == legacy {
			return seedValue, true
		}
	}
	return "", false
}

// InitConfigs 幂等写入内置配置项；不覆盖已有值。
// 对历史行同时补齐后加的列（分组 / 值类型 / 校验 / 排序 / 密文标记），
// 只填「仍为零值」的字段，避免覆盖运营在后台改过的分组、状态或停用设置。
func InitConfigs(ctx context.Context, tx *ent.Tx) error {
	for _, c := range defaultConfigs {
		row, err := tx.SysConfig.Query().Where(sysconfig.ConfigKeyEQ(c.key)).Only(ctx)
		if err == nil {
			upd := tx.SysConfig.UpdateOneID(row.ID)
			changed := false
			if !row.IsSystem {
				upd = upd.SetIsSystem(true)
				changed = true
			}
			if row.ConfigGroup == "" {
				upd = upd.SetConfigGroup(c.group)
				changed = true
			}
			if row.ValueType == "" {
				upd = upd.SetValueType(c.valueType)
				changed = true
			}
			if row.Validation == "" && c.validation != "" {
				upd = upd.SetValidation(c.validation)
				changed = true
			}
			if row.Sort == 0 {
				upd = upd.SetSort(c.sort)
				changed = true
			}
			// 密文标记只升级不降级：运营把某项改成普通配置后不应被种子强制改回。
			if !row.IsSecret && c.secret {
				upd = upd.SetIsSecret(true)
				changed = true
			}
			if v, ok := renamedDisplayValue(c.key, row.ConfigValue, c.value); ok {
				upd = upd.SetConfigValue(v)
				changed = true
			}
			if changed {
				if _, err := upd.Save(ctx); err != nil {
					return err
				}
			}
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		if _, err := tx.SysConfig.Create().
			SetConfigKey(c.key).
			SetConfigValue(c.value).
			SetName(c.name).
			SetRemark(c.remark).
			SetIsSystem(true).
			SetConfigGroup(c.group).
			SetValueType(c.valueType).
			SetValidation(c.validation).
			SetSort(c.sort).
			SetStatus(1).
			SetIsSecret(c.secret).
			Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

// InitDicts 幂等写入内置字典与字典项；按 code / value 判重，不覆盖运营改动。
func InitDicts(ctx context.Context, tx *ent.Tx) error {
	for _, d := range defaultDicts {
		dict, err := tx.SysDict.Query().Where(sysdict.CodeEQ(d.code)).Only(ctx)
		if ent.IsNotFound(err) {
			dict, err = tx.SysDict.Create().
				SetCode(d.code).
				SetName(d.name).
				SetDescription(d.description).
				SetStatus(1).
				SetDictGroup(d.group).
				SetIsSystem(true).
				Save(ctx)
		}
		if err != nil {
			return err
		}
		upd := tx.SysDict.UpdateOneID(dict.ID)
		changed := false
		if !dict.IsSystem {
			upd = upd.SetIsSystem(true)
			changed = true
		}
		// 补齐后加的分组列，同样不覆盖运营已填的值。
		if dict.DictGroup == "" {
			upd = upd.SetDictGroup(d.group)
			changed = true
		}
		if changed {
			if _, err := upd.Save(ctx); err != nil {
				return err
			}
		}

		// 先建一轮，记录 value -> id 以支持 parentValue
		valueIDs := make(map[string]int64, len(d.items))
		for _, it := range d.items {
			existing, err := tx.SysDictItem.Query().
				Where(
					sysdictitem.DictCodeEQ(d.code),
					sysdictitem.ValueEQ(it.value),
				).
				Only(ctx)
			if err == nil {
				valueIDs[it.value] = existing.ID
				continue
			}
			if !ent.IsNotFound(err) {
				return err
			}
			created, err := tx.SysDictItem.Create().
				SetDictCode(d.code).
				SetLabel(it.label).
				SetValue(it.value).
				SetStatus(1).
				SetSort(it.sort).
				SetIsDefault(it.def).
				Save(ctx)
			if err != nil {
				return err
			}
			valueIDs[it.value] = created.ID
		}
		for _, it := range d.items {
			if it.parentValue == "" {
				continue
			}
			if parentID, ok := valueIDs[it.parentValue]; ok {
				itemID := valueIDs[it.value]
				if _, err := tx.SysDictItem.UpdateOneID(itemID).SetParentID(parentID).Save(ctx); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
