// Package migration 存放需要真实 MySQL 的迁移升级集成用例。
// 用例带 `integration` build tag，普通 `go test ./...` 不会连接数据库；
// 运行时使用 `make test-integration`（需设置 MYSQL_DSN）。
package migration
