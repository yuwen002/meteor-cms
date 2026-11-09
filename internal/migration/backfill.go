package migration

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/mediafile"
	"github.com/yuwen002/meteor-cms/internal/pkg/upload"
)

// BackfillMediaFiles 为「新表上线前就已上传」的历史文件补齐 media_files 记录。
//
// media_files 只在上传时写入，升级到带媒体库的版本后，旧文件在文件管理页里
// 没有原始文件名 / MIME / 宽高。该函数按 url 判重，可重复执行；文件本体不动。
// 目录不存在时直接返回，避免本地开发环境误报。
func BackfillMediaFiles(ctx context.Context, client *ent.Client, uploadDir string) (int, error) {
	info, err := os.Stat(uploadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("读取上传目录失败: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("上传路径不是目录: %s", uploadDir)
	}

	created := 0
	err = filepath.WalkDir(uploadDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(uploadDir, path)
		if err != nil {
			return err
		}
		url := "/uploads/" + filepath.ToSlash(rel)
		exists, err := client.MediaFile.Query().Where(mediafile.URLEQ(url)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		meta := upload.Inspect(path)
		stat, err := d.Info()
		if err != nil {
			return err
		}
		mime := meta.Mime
		if mime == "" {
			mime = "application/octet-stream"
		}
		originalName := d.Name()
		if len(originalName) > 255 {
			originalName = originalName[:255]
		}
		if _, err := client.MediaFile.Create().
			SetURL(url).
			SetOriginalName(originalName).
			SetMime(mime).
			SetSize(stat.Size()).
			SetWidth(meta.Width).
			SetHeight(meta.Height).
			Save(ctx); err != nil {
			// URL 有唯一索引，并发回填时可能撞车，视为已回填。
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				return nil
			}
			return err
		}
		created++
		return nil
	})
	if err != nil {
		return created, err
	}
	return created, nil
}
