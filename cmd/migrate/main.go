// Command migrate 同步表结构、写入内置种子、记录 schema 版本。
//
// 常用组合：
//
//	go run ./cmd/migrate                       # 完整迁移（结构 + 种子 + 版本记录）
//	go run ./cmd/migrate -check                # 只检查结构漂移，退出码 2 表示有差异
//	go run ./cmd/migrate -dry-run              # 输出差异 + 期望索引清单，不写库
//	go run ./cmd/migrate -skip-seed            # 只同步表结构
//	go run ./cmd/migrate -only-seed            # 只重跑种子（表结构已就绪）
//	go run ./cmd/migrate -backfill             # 额外回填历史上传文件的媒体元数据
//	go run ./cmd/migrate -require-backup       # 校验 CMS_BACKUP_FILE 备份新鲜度后才继续
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/yuwen002/meteor-cms/ent"
	entmigrate "github.com/yuwen002/meteor-cms/ent/migrate"
	_ "github.com/yuwen002/meteor-cms/ent/runtime"
	baseconfig "github.com/yuwen002/meteor-cms/internal/config"
	"github.com/yuwen002/meteor-cms/internal/migration"
	"github.com/yuwen002/meteor-cms/internal/seed"
	"github.com/yuwen002/meteor-cms/internal/version"
)

func main() {
	// 用返回值控制退出码，保证所有 defer（释放锁、关连接）都能执行；
	// 旧实现用 log.Fatalf / os.Exit，会跳过 defer。
	os.Exit(run())
}

