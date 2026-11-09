// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/mediafile"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type FileRenameLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFileRenameLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FileRenameLogic {
	return &FileRenameLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// FileRename 在同级目录下重命名文件或目录，新名称不允许包含路径分隔符，且目标不得已存在。
func (l *FileRenameLogic) FileRename(req *types.FileRenameReq) error {
	if strings.TrimSpace(req.NewName) == "" || strings.ContainsAny(req.NewName, "/\\") {
		return common.NewBizError(common.ErrInvalidParams)
	}
	root := l.svcCtx.Config.Upload.Defaults().Dir
	src, err := utils.SafeResolve(root, req.Path)
	if err != nil {
		return common.NewBizError(common.ErrInvalidParams)
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return common.NewBizError(common.ErrFileNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}
	dst := filepath.Join(filepath.Dir(src), req.NewName)
	if _, err := utils.SafeResolve(root, dst); err != nil {
		return common.NewBizError(common.ErrInvalidParams)
	}
	if _, err := os.Stat(dst); err == nil {
		return common.NewBizError(common.ErrInvalidParams)
	}
	oldURL := mediaFileURL(req.Path)
	if info, err := os.Stat(src); err == nil && !info.IsDir() {
		used, err := mediaRefCount(l.ctx, l.svcCtx.EntClient, oldURL)
		if err != nil {
			logx.Errorf("检查媒体引用失败: %v", err)
			return common.NewBizError(common.ErrInternalServer)
		}
		if used > 0 {
			return common.NewBizErrorWithMsg(common.ErrInvalidParams, "文件正被文章引用，重命名会导致链接失效")
		}
	}
	if err := os.Rename(src, dst); err != nil {
		logx.Errorf("rename %s -> %s failed: %v", src, dst, err)
		return common.NewBizError(common.ErrInternalServer)
	}
	// 同步媒体表中的 URL，避免列表里元数据与磁盘文件脱节
	newURL := mediaFileURL(filepath.ToSlash(filepath.Join(filepath.Dir(req.Path), req.NewName)))
	if _, err := l.svcCtx.EntClient.MediaFile.Update().
		Where(mediafile.URLEQ(oldURL)).
		SetURL(newURL).
		Save(l.ctx); err != nil {
		logx.Errorf("同步媒体元数据 URL 失败 %s -> %s: %v", oldURL, newURL, err)
	}
	return nil
}
