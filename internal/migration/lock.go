package migration

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const migrateLockName = "meteor_cms_schema_migrate"

// MysqlLock 基于 MySQL 会话级 GET_LOCK。锁与连接绑定，必须始终在同一个 *sql.Conn
// 上获取与释放；进程退出或连接断开时 MySQL 会自动释放，不会出现永久死锁。
type MysqlLock struct {
	conn *sql.Conn
}

// AcquireLock 获取迁移锁。db 由调用方持有并在 Release 之后自行关闭。
func AcquireLock(ctx context.Context, db *sql.DB, wait time.Duration) (*MysqlLock, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取迁移专用连接失败: %w", err)
	}
	seconds := int(wait.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", migrateLockName, seconds).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("获取迁移锁失败: %w", err)
	}
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return nil, fmt.Errorf("另一个迁移任务正在执行（等待 %ds 未获得锁 %s）", seconds, migrateLockName)
	}
	return &MysqlLock{conn: conn}, nil
}

// Release 释放锁并归还连接。
func (l *MysqlLock) Release() {
	if l == nil || l.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = l.conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", migrateLockName)
	_ = l.conn.Close()
}
