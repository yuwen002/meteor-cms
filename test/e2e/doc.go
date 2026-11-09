// Package e2e 存放需要真实 API 服务的端到端用例。
// 用例文件带 `e2e` build tag，普通 `go test ./...` 不会连接外部服务；
// 运行时使用 `go test -tags=e2e ./test/e2e/...`。
package e2e
