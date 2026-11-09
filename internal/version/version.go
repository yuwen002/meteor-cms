// Package version 集中管理构建期注入的版本信息与期望的数据库 schema 版本。
package version

import "fmt"

// 以下变量由 Dockerfile / 发布流水线通过 -ldflags -X 注入；
// 本地 go run / go test 时使用这里的默认值。
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// SchemaVersion 是当前代码所期望的数据库 schema 版本。
// 引入需要显式迁移或数据回填的结构变更时递增；cmd/migrate 会把该值写入
// schema_migrations，API 启动时据此校验，避免漏跑迁移导致运行时才报 SQL 错误。
const SchemaVersion = "1"

// String 返回可写入启动日志的单行版本描述。
func String() string {
	return fmt.Sprintf("%s (commit=%s, built=%s, schema=%s)", Version, Commit, BuildTime, SchemaVersion)
}
