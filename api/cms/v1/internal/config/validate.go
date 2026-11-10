package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var dsnPasswordRe = regexp.MustCompile(`^([^:/]+):([^@]*)@`)

// Validate 在启动前统一校验配置。只把「必然导致运行错误」的项判为失败，
// 安全相关的弱配置由 SecurityWarnings 输出告警，避免本地开发 / E2E 因为
// 沿用了仓库里的示例密钥而无法启动。
func (c Config) Validate() error {
	if strings.TrimSpace(c.Database.Source) == "" {
		return errors.New("Database.Source 为空（可设置 MYSQL_DSN）")
	}
	if c.Database.Driver != "" && c.Database.Driver != "mysql" {
		return fmt.Errorf("Database.Driver 仅支持 mysql，当前为 %q", c.Database.Driver)
	}
	if c.Database.MaxOpenConns <= 0 || c.Database.MaxIdleConns < 0 ||
		c.Database.MaxIdleConns > c.Database.MaxOpenConns || c.Database.ConnMaxLifetime <= 0 {
		return fmt.Errorf("数据库连接池参数非法: max_open=%d max_idle=%d lifetime=%ds",
			c.Database.MaxOpenConns, c.Database.MaxIdleConns, c.Database.ConnMaxLifetime)
	}
	if strings.TrimSpace(c.Auth.AccessSecret) == "" || strings.TrimSpace(c.Auth.RefreshSecret) == "" {
		return errors.New("Auth.AccessSecret / Auth.RefreshSecret 不能为空")
	}
	if len(c.Auth.AccessSecret) < 16 || len(c.Auth.RefreshSecret) < 16 {
		return errors.New("Auth 签名密钥长度不足 16 位")
	}
	if c.Auth.AccessExpire <= 0 || c.Auth.RefreshExpire <= 0 {
		return errors.New("Auth.AccessExpire / Auth.RefreshExpire 必须为正数")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("Port 非法: %d", c.Port)
	}
	if c.Timeout <= 0 || c.MaxBytes <= 0 {
		return errors.New("Timeout / MaxBytes 必须为正数")
	}
	upload := c.Upload.Defaults()
	if c.MaxBytes <= upload.MaxSizeMB*1024*1024 {
		return fmt.Errorf("MaxBytes(%d) 必须大于 Upload.MaxSizeMB(%dMB)，否则大文件上传会被 413 提前拒绝",
			c.MaxBytes, upload.MaxSizeMB)
	}
	if c.Maintenance.TokenBlacklistCleanInterval < 0 {
		return errors.New("Maintenance.TokenBlacklistCleanInterval 不能为负")
	}
	if c.Login.MaxFailCount < 0 || c.Login.LockMinutes < 0 {
		return errors.New("Login 锁定参数不能为负")
	}
	if c.Login.RateLimit.Max <= 0 || c.Login.RateLimit.Window <= 0 || c.Login.RateLimit.BlockSeconds <= 0 ||
		c.Public.RateLimit.Max <= 0 || c.Public.RateLimit.Window <= 0 || c.Public.RateLimit.BlockSeconds <= 0 {
		return errors.New("限流的 Max / Window / BlockSeconds 必须为正数")
	}
	// CommentRate.Max<=0 是合法的「关闭评论限流」语义；开启时窗口必须为正。
	if c.Public.CommentRate.Max > 0 && c.Public.CommentRate.Window <= 0 {
		return errors.New("Public.CommentRate.Window 必须为正数")
	}
	if c.Mail.Enabled && (strings.TrimSpace(c.Mail.Host) == "" || strings.TrimSpace(c.Mail.From) == "") {
		return errors.New("Mail.Enabled=true 时必须配置 Mail.Host 与 Mail.From")
	}
	if c.Metrics.Enabled && c.Metrics.Token != "" && len(c.Metrics.Token) < 16 {
		return errors.New("METRICS_TOKEN 长度不足 16 位")
	}
	return nil
}

// SecurityWarnings 返回不影响启动、但生产环境必须处理的安全告警。
func (c Config) SecurityWarnings() []string {
	var out []string
	if c.Auth.AccessSecret == ExampleAccessSecret {
		out = append(out, "Auth.AccessSecret 仍是仓库示例值，请设置 AUTH_ACCESS_SECRET")
	}
	if c.Auth.RefreshSecret == ExampleRefreshSecret {
		out = append(out, "Auth.RefreshSecret 仍是仓库示例值，请设置 AUTH_REFRESH_SECRET")
	}
	if !c.Login.CaptchaEnabled {
		out = append(out, "登录验证码已关闭（LOGIN_CAPTCHA_ENABLED=false），生产环境必须开启")
	}
	if !c.Login.RateLimit.Enabled {
		out = append(out, "登录防爆破限流已关闭，生产环境必须开启")
	}
	if !c.Public.RateLimit.Enabled {
		out = append(out, "公开接口限流已关闭，生产环境必须开启")
	}
	if c.Public.CommentRate.Max <= 0 {
		out = append(out, "公开评论独立限流已关闭（PUBLIC_COMMENT_RATE_MAX<=0），开启评论前请恢复")
	}
	if c.Redis.Host == "" {
		out = append(out, "未配置 Redis，全局踢人 / 分布式限流降级为单实例模式")
	}
	if c.SQLDebug {
		out = append(out, "SQL_DEBUG=true，SQL 及参数会写入日志，可能包含敏感信息")
	}
	if c.Database.Source == "root:123456@tcp(127.0.0.1:3306)/go_meteor_cms?parseTime=True&loc=Local" {
		out = append(out, "Database.Source 仍是仓库默认 DSN（含弱口令），请设置 MYSQL_DSN")
	}
	if c.Metrics.Enabled && strings.TrimSpace(c.Metrics.Token) == "" {
		out = append(out, "/metrics 已开启但未设置 METRICS_TOKEN，运行时指标可被任意访问")
	}
	return out
}

// Summary 返回脱敏后的生效配置摘要，供启动日志排障使用。
func (c Config) Summary() string {
	dsn := dsnPasswordRe.ReplaceAllString(c.Database.Source, "$1:***@")
	return fmt.Sprintf(
		"port=%d timeout=%dms max_bytes=%d db_pool=%d/%d/%ds redis=%q static_dir=%q captcha=%v login_limit=%v public_limit=%v comment_rl=%d/%ds schema_strict=%v sql_debug=%v metrics=%v",
		c.Port, c.Timeout, c.MaxBytes,
		c.Database.MaxOpenConns, c.Database.MaxIdleConns, c.Database.ConnMaxLifetime,
		c.Redis.Host, c.StaticDir,
		c.Login.CaptchaEnabled, c.Login.RateLimit.Enabled, c.Public.RateLimit.Enabled,
		c.Public.CommentRate.Max, c.Public.CommentRate.Window,
		c.SchemaStrict, c.SQLDebug, c.Metrics.Enabled,
	) + " dsn=" + dsn
}
