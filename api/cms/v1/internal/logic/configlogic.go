// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/sysconfig"
	"github.com/yuwen002/meteor-cms/ent/sysconfigversion"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/secretbox"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigLogic {
	return &ConfigLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConfigLogic) secret() string {
	return l.svcCtx.Config.Auth.AccessSecret
}

func (l *ConfigLogic) invalidate() {
	if l.svcCtx != nil && l.svcCtx.Settings != nil {
		l.svcCtx.Settings.Invalidate()
	}
}

func (l *ConfigLogic) operator() (int64, string) {
	var uid int64
	if id, ok := utils.CurrentUserID(l.ctx); ok {
		uid = id
	}
	name := ""
	if claims := utils.GetUserFromCtx(l.ctx); claims != nil {
		if v, ok := claims["username"].(string); ok {
			name = v
		}
	}
	return uid, name
}

func configItem(c *ent.SysConfig) types.ConfigItem {
	value := c.ConfigValue
	if c.IsSecret {
		value = secretbox.Mask()
	}
	return types.ConfigItem{
		ID: c.ID, ConfigKey: c.ConfigKey, ConfigValue: value,
		Name: c.Name, Remark: c.Remark, IsSystem: c.IsSystem,
		ConfigGroup: c.ConfigGroup, ValueType: c.ValueType,
		Validation: c.Validation, Sort: c.Sort, Status: c.Status,
		IsSecret: c.IsSecret, CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
}

// validateValue 按值类型与自定义正则校验，空值允许（用于清空配置）。
func validateValue(valueType, validation, value string) error {
	if value != "" {
		switch valueType {
		case "int":
			if _, err := strconv.Atoi(value); err != nil {
				return common.NewBizError(common.ErrInvalidParams)
			}
		case "bool":
			if _, err := strconv.ParseBool(value); err != nil {
				return common.NewBizError(common.ErrInvalidParams)
			}
		case "json":
			if !json.Valid([]byte(value)) {
				return common.NewBizError(common.ErrInvalidParams)
			}
		case "url", "image":
			if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
				return common.NewBizError(common.ErrInvalidParams)
			}
		}
	}
	if validation != "" && value != "" {
		re, err := regexp.Compile(validation)
		if err != nil {
			logx.Errorf("配置校验规则非法: %q %v", validation, err)
			return common.NewBizError(common.ErrInvalidParams)
		}
		if !re.MatchString(value) {
			return common.NewBizError(common.ErrInvalidParams)
		}
	}
	return nil
}

// storedValue 把明文转成落库值；密文配置加密，掩码值表示「不修改」。
func (l *ConfigLogic) storedValue(c *ent.SysConfig, plain string) (string, bool, error) {
	if c.IsSecret {
		if plain == secretbox.Mask() {
			return c.ConfigValue, false, nil
		}
		encrypted, err := secretbox.Encrypt(plain, l.secret())
		return encrypted, true, err
	}
	return plain, true, nil
}

// ConfigList 查询系统配置，支持按 key/名称模糊搜索与分组筛选；密文值只回掩码。
func (l *ConfigLogic) ConfigList(req *types.ConfigListReq) (*types.ConfigListResp, error) {
	query := l.svcCtx.EntClient.SysConfig.Query()
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		query = query.Where(sysconfig.Or(
			sysconfig.ConfigKeyContainsFold(kw),
			sysconfig.NameContainsFold(kw),
		))
	}
	if g := strings.TrimSpace(req.Group); g != "" {
		query = query.Where(sysconfig.ConfigGroupEQ(g))
	}

	rows, err := query.
		Order(ent.Asc(sysconfig.FieldConfigGroup), ent.Asc(sysconfig.FieldSort), ent.Asc(sysconfig.FieldConfigKey)).
		All(l.ctx)
	if err != nil {
		logx.Errorf("list config failed: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	list := make([]types.ConfigItem, 0, len(rows))
	for _, c := range rows {
		list = append(list, configItem(c))
	}
	return &types.ConfigListResp{List: list}, nil
}

// ConfigByKey 按配置键读取单条（密文仍回掩码）。
func (l *ConfigLogic) ConfigByKey(key string) (*types.ConfigItem, error) {
	c, err := l.svcCtx.EntClient.SysConfig.Query().
		Where(sysconfig.ConfigKeyEQ(strings.TrimSpace(key))).
		Only(l.ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrConfigNotFound)
		}
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	item := configItem(c)
	return &item, nil
}

