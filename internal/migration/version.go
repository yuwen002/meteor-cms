// Package migration 承载 schema 版本记录、迁移并发锁、结构漂移检查与数据回填。
package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	entmigrate "github.com/yuwen002/meteor-cms/ent/migrate"
	"github.com/yuwen002/meteor-cms/ent/schemamigration"
	"github.com/yuwen002/meteor-cms/internal/version"
)

// SchemaChecksum 基于 Ent 生成的表定义计算稳定摘要，用于识别「版本号没变但结构变了」。
func SchemaChecksum() string {
	tables := make([]string, 0, len(entmigrate.Tables))
	for _, t := range entmigrate.Tables {
		cols := make([]string, 0, len(t.Columns))
		for _, c := range t.Columns {
			cols = append(cols, fmt.Sprintf("%s:%d:%t:%t", c.Name, c.Type, c.Nullable, c.Unique))
		}
		sort.Strings(cols)
		idxs := make([]string, 0, len(t.Indexes))
		for _, i := range t.Indexes {
			names := make([]string, 0, len(i.Columns))
			for _, c := range i.Columns {
				names = append(names, c.Name)
			}
			idxs = append(idxs, strings.Join(names, "+")+fmt.Sprintf(":%t", i.Unique))
		}
		sort.Strings(idxs)
		tables = append(tables, t.Name+"["+strings.Join(cols, ",")+"]["+strings.Join(idxs, ",")+"]")
	}
	sort.Strings(tables)
	sum := sha256.Sum256([]byte(strings.Join(tables, "\n")))
	return hex.EncodeToString(sum[:])
}

// RecordVersion 记录一次成功迁移；同一版本重复执行只更新耗时与摘要，不新增历史行。
func RecordVersion(ctx context.Context, client *ent.Client, description string, duration time.Duration) error {
	cs := SchemaChecksum()
	ms := duration.Milliseconds()
	existing, err := client.SchemaMigration.Query().
		Where(schemamigration.VersionEQ(version.SchemaVersion)).
		Only(ctx)
	if err == nil {
		_, err = existing.Update().
			SetDescription(description).
			SetChecksum(cs).
			SetDurationMs(ms).
			Save(ctx)
		return err
	}
	if !ent.IsNotFound(err) {
		return err
	}
	_, err = client.SchemaMigration.Create().
		SetVersion(version.SchemaVersion).
		SetDescription(description).
		SetChecksum(cs).
		SetDurationMs(ms).
		SetAppliedAt(time.Now()).
		Save(ctx)
	return err
}

// Status 是 readyz 与启动日志需要的版本状态。
type Status struct {
	Version          string `json:"version"`
	Checksum         string `json:"checksum"`
	ExpectedVersion  string `json:"expected_version"`
	ExpectedChecksum string `json:"expected_checksum"`
	OK               bool   `json:"ok"`
}

// ReadStatus 读取最新一条迁移记录。空表时返回一条 OK=false 的状态而非错误，
// 便于 readyz 明确区分「库没迁移」与「数据库连不上」。
func ReadStatus(ctx context.Context, client *ent.Client) (Status, error) {
	s := Status{ExpectedVersion: version.SchemaVersion, ExpectedChecksum: SchemaChecksum()}
	row, err := client.SchemaMigration.Query().
		Order(ent.Desc(schemamigration.FieldAppliedAt), ent.Desc(schemamigration.FieldID)).
		First(ctx)
	if ent.IsNotFound(err) {
		s.Version = "none"
		return s, nil
	}
	if err != nil {
		return s, err
	}
	s.Version = row.Version
	s.Checksum = row.Checksum
	s.OK = row.Version == version.SchemaVersion
	return s, nil
}

// EnsureVersion 在 API 启动时校验 schema 版本。strict=false 时只用于绕过校验，
// 主要用于旧库紧急回滚；生产不建议关闭。
func EnsureVersion(ctx context.Context, client *ent.Client, strict bool) (Status, error) {
	s, err := ReadStatus(ctx, client)
	if err != nil {
		if strict {
			return s, fmt.Errorf("读取 schema_migrations 失败（请先执行 make migrate）: %w", err)
		}
		return s, nil
	}
	if !s.OK && strict {
		return s, fmt.Errorf(
			"数据库 schema 版本 %s 与程序期望 %s 不一致，请先执行 make migrate（紧急回滚可设 SCHEMA_CHECK_STRICT=false）",
			s.Version, s.ExpectedVersion)
	}
	return s, nil
}
