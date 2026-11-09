// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"os"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/mediafile"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type FileDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFileDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FileDeleteLogic {
	return &FileDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// FileDelete 删除上传目录下指定文件；目录仅允许在为空时删除，防止误删整棵目录树。
// 被文章封面或正文引用的文件禁止删除，避免产生站内死链。
func (l *FileDeleteLogic) FileDelete(req *types.FileDeleteReq) error {
	root := l.svcCtx.Config.Upload.Defaults().Dir
	target, err := utils.SafeResolve(root, req.Path)
	if err != nil {
		return common.NewBizError(common.ErrInvalidParams)
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return common.NewBizError(common.ErrFileNotFound)
		}
		return common.NewBizError(common.ErrInternalServer)
	}
	url := mediaFileURL(req.Path)
	if info.IsDir() {
		entries, rerr := os.ReadDir(target)
		if rerr != nil {
			return common.NewBizError(common.ErrInternalServer)
		}
		if len(entries) > 0 {
			return common.NewBizError(common.ErrInvalidParams)
		}
	} else {
		used, err := mediaRefCount(l.ctx, l.svcCtx.EntClient, url)
		if err != nil {
			logx.Errorf("检查媒体引用失败: %v", err)
			return common.NewBizError(common.ErrInternalServer)
		}
		if used > 0 {
			return common.NewBizErrorWithMsg(common.ErrInvalidParams, "文件正被文章引用，请先移除引用再删除")
		}
	}
	if err := os.Remove(target); err != nil {
		logx.Errorf("delete %s failed: %v", target, err)
		return common.NewBizError(common.ErrInternalServer)
	}
	// 元数据清理失败只记日志：文件已经删除，不能因此让接口误报失败
	if _, err := l.svcCtx.EntClient.MediaFile.Delete().
		Where(mediafile.URLEQ(url)).
		Exec(l.ctx); err != nil {
		logx.Errorf("删除媒体元数据失败 %s: %v", url, err)
	}
	return nil
}