// ConfigCreate 新增配置项，键全局唯一。
func (l *ConfigLogic) ConfigCreate(req *types.ConfigCreateReq) error {
	key := strings.TrimSpace(req.ConfigKey)
	name := strings.TrimSpace(req.Name)
	if key == "" || name == "" {
		return common.NewBizError(common.ErrInvalidParams)
	}
	valueType := strings.TrimSpace(req.ValueType)
	if valueType == "" {
		valueType = "string"
	}
	if err := validateValue(valueType, req.Validation, req.ConfigValue); err != nil {
		return err
	}
	exists, err := l.svcCtx.EntClient.SysConfig.Query().
		Where(sysconfig.ConfigKeyEQ(key)).
		Exist(l.ctx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return common.NewBizError(common.ErrConfigKeyExists)
	}

	stored := req.ConfigValue
	if req.IsSecret {
		stored, err = secretbox.Encrypt(req.ConfigValue, l.secret())
		if err != nil {
			return common.NewBizError(common.ErrInternalServer)
		}
	}
	status := 1
	if req.Status != nil {
		status = *req.Status
	}
	group := strings.TrimSpace(req.ConfigGroup)
	if group == "" {
		group = "general"
	}
	if _, err := l.svcCtx.EntClient.SysConfig.Create().
		SetConfigKey(key).
		SetConfigValue(stored).
		SetName(name).
		SetRemark(req.Remark).
		SetIsSystem(false).
		SetConfigGroup(group).
		SetValueType(valueType).
		SetValidation(req.Validation).
		SetSort(req.Sort).
		SetStatus(status).
		SetIsSecret(req.IsSecret).
		Save(l.ctx); err != nil {
		logx.Errorf("create config failed: %v", err)
		return common.NewBizError(common.ErrConfigCreateFail)
	}
	l.invalidate()
	return nil
}

// applyValueInTx 在事务内写入一个配置值并追加版本记录。
func (l *ConfigLogic) applyValueInTx(ctx context.Context, tx *ent.Tx, c *ent.SysConfig, plain string, uid int64, uname string) error {
	if err := validateValue(c.ValueType, c.Validation, plain); err != nil {
		return err
	}
	stored, changed, err := l.storedValue(c, plain)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if !changed {
		return nil
	}
	if stored == c.ConfigValue {
		return nil
	}
	if _, err := tx.SysConfig.UpdateOneID(c.ID).SetConfigValue(stored).Save(ctx); err != nil {
		return err
	}
	_, err = tx.SysConfigVersion.Create().
		SetConfigID(c.ID).
		SetConfigKey(c.ConfigKey).
		SetOldValue(c.ConfigValue).
		SetNewValue(stored).
		SetOperatorID(uid).
		SetOperatorName(uname).
		Save(ctx)
	return err
}

