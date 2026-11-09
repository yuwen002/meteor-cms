package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/conf"
)

// ContainerConfigPath 是 Dockerfile 里写入运行镜像的配置文件位置。
const ContainerConfigPath = "/app/etc/cms-api.yaml"

// RepoConfigPath 是从仓库根目录执行 go run 时的相对位置。
const RepoConfigPath = "api/cms/v1/etc/cms-api.yaml"

// ResolveDSN 供 cmd/migrate 与 cmd/backup 共用的 DSN 解析。
//
// 优先级：MYSQL_DSN > -f 显式路径 > CMS_CONFIG > 容器内路径 > 仓库相对路径。
// 与旧实现的区别：不再在配置加载失败时静默回退到含弱口令的硬编码 DSN，
// 也不再依赖「当前工作目录恰好是仓库根目录」这一隐含前提。
func ResolveDSN(explicit string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("MYSQL_DSN")); v != "" {
		return v, nil
	}
	var candidates []string
	if strings.TrimSpace(explicit) != "" {
		candidates = append(candidates, explicit)
	}
	if v := strings.TrimSpace(os.Getenv("CMS_CONFIG")); v != "" {
		candidates = append(candidates, v)
	}
	candidates = append(candidates, ContainerConfigPath, RepoConfigPath)

	var lastErr error
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			lastErr = err
			continue
		}
		var c struct {
			Database struct {
				Source string
			}
		}
		if err := conf.Load(path, &c); err != nil {
			return "", fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
		}
		if strings.TrimSpace(c.Database.Source) == "" {
			return "", fmt.Errorf("配置文件 %s 缺少 Database.Source", path)
		}
		return c.Database.Source, nil
	}
	return "", fmt.Errorf("未设置 MYSQL_DSN，且未找到可用配置文件（已尝试 %v）：%v", candidates, lastErr)
}