func run() int {
	configFile := flag.String("f", "", "配置文件路径（默认自动探测容器 / 仓库路径）")
	check := flag.Bool("check", false, "只检查结构漂移，不执行任何 DDL 或写入；退出码 2 表示有差异")
	dryRun := flag.Bool("dry-run", false, "输出结构差异与期望索引清单，不执行任何 DDL 或写入")
	skipSeed := flag.Bool("skip-seed", false, "只同步表结构，不写入种子数据")
	onlySeed := flag.Bool("only-seed", false, "只写入种子数据，不执行表结构同步")
	backfill := flag.Bool("backfill", false, "回填历史上传文件的媒体元数据")
	uploadDir := flag.String("upload-dir", "public/uploads", "回填时扫描的上传目录")
	requireBackup := flag.Bool("require-backup", false, "要求存在足够新的备份文件（CMS_BACKUP_FILE）")
	backupMaxAge := flag.Int("backup-max-age", 24, "备份文件允许的最大年龄（小时）")
	timeoutSec := flag.Int("timeout", 600, "整个迁移的超时（秒）")
	showVersion := flag.Bool("version", false, "打印构建版本后退出")
	flag.Parse()

	log.Printf("🚀 migrate %s", version.String())
	if *showVersion {
		return 0
	}
	if *skipSeed && *onlySeed {
		log.Printf("❌ -skip-seed 与 -only-seed 不能同时使用")
		return 1
	}
	if *timeoutSec <= 0 || *backupMaxAge <= 0 {
		log.Printf("❌ -timeout / -backup-max-age 必须为正数")
		return 1
	}

	dsn, err := baseconfig.ResolveDSN(*configFile)
	if err != nil {
		log.Printf("❌ %v", err)
		return 1
	}
	dbName, err := migration.DatabaseName(dsn)
	if err != nil {
		log.Printf("❌ %v", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("❌ 创建数据库连接失败: %v", err)
		return 1
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("⚠️ 关闭数据库连接失败: %v", err)
		}
	}()
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	err = db.PingContext(pingCtx)
	pingCancel()
	if err != nil {
		log.Printf("❌ 数据库连通性检查失败: %v", err)
		return 1
	}
	log.Println("✅ 数据库连接成功")

	// dry-run / check 完全只读，不需要迁移锁，也不写任何记录。
	if *check || *dryRun {
		drift, err := migration.DetectDrift(ctx, db, dbName)
		if err != nil {
			log.Printf("❌ 结构检查失败: %v", err)
			return 1
		}
		printDrift(drift, *dryRun)
		if !drift.Empty() {
			return 2
		}
		log.Println("✅ 表结构与当前代码一致，无需迁移")
		return 0
	}

	if *requireBackup {
		desc, err := migration.CheckBackup(os.Getenv("CMS_BACKUP_FILE"), time.Duration(*backupMaxAge)*time.Hour)
		if err != nil {
			log.Printf("❌ 备份校验未通过: %v", err)
			return 1
		}
		log.Printf("✅ 备份校验通过: %s", desc)
	}

	// MySQL GET_LOCK 保证多副本 / 重复 up 时只有一个进程执行迁移与种子。
	lock, err := migration.AcquireLock(ctx, db, 30*time.Second)
	if err != nil {
		log.Printf("❌ %v", err)
		return 1
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("⚠️ 关闭 Ent 客户端失败: %v", err)
		}
	}()
	// 后注册、先执行：必须在 client.Close() 关闭底层连接池之前释放锁。
	defer lock.Release()

	start := time.Now()
	if !*onlySeed {
		// 加唯一索引之前先清理历史重复权限，否则升级到本版本会建索引失败。
		removed, err := migration.DedupePermissions(ctx, db)
		if err != nil {
			log.Printf("❌ 清理重复权限失败: %v", err)
			return 1
		}
		if removed > 0 {
			log.Printf("🧹 已清理重复权限 %d 条", removed)
		}
		if err := client.Schema.Create(ctx, entmigrate.WithDropColumn(false)); err != nil {
			log.Printf("❌ 表结构同步失败（未写入种子）: %v", err)
			return 1
		}
		log.Println("✅ 表结构同步完成")
	}

	if !*skipSeed {
		tx, err := client.Tx(ctx)
		if err != nil {
			log.Printf("❌ 开启种子事务失败: %v", err)
			return 1
		}
		if err := seed.InitSeed(ctx, tx); err != nil {
			_ = tx.Rollback()
			log.Printf("❌ 初始化数据失败（事务已回滚，表结构已同步）: %v", err)
			return 1
		}
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			log.Printf("❌ 提交种子事务失败: %v", err)
			return 1
		}
		log.Println("✅ 数据初始化完成（单事务提交）")
	}

	if *backfill {
		n, err := migration.BackfillMediaFiles(ctx, client, *uploadDir)
		if err != nil {
			log.Printf("❌ 媒体元数据回填失败: %v", err)
			return 1
		}
		log.Printf("✅ 媒体元数据回填完成，新增 %d 条", n)
	}

	if !*onlySeed {
		description := fmt.Sprintf("schema=%s seed=%v backfill=%v", version.SchemaVersion, !*skipSeed, *backfill)
		if err := migration.RecordVersion(ctx, client, description, time.Since(start)); err != nil {
			log.Printf("❌ 记录 schema 版本失败: %v", err)
			return 1
		}
		log.Printf("✅ schema 版本已记录: %s（耗时 %s）", version.SchemaVersion, time.Since(start).Round(time.Millisecond))
	}

	printStats(ctx, client)
	return 0
}

func printDrift(drift migration.Drift, verbose bool) {
	log.Printf("📋 期望表 %d 张；缺失表 %d 张 / 缺失列 %d 个",
		len(entmigrate.Tables), len(drift.MissingTables), len(drift.MissingColumns))
	for _, t := range drift.MissingTables {
		log.Printf("  + 新建表 %s", t)
	}
	for _, c := range drift.MissingColumns {
		log.Printf("  + 新增列 %s", c)
	}
	if verbose {
		log.Printf("📋 期望索引 %d 个（Ent 将按需创建，不会自动删除）：", len(drift.ExpectedIndexes))
		for _, i := range drift.ExpectedIndexes {
			log.Printf("    %s", i)
		}
		log.Println("📋 以上为只读预览：未执行 DDL、未写入种子、未记录版本")
	}
}

func printStats(ctx context.Context, client *ent.Client) {
	perms, err := client.AdminPermission.Query().All(ctx)
	if err != nil {
		log.Printf("⚠️ 查询权限统计失败: %v", err)
		return
	}
	var menu, button, api int
	for _, p := range perms {
		switch p.Type {
		case seed.PermissionTypeMenu:
			menu++
		case seed.PermissionTypeButton:
			button++
		case seed.PermissionTypeAPI:
			api++
		}
	}
	log.Printf("📊 权限统计：菜单 %d / 按钮 %d / API %d（合计 %d）", menu, button, api, len(perms))
}
