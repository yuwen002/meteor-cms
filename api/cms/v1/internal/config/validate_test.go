package config

import (
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// loadFixture 读取服务实际使用的配置文件，保证测试校验的就是生产默认配置。
func loadFixture(t *testing.T) Config {
	t.Helper()
	var c Config
	if err := conf.Load("../../etc/cms-api.yaml", &c); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	return c
}

func TestDefaultConfigPassesValidation(t *testing.T) {
	c := loadFixture(t)
	if err := c.Validate(); err != nil {
		t.Fatalf("随仓库分发的默认配置应能通过启动校验: %v", err)
	}
}

// 新增字段不依赖 go-zero 的 default tag，全部由 yaml 显式给出，这里防止改配置时漏项。
func TestExplicitDefaultsAreLoaded(t *testing.T) {
	c := loadFixture(t)
	if c.Database.MaxOpenConns != 20 || c.Database.MaxIdleConns != 10 || c.Database.ConnMaxLifetime != 3600 {
		t.Fatalf("连接池默认值未加载: %+v", c.Database)
	}
	if !c.SchemaStrict {
		t.Fatal("SchemaStrict 应默认开启，否则漏跑迁移不会被发现")
	}
	if c.SQLDebug || c.StrictSecurity {
		t.Fatal("SQLDebug / StrictSecurity 应默认关闭")
	}
	if c.Public.CommentRate.Max != 5 || c.Public.CommentRate.Window != 60 {
		t.Fatalf("评论限流默认值未加载: %+v", c.Public.CommentRate)
	}
}

func TestValidateRejectsBrokenValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"空 DSN", func(c *Config) { c.Database.Source = "" }},
		{"错误驱动", func(c *Config) { c.Database.Driver = "postgres" }},
		{"连接池非法", func(c *Config) { c.Database.MaxIdleConns = c.Database.MaxOpenConns + 1 }},
		{"短密钥", func(c *Config) { c.Auth.AccessSecret = "short" }},
		{"零过期时间", func(c *Config) { c.Auth.AccessExpire = 0 }},
		{"非法端口", func(c *Config) { c.Port = 0 }},
		{"请求体上限小于上传上限", func(c *Config) { c.MaxBytes = 1024 }},
		{"邮件启用但缺 Host", func(c *Config) { c.Mail.Enabled = true; c.Mail.Host = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := loadFixture(t)
			tc.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("非法配置应被拒绝")
			}
		})
	}
}

func TestSecurityWarningsCoverExampleSecrets(t *testing.T) {
	c := loadFixture(t)
	joined := strings.Join(c.SecurityWarnings(), "\n")
	for _, want := range []string{"AUTH_ACCESS_SECRET", "AUTH_REFRESH_SECRET"} {
		if !strings.Contains(joined, want) {
			t.Errorf("示例密钥应产生安全告警: %s", want)
		}
	}
}

// scripts/preflight.sh 的检查项与严格模式启动校验逐项对齐：合规的生产配置
// 在 STRICT_SECURITY=true 下必须零告警，否则 API 会直接拒绝启动。
func TestProductionConfigHasNoSecurityWarnings(t *testing.T) {
	c := loadFixture(t)
	c.Auth.AccessSecret = "prod-access-secret-2026-abcdefgh"
	c.Auth.RefreshSecret = "prod-refresh-secret-2026-ijklmnop"
	c.Database.Source = "cms:Str0ngPass@tcp(mysql:3306)/go_meteor_cms?parseTime=True&loc=Local"
	c.Redis.Host = "redis:6379"
	c.StrictSecurity = true
	c.SQLDebug = false
	c.Login.CaptchaEnabled = true
	c.Login.RateLimit.Enabled = true
	c.Public.RateLimit.Enabled = true
	c.Public.CommentRate.Max = 5
	if err := c.Validate(); err != nil {
		t.Fatalf("生产配置应能通过启动校验: %v", err)
	}
	if warnings := c.SecurityWarnings(); len(warnings) != 0 {
		t.Fatalf("生产配置不应有安全告警（严格模式会据此拒绝启动）: %v", warnings)
	}
}

func TestSummaryMasksDSNPassword(t *testing.T) {
	c := loadFixture(t)
	s := c.Summary()
	if strings.Contains(s, "root:123456@") {
		t.Fatalf("生效配置摘要泄露了 DSN 密码: %s", s)
	}
	if !strings.Contains(s, "***@") {
		t.Fatalf("生效配置摘要未标注脱敏: %s", s)
	}
}
