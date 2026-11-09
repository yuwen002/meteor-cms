package upload

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPath(t *testing.T) {
	root := t.TempDir()
	p, err := LocalPath(root, "/uploads/2026/01/a.png")
	if err != nil {
		t.Fatalf("合法 URL 解析失败: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "/2026/01/a.png") {
		t.Fatalf("路径还原错误: %s", p)
	}
	for _, bad := range []string{
		"/other/a.png",
		"/uploads/../etc/passwd",
		"/uploads/",
	} {
		if _, err := LocalPath(root, bad); err == nil {
			t.Fatalf("非法 URL 未被拒绝: %s", bad)
		}
	}
}

func TestInspectPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	m := Inspect(path)
	if m.Mime != "image/png" || m.Width != 3 || m.Height != 2 {
		t.Fatalf("PNG 元数据解析错误: %+v", m)
	}
}

func TestInspectPlainTextAndMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := Inspect(path)
	if !strings.HasPrefix(m.Mime, "text/plain") || m.Width != 0 || m.Height != 0 {
		t.Fatalf("文本元数据解析错误: %+v", m)
	}
	if m := Inspect(filepath.Join(t.TempDir(), "missing")); m.Mime != "" || m.Width != 0 {
		t.Fatalf("不存在文件应返回空元数据: %+v", m)
	}
}
