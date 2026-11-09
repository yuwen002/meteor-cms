// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/config"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/middleware"
	"github.com/yuwen002/meteor-cms/ent"
	_ "github.com/yuwen002/meteor-cms/ent/runtime"
	"github.com/yuwen002/meteor-cms/internal/pkg/dicts"
	"github.com/yuwen002/meteor-cms/internal/pkg/mail"
	"github.com/yuwen002/meteor-cms/internal/pkg/permcheck"
	"github.com/yuwen002/meteor-cms/internal/pkg/settings"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type ServiceContext struct {
	Config                 config.Config
	EntClient              *ent.Client
	Redis                  *redis.Redis
	JwtMiddleware          rest.Middleware
	PermissionMiddleware   rest.Middleware
	PermissionChecker      *permcheck.Checker
	MailSender             mail.Sender
	Settings               *settings.Service
	Dicts                  *dicts.Service
	OperationLogMiddleware rest.Middleware
	ClientInfoMiddleware   rest.Middleware
}

func NewServiceContext(c config.Config, rds *redis.Redis) *ServiceContext {
	// 自己创建 sql.DB 才能设置连接池；ent.Open 使用 database/sql 默认配置（连接数无上限），
	// 生产高并发下容易把 MySQL 连接打满。这里同时做一次显式 Ping，避免「启动成功、
	// 第一个请求才报数据库连不上」。
	db, err := sql.Open("mysql", c.Database.Source)
	if err != nil {
		panic(fmt.Errorf("创建数据库连接池失败: %w", err))
	}
	db.SetMaxOpenConns(c.Database.MaxOpenConns)
	db.SetMaxIdleConns(c.Database.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(c.Database.ConnMaxLifetime) * time.Second)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		panic(fmt.Errorf("数据库连通性检查失败: %w", err))
	}

	opts := []ent.Option{ent.Driver(entsql.OpenDB(dialect.MySQL, db))}
	// ent.Debug() 会把每条 SQL 及参数明文写日志，默认关闭，仅排障时通过 SQL_DEBUG=true 打开。
	if c.SQLDebug {
		opts = append(opts, ent.Debug())
	}
	client := ent.NewClient(opts...)

	checker, err := permcheck.New(client, rds)
	if err != nil {
		panic(err)
	}

	// 未启用 SMTP 时降级为控制台输出，保证本地与未配置环境可用
	var sender mail.Sender = mail.ConsoleSender{}
	if c.Mail.Enabled {
		sender = mail.NewSMTPSender(mail.Config{
			Enabled:  true,
			Host:     c.Mail.Host,
			Port:     c.Mail.Port,
			Username: c.Mail.Username,
			Password: c.Mail.Password,
			From:     c.Mail.From,
		})
	}

	return &ServiceContext{
		Config:                 c,
		EntClient:              client,
		Redis:                  rds,
		JwtMiddleware:          middleware.NewJwtMiddleware(&c, client, rds),
		PermissionMiddleware:   middleware.NewPermissionMiddleware(&c, client, checker),
		PermissionChecker:      checker,
		MailSender:             sender,
		Settings:               settings.New(client, c.Auth.AccessSecret, rds),
		Dicts:                  dicts.New(client, rds),
		OperationLogMiddleware: middleware.NewOperationLogMiddleware(c, client),
		ClientInfoMiddleware:   middleware.NewClientInfoMiddleware().Handle,
	}
}
