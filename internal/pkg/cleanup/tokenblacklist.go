// Package cleanup 提供后台维护任务。
package cleanup

import (
	"context"
	"sync"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/tokenblacklist"
	"github.com/zeromicro/go-zero/core/logx"
)

// DefaultCleanInterval 默认清理间隔
const DefaultCleanInterval = time.Hour

// PurgeExpiredTokenBlacklist 删除已过期的黑名单记录。
//
// 黑名单只用于拦截「登出后仍在有效期内的 token」，
// 一旦记录中的 expired_at 早于当前时间，该 token 本身也已失效，记录可安全删除，
// 否则表会随每次登出无限增长。
func PurgeExpiredTokenBlacklist(ctx context.Context, client *ent.Client) (int, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return client.TokenBlacklist.
		Delete().
		Where(tokenblacklist.ExpiredAtLT(time.Now())).
		Exec(timeoutCtx)
}

// StartTokenBlacklistCleaner 启动黑名单过期清理任务。
//
// 启动时立即执行一次，之后按 interval 周期执行；ctx 取消后任务退出。
// interval <= 0 时使用 DefaultCleanInterval。
func StartTokenBlacklistCleaner(ctx context.Context, client *ent.Client, interval time.Duration, wg *sync.WaitGroup) {
	if interval <= 0 {
		interval = DefaultCleanInterval
	}
	wg.Add(1)

	go func() {
		defer wg.Done()
		run := func() {
			deleted, err := PurgeExpiredTokenBlacklist(ctx, client)
			if err != nil {
				logx.Errorf("清理过期 token 黑名单失败: %v", err)
				return
			}
			if deleted > 0 {
				logx.Infof("已清理过期 token 黑名单 %d 条", deleted)
			}
		}

		// 启动即清理一次，避免长期不重启时历史数据堆积
		run()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
