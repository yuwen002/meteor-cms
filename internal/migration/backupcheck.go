package migration

import (
	"fmt"
	"os"
	"time"
)

// CheckBackup 在 -require-backup 时确认存在一份足够新的备份，防止生产裸跑 DDL。
// 路径由 CMS_BACKUP_FILE 指定；不指定则拒绝继续。
func CheckBackup(path string, maxAge time.Duration) (string, error) {
	if path == "" {
		return "", fmt.Errorf("启用 -require-backup 时必须通过 CMS_BACKUP_FILE 指定备份文件")
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("读取备份文件失败: %w", err)
	}
	if st.IsDir() || st.Size() == 0 {
		return "", fmt.Errorf("备份文件 %s 不是有效的非空文件", path)
	}
	age := time.Since(st.ModTime())
	if age > maxAge {
		return "", fmt.Errorf("备份 %s 已过期（%.1f 小时前，要求 %s 内），请先执行 make backup",
			path, age.Hours(), maxAge)
	}
	return fmt.Sprintf("%s (%.1f KB, %.1f 小时前)", path, float64(st.Size())/1024, age.Hours()), nil
}
