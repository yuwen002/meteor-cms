// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/config"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/handler"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/middleware"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/migration"
	"github.com/yuwen002/meteor-cms/internal/pkg/audit"
	"github.com/yuwen002/meteor-cms/internal/pkg/cleanup"
	"github.com/yuwen002/meteor-cms/internal/pkg/metrics"
	"github.com/yuwen002/meteor-cms/internal/version"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "api/cms/v1/etc/cms-api.yaml", "the config file")

// uploadURLPrefix 上传文件的访问路径前缀
const uploadURLPrefix = "/uploads"

// envBool / envInt 提供环境变量覆盖配置的能力；未设置或值非法时保留配置文件的值。
// 配置不会自动映射环境变量，新增开关必须在这里显式读取，否则容器里传入的 env 不生效。
func envBool(name string, current bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return current
}

func envInt(name string, current int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return current
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return current
	}
	return n
}

func envInt64(name string, current int64) int64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return current
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return current
	}
	return n
}

func envStr(name string, current string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return current
}

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 环境变量优先：便于容器化部署时注入数据库地址与密钥，避免把机密写进镜像。
	// 配置不会自动映射环境变量，新增开关必须在这里显式读取，否则容器里传入的 env 不生效。
	c.Database.Source = envStr("MYSQL_DSN", c.Database.Source)
	c.Database.MaxOpenConns = envInt("DB_MAX_OPEN_CONNS", c.Database.MaxOpenConns)
	c.Database.MaxIdleConns = envInt("DB_MAX_IDLE_CONNS", c.Database.MaxIdleConns)
	c.Database.ConnMaxLifetime = envInt("DB_CONN_MAX_LIFETIME", c.Database.ConnMaxLifetime)
	c.StaticDir = envStr("WEB_DIST", c.StaticDir)
	c.WebBaseURL = envStr("WEB_BASE_URL", c.WebBaseURL)
	c.Redis.Host = envStr("REDIS_HOST", c.Redis.Host)
	c.Redis.Pass = envStr("REDIS_PASS", c.Redis.Pass)

	// JWT 密钥：仓库里的是示例值，生产必须通过这两个环境变量覆盖。
	c.Auth.AccessSecret = envStr("AUTH_ACCESS_SECRET", c.Auth.AccessSecret)
	c.Auth.RefreshSecret = envStr("AUTH_REFRESH_SECRET", c.Auth.RefreshSecret)
	c.Auth.AccessExpire = envInt64("AUTH_ACCESS_EXPIRE", c.Auth.AccessExpire)
	c.Auth.RefreshExpire = envInt64("AUTH_REFRESH_EXPIRE", c.Auth.RefreshExpire)
	c.Auth.RefreshGraceSeconds = envInt("AUTH_REFRESH_GRACE_SECONDS", c.Auth.RefreshGraceSeconds)

	c.Mail.Enabled = envBool("MAIL_ENABLED", c.Mail.Enabled)
	c.Mail.Host = envStr("MAIL_HOST", c.Mail.Host)
	c.Mail.Port = envInt("MAIL_PORT", c.Mail.Port)
	c.Mail.Username = envStr("MAIL_USERNAME", c.Mail.Username)
	c.Mail.Password = envStr("MAIL_PASSWORD", c.Mail.Password)
	c.Mail.From = envStr("MAIL_FROM", c.Mail.From)

	c.Maintenance.TokenBlacklistCleanInterval = envInt("MAINTENANCE_BLACKLIST_CLEAN_INTERVAL", c.Maintenance.TokenBlacklistCleanInterval)
	c.Maintenance.AuditLogRetentionDays = envInt("MAINTENANCE_AUDIT_RETENTION_DAYS", c.Maintenance.AuditLogRetentionDays)

	// 登录验证码开关：默认开启；仅本地开发 / 自动化 E2E 可关闭。
	c.Login.CaptchaEnabled = envBool("LOGIN_CAPTCHA_ENABLED", c.Login.CaptchaEnabled)
	// 登录防爆破限流：跟随配置文件（默认开启）；完整 E2E 套件会在短时间内多次登录，
	// 可通过 LOGIN_RATE_LIMIT_ENABLED=false 临时关闭，生产环境必须保持开启。
	c.Login.RateLimit.Enabled = envBool("LOGIN_RATE_LIMIT_ENABLED", c.Login.RateLimit.Enabled)
	c.Login.RateLimit.Max = envInt("LOGIN_RATE_LIMIT_MAX", c.Login.RateLimit.Max)
	c.Login.RateLimit.Window = envInt("LOGIN_RATE_LIMIT_WINDOW", c.Login.RateLimit.Window)
	c.Login.RateLimit.BlockSeconds = envInt("LOGIN_RATE_LIMIT_BLOCK", c.Login.RateLimit.BlockSeconds)
	// 公开接口限流：默认开启，防止免鉴权接口被刷。
	c.Public.RateLimit.Enabled = envBool("PUBLIC_RATE_LIMIT_ENABLED", c.Public.RateLimit.Enabled)
	c.Public.RateLimit.Max = envInt("PUBLIC_RATE_LIMIT_MAX", c.Public.RateLimit.Max)
	c.Public.RateLimit.Window = envInt("PUBLIC_RATE_LIMIT_WINDOW", c.Public.RateLimit.Window)
	c.Public.RateLimit.BlockSeconds = envInt("PUBLIC_RATE_LIMIT_BLOCK", c.Public.RateLimit.BlockSeconds)
	// 公开评论独立配额：feature_comment_enabled 打开后防止单 IP 灌水。
	c.Public.CommentRate.Max = envInt("PUBLIC_COMMENT_RATE_MAX", c.Public.CommentRate.Max)
	c.Public.CommentRate.Window = envInt("PUBLIC_COMMENT_RATE_WINDOW", c.Public.CommentRate.Window)

	c.SQLDebug = envBool("SQL_DEBUG", c.SQLDebug)
	c.StrictSecurity = envBool("STRICT_SECURITY", c.StrictSecurity)
	c.SchemaStrict = envBool("SCHEMA_CHECK_STRICT", c.SchemaStrict)
	c.Metrics.Enabled = envBool("METRICS_ENABLED", c.Metrics.Enabled)
	c.Metrics.Token = envStr("METRICS_TOKEN", c.Metrics.Token)

	if err := c.Validate(); err != nil {
		logx.Errorf("配置校验失败: %v", err)
		os.Exit(1)
	}
	warnings := c.SecurityWarnings()
	if c.StrictSecurity && len(warnings) > 0 {
		logx.Errorf("STRICT_SECURITY=true，存在 %d 项安全配置问题，拒绝启动: %v", len(warnings), warnings)
		os.Exit(1)
	}
	for _, w := range warnings {
		logx.Infof("[安全告警] %s", w)
	}
	logx.Infof("启动版本 %s", version.String())
	logx.Infof("生效配置 %s", c.Summary())
	logx.Infof("[安全告警] 默认管理员 admin/123456 为初始化弱口令，请在首次登录后立即修改")

	// 静态资源：让 public/uploads 下的文件可通过 /uploads/xxx 访问。
	// WithFileServer 是 RunOption，必须在创建 server 时传入；它仅拦截 GET 且
	// 路径以 /uploads/ 前缀开头且文件存在的请求，其余请求仍走 API 路由。
	uploadDir := c.Upload.Defaults().Dir

	// ent client 与 settings 服务都在 ServiceContext 里创建，但 SPAHandler 必须在
	// MustNewServer 时构造。用闭包延迟绑定：只在请求到达时读取，此时依赖均已赋值。
	var entClient *ent.Client
	var ctx *svc.ServiceContext
	opts := []rest.RunOption{
		rest.WithFileServer(uploadURLPrefix, http.Dir(uploadDir)),
		// 若配置了前端产物目录，则通过自定义 NotFoundHandler 直接托管 SPA（含 history 路由回退）。
		// 不能用 server.Use 注册：go-zero 的全局中间件不会作用于 404（未匹配）路由，
		// 而 notFoundHandler 在 MustNewServer 时即绑定中间件链，早于 server.Use 调用。
		rest.WithNotFoundHandler(middleware.SPAHandler(
			c.StaticDir,
			middleware.WithPageResolver(func(slug string) (middleware.PageData, bool) {
				if entClient == nil {
					return middleware.PageData{}, false
				}
				return newArticlePageResolver(entClient)(slug)
			}),
			middleware.WithSiteValues(func(reqCtx context.Context) map[string]string {
				if ctx == nil {
					return nil
				}
				return ctx.Settings.All(reqCtx)
			}),
		)),
	}
	if len(c.Cors) > 0 {
		opts = append(opts, rest.WithCors(c.Cors...))
	}
	server := rest.MustNewServer(c.RestConf, opts...)
	defer server.Stop()

	// 将登出请求携带的 X-Refresh-Token 透传进 context，供登出接口一并吊销
	server.Use(middleware.NewRefreshTokenHeaderMiddleware())
	// 进程级请求指标（总数 / 5xx / 在途），供 /admin/monitor 读取
	server.Use(metrics.Middleware)

	// 登录限流 / token 版本（全局踢人）：配置了 Redis 则启用分布式能力，
	// 否则（含 Redis 不可用）自动降级为内存限流 / 不校验版本，防护不失效。
	var rds *redis.Redis
	if c.Redis.Host != "" {
		if r, err := redis.NewRedis(c.Redis); err != nil {
			logx.Errorf("Redis 初始化失败，登录限流将使用内存模式: %v", err)
		} else {
			rds = r
		}
	}
	server.Use(middleware.NewLoginRateLimitMiddleware(
		rds,
		c.Login.RateLimit.Enabled,
		c.Login.RateLimit.Max,
		c.Login.RateLimit.Window,
		c.Login.RateLimit.BlockSeconds,
		func(r *http.Request) {
			// 被限流拦截的登录请求不会进入 LoginLogic，需在此补记失败登录日志，
			// 否则爆破尝试在审计里完全不可见。entClient 在请求到达时已赋值。
			if entClient != nil {
				middleware.RecordRateLimitedLogin(r, entClient)
			}
		},
	))
	// 公开接口限流：只拦截 /api/ 前缀，后台接口与静态资源不受影响。
	server.Use(middleware.NewPublicRateLimitMiddleware(
		rds,
		c.Public.RateLimit.Enabled,
		c.Public.RateLimit.Max,
		c.Public.RateLimit.Window,
		c.Public.RateLimit.BlockSeconds,
	))

	ctx = svc.NewServiceContext(c, rds)
	entClient = ctx.EntClient

	// 启动即校验 schema 版本：漏跑 migrate 时直接拒绝启动，而不是等到运行时才
	// 以 "Unknown column" 之类的 SQL 错误暴露。紧急回滚可设 SCHEMA_CHECK_STRICT=false。
	schemaStatus, err := migration.EnsureVersion(context.Background(), ctx.EntClient, c.SchemaStrict)
	if err != nil {
		logx.Errorf("schema 版本校验失败: %v", err)
		server.Stop()
		os.Exit(1)
	}
	logx.Infof("schema 版本 %s（期望 %s，摘要匹配=%v）",
		schemaStatus.Version, schemaStatus.ExpectedVersion,
		schemaStatus.Checksum == schemaStatus.ExpectedChecksum)

	// 预热进程内缓存，避免首个请求同时触发配置 / 字典 / 权限查库。
	warmCtx, cancelWarm := context.WithTimeout(context.Background(), 3*time.Second)
	settingsCount := len(ctx.Settings.All(warmCtx))
	_ = ctx.Dicts.Items(warmCtx, "config_group")
	cancelWarm()
	logx.Infof("缓存预热完成：站点配置 %d 项", settingsCount)

	handler.RegisterHandlers(server, ctx)
	// 文章发布模块独立注册：routes.go 是 goctl 生成文件且被手工追加过，
	// 单独注册可避免重跑 make gen-api 时丢失路由。
	handler.RegisterContentRoutes(server, ctx)
	handler.RegisterLogRoutes(server, ctx)
	handler.RegisterSystemExtraRoutes(server, ctx)
	handler.RegisterPublicSiteRoutes(server, ctx)
	handler.RegisterSiteExtRoutes(server, ctx)
	handler.RegisterMetricsRoutes(server, c.Metrics.Enabled, c.Metrics.Token)
	handler.RegisterSessionRoutes(server, ctx)

	// 启动后台维护任务，随服务停止而退出；WaitGroup 保证退出时在途清理不会被截断。
	taskCtx, cancelTasks := context.WithCancel(context.Background())
	defer cancelTasks()
	var cleanupWG sync.WaitGroup
	cleanup.StartTokenBlacklistCleaner(
		taskCtx,
		ctx.EntClient,
		time.Duration(c.Maintenance.TokenBlacklistCleanInterval)*time.Second,
		&cleanupWG,
	)
	// 操作/登录日志定期清理（retentionDays<=0 时内部不启动，永久保留）
	cleanup.StartAuditLogCleaner(taskCtx, ctx.EntClient, c.Maintenance.AuditLogRetentionDays, rds, &cleanupWG)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()

	// Start 阻塞至服务停止；此时不会再有新请求，按「维护任务 → 审计队列 → 数据库连接」
	// 的顺序收尾，每一步都有超时兜底，避免优雅关闭变成永久挂起。
	cancelTasks()
	done := make(chan struct{})
	go func() {
		cleanupWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		logx.Info("后台维护任务已全部退出")
	case <-time.After(10 * time.Second):
		logx.Info("等待后台维护任务超时（10s），继续关闭")
	}
	audit.Shutdown(5 * time.Second)
	if err := ctx.EntClient.Close(); err != nil {
		logx.Errorf("关闭数据库连接失败: %v", err)
	}
}
