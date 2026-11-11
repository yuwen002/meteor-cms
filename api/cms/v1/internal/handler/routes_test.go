package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOperationLogPrecedesPermission 固化中间件顺序：
// OperationLog 必须在 Permission 之前，否则被权限拒绝的越权尝试不会留下任何审计记录。
// 受保护链既有 routes.go 的单行数组写法，也有各模块 *routes.go 的竖排写法，这里都覆盖。
func TestOperationLogPrecedesPermission(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("扫描 handler 源码失败: %v", err)
	}
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", file, err)
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "PermissionMiddleware") {
				continue
			}
			checked++
			if strings.Contains(line, "OperationLogMiddleware") {
				if strings.Index(line, "OperationLogMiddleware") > strings.Index(line, "PermissionMiddleware") {
					t.Errorf("%s:%d 受保护中间件链必须先审计再鉴权: %s", file, i+1, strings.TrimSpace(line))
				}
				continue
			}
			if i == 0 || !strings.Contains(lines[i-1], "OperationLogMiddleware") {
				t.Errorf("%s:%d PermissionMiddleware 前缺少 OperationLogMiddleware", file, i+1)
			}
			if i+1 < len(lines) && strings.Contains(lines[i+1], "OperationLogMiddleware") {
				t.Errorf("%s:%d 审计中间件排到了鉴权之后", file, i+1)
			}
		}
	}
	if checked < 5 {
		t.Fatalf("受保护中间件链数量异常: %d", checked)
	}
}
