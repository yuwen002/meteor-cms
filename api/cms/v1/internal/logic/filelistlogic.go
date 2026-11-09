// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/mediafile"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

const uploadURLPrefix = "/uploads"

type FileListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFileListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FileListLogic {
	return &FileListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// FileList 列出上传目录下指定子目录中的文件，支持关键字过滤与分页。
// 目录在前、其次按名称排序；单页上限 100。
// 媒体元数据（原始文件名 / MIME / 宽高）与文章引用数在这里一次性补齐。
func (l *FileListLogic) FileList(req *types.FileListReq) (resp *types.FileListResp, err error) {
	root := l.svcCtx.Config.Upload.Defaults().Dir
	dir, err := utils.SafeResolve(root, req.Dir)
	if err != nil {
		return nil, common.NewBizError(common.ErrInvalidParams)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, common.NewBizError(common.ErrFileNotFound)
		}
		logx.Errorf("read dir %s failed: %v", dir, err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	kw := strings.ToLower(strings.TrimSpace(req.Keyword))
	var items []types.FileItem
	for _, e := range entries {
		name := e.Name()
		if kw != "" && !strings.Contains(strings.ToLower(name), kw) {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		rel, rerr := filepath.Rel(root, filepath.Join(dir, name))
		if rerr != nil {
			continue
		}
		relPath := strings.ReplaceAll(rel, string(os.PathSeparator), "/")
		items = append(items, types.FileItem{
			Name:     name,
			Path:     relPath,
			Url:      uploadURLPrefix + "/" + relPath,
			Size:     info.Size(),
			Modified: info.ModTime().Unix(),
			IsDir:    e.IsDir(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	l.attachMediaMeta(items)

	total := int64(len(items))
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &types.FileListResp{
		List:       items[start:end],
		Total:      total,
		CurrentDir: strings.ReplaceAll(strings.TrimPrefix(req.Dir, "/"), string(os.PathSeparator), "/"),
	}, nil
}

// attachMediaMeta 合并媒体表元数据与文章引用数；历史文件没有元数据行时留空，
// 不影响文件列表可用性。
func (l *FileListLogic) attachMediaMeta(items []types.FileItem) {
	urls := make([]string, 0, len(items))
	for _, it := range items {
		if !it.IsDir {
			urls = append(urls, it.Url)
		}
	}
	if len(urls) == 0 {
		return
	}
	metaByURL := make(map[string]*ent.MediaFile, len(urls))
	rows, err := l.svcCtx.EntClient.MediaFile.Query().
		Where(mediafile.URLIn(urls...)).
		All(l.ctx)
	if err != nil {
		logx.Errorf("读取媒体元数据失败: %v", err)
	} else {
		for _, m := range rows {
			metaByURL[m.URL] = m
		}
	}
	refs := mediaRefCounts(l.ctx, l.svcCtx.EntClient, urls)
	for i := range items {
		if items[i].IsDir {
			continue
		}
		if m, ok := metaByURL[items[i].Url]; ok {
			items[i].OriginalName = m.OriginalName
			items[i].Mime = m.Mime
			items[i].Width = m.Width
			items[i].Height = m.Height
		}
		items[i].RefCount = refs[items[i].Url]
	}
}
