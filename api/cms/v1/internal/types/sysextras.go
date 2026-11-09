// Code scaffolded manually. Safe to edit.
// 系统配置 / 字典 / 站点信息新增接口的类型单独放这里，避免 goctl 重生成 types.go 时被覆盖。

package types

type ConfigKeyReq struct {
	Key string `path:"key"`
}

type ConfigBatchItem struct {
	ID          int64   `json:"id"`
	ConfigValue *string `json:"config_value,optional"`
}

type ConfigBatchUpdateReq struct {
	Items []ConfigBatchItem `json:"items"`
}

type ConfigVersionItem struct {
	ID           int64  `json:"id"`
	ConfigID     int64  `json:"config_id"`
	ConfigKey    string `json:"config_key"`
	OldValue     string `json:"old_value"`
	NewValue     string `json:"new_value"`
	OperatorID   int64  `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	CreatedAt    string `json:"created_at"`
}

type ConfigVersionListResp struct {
	List []ConfigVersionItem `json:"list"`
}

type ConfigRollbackReq struct {
	ID        int64 `path:"id"`
	VersionID int64 `json:"version_id"`
}

type PublicDictItem struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	IsDefault bool   `json:"is_default"`
	ParentID  int64  `json:"parent_id"`
	Sort      int    `json:"sort"`
}

type PublicDictResp struct {
	Code  string           `json:"code"`
	Items []PublicDictItem `json:"items"`
}

type PublicDictReq struct {
	Code string `path:"code"`
}

type SiteInfoResp struct {
	Name           string `json:"name"`
	Logo           string `json:"logo"`
	Description    string `json:"description"`
	Keywords       string `json:"keywords"`
	ICP            string `json:"icp"`
	Footer         string `json:"footer"`
	Language       string `json:"language"`
	TitleTemplate  string `json:"title_template"`
	Theme          string `json:"theme"`
	PrimaryColor   string `json:"primary_color"`
	CommentEnabled bool   `json:"comment_enabled"`
}
