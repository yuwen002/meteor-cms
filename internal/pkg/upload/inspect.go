package upload

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Meta 是从已落盘文件探测出的媒体元数据。
type Meta struct {
	Mime   string
	Width  int
	Height int
}

// Inspect 读取文件头探测真实 MIME，并尝试解码图片宽高。
// 非图片或不支持解码的格式（如 WebP）宽高为 0，不影响上传主流程。
func Inspect(path string) Meta {
	f, err := os.Open(path)
	if err != nil {
		return Meta{}
	}
	defer f.Close()

	head := make([]byte, 512)
	n, _ := f.Read(head)
	mimeType := http.DetectContentType(head[:n])
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = strings.TrimSpace(mimeType[:i])
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Meta{Mime: mimeType}
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return Meta{Mime: mimeType}
	}
	return Meta{Mime: mimeType, Width: cfg.Width, Height: cfg.Height}
}

// LocalPath 把 /uploads/... 形式的站内 URL 还原为磁盘路径。
func LocalPath(root, url string) (string, error) {
	const prefix = "/uploads/"
	if !strings.HasPrefix(url, prefix) {
		return "", fmt.Errorf("非法的上传路径: %s", url)
	}
	rel := strings.TrimPrefix(url, prefix)
	if rel == "" || strings.Contains(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("非法的上传路径: %s", url)
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}
