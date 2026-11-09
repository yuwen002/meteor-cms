// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/upload"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

// uploadFormKey 上传表单的文件字段名
const uploadFormKey = "file"

type UploadFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadFileLogic {
	return &UploadFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UploadFile 接收单个文件并落盘到配置指定的目录
func (l *UploadFileLogic) UploadFile() (resp *types.UploadFileResp, err error) {
	// goctl 生成的 handler 没有绑定 multipart 入参，这里从 ClientInfoMiddleware 注入的请求中取文件
	httpReq := utils.GetRequest(l.ctx)
	if httpReq == nil {
		l.Logger.Error("无法从上下文中获取 HTTP 请求")
		return nil, common.NewBizError(common.ErrInvalidParams)
	}

	file, header, err := httpReq.FormFile(uploadFormKey)
	if err != nil {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, "请选择要上传的文件")
	}
	defer file.Close()

	cfg := l.svcCtx.Config.Upload.Defaults()

	info, err := upload.Save(header, cfg.Dir, cfg.MaxSizeMB, cfg.AllowedExts)
	if err != nil {
		l.Logger.Errorf("保存上传文件失败: %v", err)
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, err.Error())
	}

	// 采集媒体元数据。元数据写入失败只记日志，不能让已经落盘的文件变成"上传失败"。
	meta := upload.Meta{}
	if path, perr := upload.LocalPath(cfg.Dir, info.URL); perr == nil {
		meta = upload.Inspect(path)
	}
	create := l.svcCtx.EntClient.MediaFile.Create().
		SetURL(info.URL).
		SetOriginalName(header.Filename).
		SetMime(meta.Mime).
		SetSize(info.Size).
		SetWidth(meta.Width).
		SetHeight(meta.Height)
	if uid, ok := utils.CurrentUserID(l.ctx); ok && uid > 0 {
		create = create.SetUploaderID(uid)
	}
	if _, err := create.Save(l.ctx); err != nil {
		l.Logger.Errorf("写入媒体元数据失败 %s: %v", info.URL, err)
	}

	return &types.UploadFileResp{
		URL:      info.URL,
		Filename: info.Filename,
		Size:     info.Size,
	}, nil
}
