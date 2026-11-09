//go:build integration

package migration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/yuwen002/meteor-cms/ent"
	entmigrate "github.com/yuwen002/meteor-cms/ent/migrate"
	_ "github.com/yuwen002/meteor-cms/ent/runtime"
	"github.com/yuwen002/meteor-cms/ent/sysconfig"
	"github.com/yuwen002/meteor-cms/internal/migration"
	"github.com/yuwen002/meteor-cms/internal/seed"
)

// upgradeDB 与主库隔离，避免影响 CI 里同一 MySQL 服务的其它迁移用例。
const upgradeDB = "meteor_cms_upgrade_test"

// TestUpgradeFromLegacySchema 模拟「上一版 schema + 存量配置」升级到当前版本：
// 漂移检查发现缺列 → Ent 补列 → 种子补齐并迁移历史显示名 → 版本记录 → 启动校验放行。
// 这也是 CI 中覆盖「旧库升级」而非只覆盖空库安装的门禁用例。
func TestUpgradeFromLegacySchema(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("需要 MYSQL_DSN（make test-integration）；无数据库时跳过")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("解析 MYSQL_DSN 失败: %v", err)
	}
	if cfg.DBName == "" {
		t.Fatal("MYSQL_DSN 必须指定数据库名")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	rootCfg := *cfg
	rootCfg.DBName = ""
	rootDB, err := sql.Open("mysql", rootCfg.FormatDSN())
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer rootDB.Close()
	if _, err := rootDB.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+upgradeDB+"` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("创建隔离库失败: %v", err)
	}
	defer func() {
		_, _ = rootDB.Exec("DROP DATABASE IF EXISTS `" + upgradeDB + "`")
	}()

	upCfg := *cfg
	upCfg.DBName = upgradeDB
	db, err := sql.Open("mysql", upCfg.FormatDSN())
	if err != nil {
		t.Fatalf("连接隔离库失败: %v", err)
	}
	defer db.Close()

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	defer client.Close()

	if err := client.Schema.Create(ctx, entmigrate.WithDropColumn(false)); err != nil {
		t.Fatalf("初始化表结构失败: %v", err)
	}
	// 上一版留下的三行配置：更名前站名、更名后但未模板化的页脚、运营自定义描述。
	for _, row := range []struct{ key, value string }{
		{"site_name", "Go Meteor CMS"},
		{"site_footer", "© 2026 Meteor CMS"},
		{"site_description", "运营自定义描述"},
	} {
		err := client.SysConfig.Create().
			SetConfigKey(row.key).
			SetConfigValue(row.value).
			SetName(row.key).
			SetConfigGroup("site").
			SetValueType("string").
			SetIsSystem(true).
			SetStatus(1).
			Exec(ctx)
		if err != nil {
			t.Fatalf("写入存量配置 %s 失败: %v", row.key, err)
		}
	}

	// 删掉「后加的列」，把当前结构回退成上一版 schema。
	if _, err := db.ExecContext(ctx,
		"ALTER TABLE sys_configs DROP COLUMN value_type, DROP COLUMN validation, DROP COLUMN is_secret"); err != nil {
		t.Fatalf("构造旧版 schema 失败: %v", err)
	}
	drift, err := migration.DetectDrift(ctx, db, upgradeDB)
	if err != nil {
		t.Fatalf("漂移检查失败: %v", err)
	}
	if !containsAll(drift.MissingColumns,
		"sys_configs.value_type", "sys_configs.validation", "sys_configs.is_secret") {
		t.Fatalf("旧版 schema 的缺失列未被识别: %v", drift.MissingColumns)
	}

	if err := client.Schema.Create(ctx, entmigrate.WithDropColumn(false)); err != nil {
		t.Fatalf("升级表结构失败: %v", err)
	}
	drift, err = migration.DetectDrift(ctx, db, upgradeDB)
	if err != nil || !drift.Empty() {
		t.Fatalf("升级后仍存在漂移: drift=%+v err=%v", drift, err)
	}

	if err := runConfigSeed(ctx, client); err != nil {
		t.Fatalf("升级种子失败: %v", err)
	}
	assertConfigs(t, ctx, client)

	countBefore, err := client.SysConfig.Query().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := runConfigSeed(ctx, client); err != nil {
		t.Fatalf("重复执行种子失败: %v", err)
	}
	countAfter, err := client.SysConfig.Query().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if countAfter != countBefore {
		t.Fatalf("种子不幂等：重跑后配置数从 %d 变为 %d", countBefore, countAfter)
	}
	assertConfigs(t, ctx, client)

	status, err := migration.ReadStatus(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	if status.OK || status.Version != "none" {
		t.Fatalf("未记录版本时 readyz 不应放行: %+v", status)
	}
	if err := migration.RecordVersion(ctx, client, "integration upgrade", time.Second); err != nil {
		t.Fatalf("记录 schema 版本失败: %v", err)
	}
	status, err = migration.EnsureVersion(ctx, client, true)
	if err != nil || !status.OK {
		t.Fatalf("升级完成后启动校验应放行: status=%+v err=%v", status, err)
	}
}

func runConfigSeed(ctx context.Context, client *ent.Client) error {
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := seed.InitConfigs(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func assertConfigs(t *testing.T, ctx context.Context, client *ent.Client) {
	t.Helper()
	rows, err := client.SysConfig.Query().All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string, len(rows))
	for _, r := range rows {
		values[r.ConfigKey] = r.ConfigValue
	}
	for key, want := range map[string]string{
		"site_name":        "Meteor CMS",
		"site_footer":      "© {year} Meteor CMS",
		"site_description": "运营自定义描述",
	} {
		if values[key] != want {
			t.Errorf("%s = %q, want %q", key, values[key], want)
		}
	}
	row, err := client.SysConfig.Query().Where(sysconfig.ConfigKeyEQ("site_description")).Only(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row.ValueType == "" || row.ConfigGroup != "site" {
		t.Errorf("历史行的新增列未被补齐: value_type=%q config_group=%q", row.ValueType, row.ConfigGroup)
	}
}

func containsAll(got []string, wants ...string) bool {
	set := make(map[string]bool, len(got))
	for _, g := range got {
		set[g] = true
	}
	for _, w := range wants {
		if !set[w] {
			return false
		}
	}
	return true
}
