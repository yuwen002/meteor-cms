package logic

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/zeromicro/go-zero/core/logx"
)

// mediaFileURL 把相对上传根目录的路径转成站内访问 URL。
func mediaFileURL(rel string) string {
	p := strings.ReplaceAll(rel, `\`, "/")
	p = strings.TrimPrefix(filepath.ToSlash(p), "/")
	return uploadURLPrefix + "/" + p
}

// mediaRefCount 统计某个文件被多少篇文章的封面或正文引用。
func mediaRefCount(ctx context.Context, client *ent.Client, url string) (int, error) {
	return client.Article.Query().
		Where(article.Or(article.CoverEQ(url), article.ContentContains(url))).
		Count(ctx)
}

// mediaRefCounts 批量统计引用篇数：只查一次文章的封面/正文列，
// 避免文件列表每行各发一条 SQL。结果按 URL 去重计数（同一篇只算一次）。
func mediaRefCounts(ctx context.Context, client *ent.Client, urls []string) map[string]int {
	out := make(map[string]int, len(urls))
	if len(urls) == 0 {
		return out
	}
	rows, err := client.Article.Query().
		Select(article.FieldCover, article.FieldContent).
		All(ctx)
	if err != nil {
		logx.Errorf("统计媒体引用失败: %v", err)
		return out
	}
	for _, a := range rows {
		for _, u := range urls {
			if a.Cover == u || strings.Contains(a.Content, u) {
				out[u]++
			}
		}
	}
	return out
}
