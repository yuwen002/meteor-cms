// Command backup 导出/恢复 CMS 内容快照，供 make backup / make restore 使用。
//
// 这是逻辑备份：用 Ent 读取内容/分类/标签/配置/字典并写成可移植 JSON，
// 不依赖 mysqldump，也不包含管理员密码哈希、令牌黑名单、审计日志等敏感表。
// 完整物理备份仍应使用数据库自带的 dump 工具；本命令用于站点迁移与内容回滚。
//
// 用法：
//
//	go run ./cmd/backup -mode export [-file backup/xxx.json]
//	go run ./cmd/backup -mode import -file backup/xxx.json [-overwrite]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yuwen002/meteor-cms/ent"
	baseconfig "github.com/yuwen002/meteor-cms/internal/config"
	"github.com/yuwen002/meteor-cms/internal/pkg/contentbundle"
	"github.com/yuwen002/meteor-cms/internal/version"
)

func main() {
	mode := flag.String("mode", "export", "export（导出）或 import（恢复）")
	file := flag.String("file", "", "快照文件路径；导出时省略则自动生成带时间戳的文件名")
	configFile := flag.String("f", "", "配置文件路径（默认自动探测容器 / 仓库路径）")
	overwrite := flag.Bool("overwrite", false, "恢复时是否覆盖同 slug 的文章")
	showVersion := flag.Bool("version", false, "打印构建版本后退出")
	flag.Parse()

	if *showVersion {
		fmt.Printf("backup %s\n", version.String())
		return
	}
	log.Printf("backup %s", version.String())

	dsn, err := baseconfig.ResolveDSN(*configFile)
	if err != nil {
		log.Fatalf("解析数据库配置失败: %v", err)
	}

	if *mode != "export" && *mode != "import" {
		log.Fatal("-mode 只能是 export 或 import")
	}
	if *mode == "import" && *file == "" {
		log.Fatal("恢复必须显式指定 -file，避免误操作")
	}
	if *file == "" {
		*file = filepath.Join("backup", "cms-content-"+time.Now().Format("20060102-150405")+".json")
	}

	client, err := ent.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	switch *mode {
	case "export":
		if err := os.MkdirAll(filepath.Dir(*file), 0o755); err != nil {
			log.Fatalf("创建备份目录失败: %v", err)
		}
		bundle, err := contentbundle.Export(ctx, client)
		if err != nil {
			log.Fatalf("读取内容失败: %v", err)
		}
		f, err := os.Create(*file)
		if err != nil {
			log.Fatalf("创建备份文件失败: %v", err)
		}
		if err := contentbundle.Write(f, bundle); err != nil {
			_ = f.Close()
			log.Fatalf("写入备份失败: %v", err)
		}
		if err := f.Close(); err != nil {
			log.Fatalf("关闭备份文件失败: %v", err)
		}
		fmt.Printf("导出完成: %s（分类 %d / 标签 %d / 文章 %d / 配置 %d / 字典 %d）\n",
			*file, len(bundle.Categories), len(bundle.Tags), len(bundle.Articles),
			len(bundle.Configs), len(bundle.Dicts))

	case "import":
		f, err := os.Open(*file)
		if err != nil {
			log.Fatalf("打开备份文件失败: %v", err)
		}
		defer func() { _ = f.Close() }()
		bundle, err := contentbundle.Decode(f)
		if err != nil {
			log.Fatalf("解析备份文件失败: %v", err)
		}
		res, err := contentbundle.Import(ctx, client, bundle, *overwrite)
		if err != nil {
			log.Fatalf("恢复失败（已回滚）: %v", err)
		}
		fmt.Printf("恢复完成: 分类 +%d/%d跳过, 标签 +%d/%d跳过, 文章 +%d新建/%d覆盖/%d跳过, 配置 +%d/%d跳过, 字典 +%d/%d跳过, 字典项 +%d/%d跳过\n",
			res.CategoriesCreated, res.CategoriesSkipped,
			res.TagsCreated, res.TagsSkipped,
			res.ArticlesCreated, res.ArticlesUpdated, res.ArticlesSkipped,
			res.ConfigsCreated, res.ConfigsSkipped,
			res.DictsCreated, res.DictsSkipped,
			res.DictItemsCreated, res.DictItemsSkipped)
	}
}
