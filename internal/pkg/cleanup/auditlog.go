package cleanup

import (
	"context"
	"sync"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/loginlog"
	"github.com/yuwen002/meteor-cms/ent/operationlog"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// auditLogCleanInterval 审计日志清理周期
	auditLogCleanInterval = 24 * time.Hour
	// auditLogPurgeBatch 每批删除条数，避免 90 天数据一次性 DELETE 造成大事务与主从延迟
	auditLogPurgeBatch = 1000
	// auditLogLockKey 多实例部署时的选主锁；TTL 远大于单次清理耗时
	auditLogLockKey = "maintenance:audit_log_cleaner"
	auditLogLockTTL = 3600
)

// auditLogLockLua 原子抢锁，只有首个实例返回 1。
var auditLogLockLua = `
if redis.call('setnx', KEYS[1], '1') == 1 then
  redis.call('expire', KEYS[1], ARGV[1])
  return 1
end
return 0
`

// PurgeOldOperationLogs 分批删除早于 retentionDays 天的操作日志。
// retentionDays <= 0 时直接返回，表示永久保留。
func PurgeOldOperationLogs(ctx context.Context, client *ent.Client, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	total := 0
	for {
		qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		ids, err := client.OperationLog.Query().
			Where(operationlog.CreatedAtLT(cutoff)).
			Order(ent.Asc(operationlog.FieldID)).
			Limit(auditLogPurgeBatch).
			IDs(qctx)
		cancel()
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}

		dctx, dcancel := context.WithTimeout(ctx, 30*time.Second)
		n, err := client.OperationLog.Delete().Where(operationlog.IDIn(ids...)).Exec(dctx)
		dcancel()
		if err != nil {
			return total, err
		}
		total += n
		if len(ids) < auditLogPurgeBatch {
			return total, nil
		}
	}
}

// PurgeOldLoginLogs 分批删除早于 retentionDays 天的登录日志。
func PurgeOldLoginLogs(ctx context.Context, client *ent.Client, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	total := 0
	for {
		qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		ids, err := client.LoginLog.Query().
			Where(loginlog.LoginAtLT(cutoff)).
			Order(ent.Asc(loginlog.FieldID)).
			Limit(auditLogPurgeBatch).
			IDs(qctx)
		cancel()
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}

		dctx, dcancel := context.WithTimeout(ctx, 30*time.Second)
		n, err := client.LoginLog.Delete().Where(loginlog.IDIn(ids...)).Exec(dctx)
		dcancel()
		if err != nil {
			return total, err
		}
		total += n
		if len(ids) < auditLogPurgeBatch {
			return total, nil
		}
	}
}

// acquireCleanupLock 用 Redis 做选主，避免多副本各自对同一批数据发起 DELETE。
// rds 为 nil 或 Redis 故障时退化为单实例执行（重复删除本身幂等，不停摆优先）。
func acquireCleanupLock(rds *redis.Redis) bool {
	res, err := rds.Eval(auditLogLockLua, []string{auditLogLockKey}, auditLogLockTTL)
	if err != nil {
		logx.Errorf("获取审计日志清理锁失败，退化为单实例执行: %v", err)
		return true
	}
	switch v := res.(type) {
	case int64:
		return v == 1
	case int:
		return v == 1
	case float64:
		return v == 1
	default:
		logx.Errorf("审计日志清理锁返回值异常 (%T %v)，退化为单实例执行", res, res)
		return true
	}
}

// StartAuditLogCleaner 启动审计日志定期清理任务。
// 启动时立即执行一次，之后每 24 小时执行；retentionDays <= 0 时不启动（永久保留）。
func StartAuditLogCleaner(ctx context.Context, client *ent.Client, retentionDays int, rds *redis.Redis, wg *sync.WaitGroup) {
	if retentionDays <= 0 {
		return
	}
	wg.Add(1)

	go func() {
		defer wg.Done()
		run := func() {
			if rds != nil && !acquireCleanupLock(rds) {
				logx.Infof("审计日志清理由其它实例执行，本次跳过")
				return
			}

			// 两张表独立处理：其中一张失败不应让另一张的清理被跳过。
			opDeleted, err := PurgeOldOperationLogs(ctx, client, retentionDays)
			if err != nil {
				logx.Errorf("清理过期操作日志失败: %v", err)
			}
			loginDeleted, err := PurgeOldLoginLogs(ctx, client, retentionDays)
			if err != nil {
				logx.Errorf("清理过期登录日志失败: %v", err)
			}
			if opDeleted+loginDeleted > 0 {
				logx.Infof("已清理过期审计日志 操作日志 %d 条 / 登录日志 %d 条", opDeleted, loginDeleted)
			}
		}

		run()

		ticker := time.NewTicker(auditLogCleanInterval)
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