// ConfigUpdate 更新配置项；指针字段区分「未传」与「显式清空」。
func (l *ConfigLogic) ConfigUpdate(req *types.ConfigUpdateReq) error {
	c, err := l.svcCtx.EntClient.SysConfig.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return common.NewBizError(common.ErrConfigNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}

	// 系统内置项允许改值，但不允许改键、分组、类型、密文标记与启用状态
	meta := c.Update()
	if req.Name != nil {
		meta = meta.SetName(strings.TrimSpace(*req.Name))
	}
	if req.Remark != nil {
		meta = meta.SetRemark(*req.Remark)
	}
	if !c.IsSystem {
		if req.ConfigGroup != nil {
			meta = meta.SetConfigGroup(strings.TrimSpace(*req.ConfigGroup))
		}
		if req.ValueType != nil {
			meta = meta.SetValueType(strings.TrimSpace(*req.ValueType))
		}
		if req.Validation != nil {
			meta = meta.SetValidation(*req.Validation)
		}
		if req.Sort != nil {
			meta = meta.SetSort(*req.Sort)
		}
		if req.Status != nil {
			meta = meta.SetStatus(*req.Status)
		}
	}

	txCtx := &l.ctx
	cancel := newTxContext(txCtx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(*txCtx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	name, remark := c.Name, c.Remark
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	if req.Remark != nil {
		remark = *req.Remark
	}
	metaApplied := tx.SysConfig.UpdateOneID(c.ID).
		SetName(name).
		SetRemark(remark)
	if !c.IsSystem {
		if req.ConfigGroup != nil {
			metaApplied = metaApplied.SetConfigGroup(strings.TrimSpace(*req.ConfigGroup))
		}
		if req.ValueType != nil {
			metaApplied = metaApplied.SetValueType(strings.TrimSpace(*req.ValueType))
		}
		if req.Validation != nil {
			metaApplied = metaApplied.SetValidation(*req.Validation)
		}
		if req.Sort != nil {
			metaApplied = metaApplied.SetSort(*req.Sort)
		}
		if req.Status != nil {
			metaApplied = metaApplied.SetStatus(*req.Status)
		}
	}
	if _, err := metaApplied.Save(*txCtx); err != nil {
		logx.Errorf("update config meta failed: %v", err)
		return common.NewBizError(common.ErrConfigUpdateFail)
	}

	uid, uname := l.operator()
	if req.ConfigValue != nil {
		if err := l.applyValueInTx(*txCtx, tx, c, *req.ConfigValue, uid, uname); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed = true
	l.invalidate()
	return nil
}

// ConfigBatchUpdate 一次事务保存一组配置值，全部校验通过才提交。
func (l *ConfigLogic) ConfigBatchUpdate(req *types.ConfigBatchUpdateReq) error {
	if len(req.Items) == 0 {
		return common.NewBizError(common.ErrInvalidParams)
	}
	txCtx := &l.ctx
	cancel := newTxContext(txCtx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(*txCtx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	uid, uname := l.operator()
	for _, item := range req.Items {
		if item.ConfigValue == nil {
			continue
		}
		c, err := tx.SysConfig.Query().Where(sysconfig.IDEQ(item.ID)).Only(*txCtx)
		if err != nil {
			return common.NewBizError(common.ErrConfigNotFound)
		}
		if err := l.applyValueInTx(*txCtx, tx, c, *item.ConfigValue, uid, uname); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed = true
	l.invalidate()
	return nil
}

// ConfigVersions 返回某配置最近的变更历史（密文值不解密，避免二次泄露）。
func (l *ConfigLogic) ConfigVersions(id int64) (*types.ConfigVersionListResp, error) {
	rows, err := l.svcCtx.EntClient.SysConfigVersion.Query().
		Where(sysconfigversion.ConfigIDEQ(id)).
		Order(ent.Desc(sysconfigversion.FieldID)).
		Limit(50).
		All(l.ctx)
	if err != nil {
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	list := make([]types.ConfigVersionItem, 0, len(rows))
	for _, v := range rows {
		oldValue, newValue := v.OldValue, v.NewValue
		if c, err := l.svcCtx.EntClient.SysConfig.Get(l.ctx, id); err == nil && c.IsSecret {
			oldValue, newValue = secretbox.Mask(), secretbox.Mask()
		}
		list = append(list, types.ConfigVersionItem{
			ID: v.ID, ConfigID: v.ConfigID, ConfigKey: v.ConfigKey,
			OldValue: oldValue, NewValue: newValue,
			OperatorID: v.OperatorID, OperatorName: v.OperatorName,
			CreatedAt: v.CreatedAt.Format(time.RFC3339),
		})
	}
	return &types.ConfigVersionListResp{List: list}, nil
}

// ConfigRollback 把配置值回滚到某个历史版本，并追加一条新的版本记录。
func (l *ConfigLogic) ConfigRollback(req *types.ConfigRollbackReq) error {
	v, err := l.svcCtx.EntClient.SysConfigVersion.Get(l.ctx, req.VersionID)
	if err != nil || v.ConfigID != req.ID {
		return common.NewBizError(common.ErrConfigNotFound)
	}
	c, err := l.svcCtx.EntClient.SysConfig.Get(l.ctx, req.ID)
	if err != nil {
		return common.NewBizError(common.ErrConfigNotFound)
	}

	txCtx := &l.ctx
	cancel := newTxContext(txCtx)
	defer cancel()
	tx, err := l.svcCtx.EntClient.Tx(*txCtx)
	if err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.SysConfig.UpdateOneID(c.ID).SetConfigValue(v.OldValue).Save(*txCtx); err != nil {
		return common.NewBizError(common.ErrConfigUpdateFail)
	}
	uid, uname := l.operator()
	if _, err := tx.SysConfigVersion.Create().
		SetConfigID(c.ID).
		SetConfigKey(c.ConfigKey).
		SetOldValue(c.ConfigValue).
		SetNewValue(v.OldValue).
		SetOperatorID(uid).
		SetOperatorName(uname).
		Save(*txCtx); err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	if err := tx.Commit(); err != nil {
		return common.NewBizError(common.ErrInternalServer)
	}
	committed = true
	l.invalidate()
	return nil
}

// ConfigDetail 按 ID 返回单条配置（密文值回掩码）。
func (l *ConfigLogic) ConfigDetail(id int64) (*types.ConfigItem, error) {
	c, err := l.svcCtx.EntClient.SysConfig.Get(l.ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, common.NewBizError(common.ErrConfigNotFound)
		}
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	item := configItem(c)
	return &item, nil
}

// ConfigDelete 删除配置项；系统内置配置禁止删除。
func (l *ConfigLogic) ConfigDelete(req *types.ConfigDetailReq) error {
	c, err := l.svcCtx.EntClient.SysConfig.Get(l.ctx, req.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return common.NewBizError(common.ErrConfigNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}
	if c.IsSystem {
		return common.NewBizError(common.ErrConfigDeleteSystem)
	}
	if err := l.svcCtx.EntClient.SysConfig.DeleteOneID(req.ID).Exec(l.ctx); err != nil {
		return common.NewBizError(common.ErrConfigUpdateFail)
	}
	l.invalidate()
	return nil
}
