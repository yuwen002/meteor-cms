package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchemaChecksumIsStable(t *testing.T) {
	a := SchemaChecksum()
	b := SchemaChecksum()
	if a != b {
		t.Fatalf("同一份 Ent 表定义两次计算摘要不一致: %s / %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("摘要应为 SHA-256 十六进制（64 字符），实际 %d", len(a))
	}
}

func TestDatabaseName(t *testing.T) {
	name, err := DatabaseName("root:pwd@tcp(127.0.0.1:3306)/go_meteor_cms?parseTime=True&loc=Local")
	if err != nil || name != "go_meteor_cms" {
		t.Fatalf("解析库名失败: name=%q err=%v", name, err)
	}
	if _, err := DatabaseName("root:pwd@tcp(127.0.0.1:3306)/?parseTime=True"); err == nil {
		t.Fatal("未指定库名时应报错")
	}
	if _, err := DatabaseName("not-a-dsn"); err == nil {
		t.Fatal("非法 DSN 应报错")
	}
}

func TestCheckBackupRejectsMissingAndStale(t *testing.T) {
	if _, err := CheckBackup("", 24*time.Hour); err == nil {
		t.Fatal("未指定备份文件应报错")
	}
	if _, err := CheckBackup("definitely-not-exists.json", 24*time.Hour); err == nil {
		t.Fatal("不存在的备份文件应报错")
	}
	if _, err := CheckBackup("definitely-not-exists.json", 0); err == nil {
		t.Fatal("过期门禁应生效")
	}
}

// 生产迁移用 make migrate-safe（-require-backup）时，新导出的备份必须能通过门禁。
func TestCheckBackupAcceptsFreshFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.json")
	if err := os.WriteFile(path, []byte(`{"articles":[]}`), 0o600); err != nil {
		t.Fatalf("写入临时备份失败: %v", err)
	}
	got, err := CheckBackup(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("新鲜备份应通过 -require-backup 校验: %v", err)
	}
	if !strings.Contains(got, path) {
		t.Fatalf("校验结果未包含备份路径: %s", got)
	}
}

func TestPlaceholders(t *testing.T) {
	in, args := placeholders([]int64{1, 2, 3})
	if in != "?,?,?" || len(args) != 3 {
		t.Fatalf("占位符生成错误: %s / %v", in, args)
	}
	if strings.Contains(in, "1") {
		t.Fatalf("占位符不应内联参数值: %s", in)
	}
}
