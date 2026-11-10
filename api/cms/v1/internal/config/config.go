// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package config

import (
	baseconfig "github.com/yuwen002/meteor-cms/internal/config"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	// Cors 允许跨域访问的前端源列表（前端独立域名部署时配置），如 ["https://admin.example.com"]；
	// 含 "*" 表示允许任意源（仅适合无 cookie 凭据、纯 token 鉴权的场景）
	Cors []string `json:",optional"`
	// Redis 登录限流的分布式存储；Host 为空时登录限流自动降级为单实例内存模式
	Redis    redis.RedisConf `json:",optional"`
	Database struct {
		Driver string
		Source string
		// 连接池参数。database/sql 默认不限制连接数，生产环境高并发下容易把 MySQL 打满，
		// 因此在这里显式给默认值，并允许通过 DB_MAX_* 环境变量覆盖。
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime int // 秒；默认值由 etc/cms-api.yaml 显式给出
	}
	Auth struct {
		AccessSecret  string
		AccessExpire  int64
		RefreshSecret string // 刷新令牌专用密钥，与 AccessSecret 区分，避免 access 泄露后 refresh 被冒用
		RefreshExpire int64  `json:",default=604800"` // 刷新令牌有效期，单位秒（默认 7 天）
		// RefreshGraceSeconds 刷新令牌轮换后的并发宽限期（秒），默认 5。
		// 轮换会把旧令牌签入黑名单，同一旧令牌在宽限期内重复提交仍被接受，避免并发请求误登出；
		// 超出宽限期判定为盗用重放并拒绝。
		RefreshGraceSeconds int `json:",default=5"`
	}
	Mail struct {
		Enabled  bool
		Host     string
		Port     int
		Username string
		Password string
		From     string
	}
	Maintenance struct {
		// TokenBlacklistCleanInterval 过期 token 黑名单的清理间隔（秒），<=0 时按 3600 秒处理
		TokenBlacklistCleanInterval int
		// AuditLogRetentionDays 操作/登录日志保留天数，<=0 表示永久保留（不清理）
		AuditLogRetentionDays int `json:",default=90"`
	}
	Login struct {
		// MaxFailCount 连续登录失败达到该次数后锁定账号，<=0 时不启用锁定
		MaxFailCount int `json:",default=5"`
		// LockMinutes 锁定持续时间（分钟），<=0 时按 15 处理
		LockMinutes int `json:",default=15"`
		// CaptchaEnabled 登录是否强制校验验证码。默认 true；
		// 仅本地开发与自动化 E2E 可通过 LOGIN_CAPTCHA_ENABLED=false 关闭，生产环境务必保持开启。
		CaptchaEnabled bool `json:",default=true"`
		// RateLimit 登录接口按客户端 IP 的限流配置（防爆破）；默认关闭，需显式开启
		RateLimit struct {
			Enabled      bool `json:",default=false"`
			Max          int  `json:",default=10"`  // 窗口内允许的最大尝试次数
			Window       int  `json:",default=60"`  // 统计窗口（秒）
			BlockSeconds int  `json:",default=300"` // 触发限流后拒绝时长（秒）
		}
	}
	// Public 公开（免鉴权）接口的配置
	Public struct {
		// RateLimit 按客户端 IP 限制 /api/ 前缀公开接口的访问频率，防止被刷。
		// 默认开启；Redis 不可用时自动降级为单实例内存限流。
		RateLimit struct {
			Enabled      bool `json:",default=true"`
			Max          int  `json:",default=60"` // 窗口内允许的最大请求数
			Window       int  `json:",default=60"` // 统计窗口（秒）
			BlockSeconds int  `json:",default=60"` // 触发限流后拒绝时长（秒）
		}
		// CommentRate 公开评论提交的独立配额。全局限流是 /api/ 共享的，
		// 评论功能开启后需要更严格的写入配额；默认值由 etc/cms-api.yaml 显式给出。
		CommentRate struct {
			Max    int // 窗口内允许提交的最大评论数，<=0 表示不限流
			Window int // 统计窗口（秒）
		}
	}
	// WebBaseURL 前端站点地址，用于拼接找回密码邮件链接与文章 canonical / RSS / sitemap 绝对地址
	WebBaseURL string
	// StaticDir 前端构建产物（dist）目录；非空时由本服务直接托管 SPA（含 history 路由回退）。
	// 容器化部署通常通过环境变量 WEB_DIST 注入，本地开发留空即可（仅暴露 API）。
	// optional：仅托管 API 时配置文件里可以不写这一项，否则 go-zero 严格解析会直接启动失败。
	StaticDir string `json:",optional"`
	Upload    baseconfig.Upload

	// 运行开关：均可在 cms.go 里通过同名大写环境变量覆盖。
	// SQLDebug 默认关闭；生产若显式打开会把每条 SQL（含参数）写入日志，需谨慎。
	SQLDebug bool
	// StrictSecurity=true 时把「示例密钥 / 关闭限流」等告警升级为启动失败。
	StrictSecurity bool
	// SchemaStrict=true 时启动即校验 schema_migrations 的当前版本。
	SchemaStrict bool
	// Metrics 控制 Prometheus 抓取端点 /metrics。默认关闭，避免运行时信息对外暴露。
	Metrics struct {
		// Enabled 是否暴露 /metrics；关闭时端点返回 404，无需重新构建即可开关。
		Enabled bool
		// Token 非空时要求 Authorization: Bearer <Token>；为空表示仅靠网络层保护。
		Token string `json:",optional"`
	}
}

// 与 cms-api.yaml 中随仓库分发的示例值一致。生产必须通过环境变量覆盖，
// 否则所有实例的 JWT 签名密钥相同，任何拿到仓库的人都能伪造管理员令牌。
const (
	ExampleAccessSecret  = "8x!A%D*G-KaPdSgVkYp3s6v9y$B?E(H+MbQeThWmZq4t7w!z%C*F-J@NcRfUjXn2"
	ExampleRefreshSecret = "rEfReSh-6KpVq2Jx!YmN3sLz8Wb@TcE5HgU7DfA1"
)
