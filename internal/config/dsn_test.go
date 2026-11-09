package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDSNEnvWins(t *testing.T) {
	t.Setenv("MYSQL_DSN", "user:pwd@tcp(db:3306)/override?parseTime=True")
	dsn, err := ResolveDSN("")
	if err != nil {
		t.Fatalf("MYSQL_DSN 应直接生效: %v", err)
	}
	if !strings.Contains(dsn, "/override") {
		t.Fatalf("环境变量未生效: %s", dsn)
	}
}

func TestResolveDSNFromExplicitConfig(t *testing.T) {
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("CMS_CONFIG", "")
	path := filepath.Join("..", "..", "api", "cms", "v1", "etc", "cms-api.yaml")
	dsn, err := ResolveDSN(path)
	if err != nil {
		t.Fatalf("显式配置文件应能解析: %v", err)
	}
	if !strings.Contains(dsn, "go_meteor_cms") {
		t.Fatalf("解析出的 DSN 异常: %s", dsn)
	}
}

func TestResolveDSNFromCMSConfigEnv(t *testing.T) {
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("CMS_CONFIG", filepath.Join("..", "..", "api", "cms", "v1", "etc", "cms-api.yaml"))
	dsn, err := ResolveDSN("")
	if err != nil {
		t.Fatalf("CMS_CONFIG 应生效: %v", err)
	}
	if !strings.Contains(dsn, "go_meteor_cms") {
		t.Fatalf("解析出的 DSN 异常: %s", dsn)
	}
}
