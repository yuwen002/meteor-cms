package contentbundle

import (
	"bytes"
	"strings"
	"testing"
)

func TestBundleValidate(t *testing.T) {
	valid := &Bundle{
		Version:    Version,
		Categories: []Category{{Name: "技术", Slug: "tech"}},
		Tags:       []Tag{{Name: "Go", Slug: "go"}},
		Articles: []Article{{
			Title: "第一篇文章", Slug: "hello", CategorySlug: "tech",
			TagSlugs: []string{"go"}, PublishedAt: "2026-01-02T03:04:05Z",
		}},
		Configs: []Config{{Key: "site_name", Name: "站点名称", Value: "测试"}},
		Dicts:   []Dict{{Code: "yes_no", Name: "是否", Items: []DictItem{{Label: "是", Value: "1"}}}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("合法内容包被拒绝: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Bundle)
	}{
		{"版本不匹配", func(b *Bundle) { b.Version = "9.9" }},
		{"文章缺标题", func(b *Bundle) { b.Articles[0].Title = "" }},
		{"文章 slug 重复", func(b *Bundle) {
			b.Articles = append(b.Articles, Article{Title: "重复", Slug: "hello"})
		}},
		{"分类 slug 重复", func(b *Bundle) {
			b.Categories = append(b.Categories, Category{Name: "重复", Slug: "tech"})
		}},
		{"时间格式非法", func(b *Bundle) { b.Articles[0].PublishedAt = "2026-01-02" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := *valid
			b.Articles = append([]Article(nil), valid.Articles...)
			b.Categories = append([]Category(nil), valid.Categories...)
			c.mutate(&b)
			// 时间格式由 Import 的 parseTime 校验，Validate 只检查结构
			if c.name == "时间格式非法" {
				if _, err := parseTime(b.Articles[0].PublishedAt); err == nil {
					t.Fatal("非法时间应被拒绝")
				}
				return
			}
			if err := b.Validate(); err == nil {
				t.Fatal("非法内容包应被拒绝")
			}
		})
	}
}

func TestBundleWriteDecodeRoundTrip(t *testing.T) {
	in := &Bundle{
		Version: Version, ExportedAt: "2026-01-02T03:04:05Z",
		Categories: []Category{{Name: "技术", Slug: "tech", IsActive: true}},
		Articles:   []Article{{Title: "标题", Slug: "hello", Content: "# 正文"}},
	}
	var buf bytes.Buffer
	if err := Write(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != Version || len(out.Articles) != 1 || out.Articles[0].Content != "# 正文" {
		t.Fatalf("往返结果不一致: %+v", out)
	}
}

func TestDecodeRejectsInvalidJSONAndVersion(t *testing.T) {
	if _, err := Decode(strings.NewReader(`{"version":`)); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	if _, err := Decode(strings.NewReader(`{"version":"0.1"}`)); err == nil {
		t.Fatal("旧版本应被拒绝")
	}
	if _, err := Decode(strings.NewReader(`{"version":"1.0","articles":[{"title":"x","slug":""}]}`)); err == nil {
		t.Fatal("空 slug 应被拒绝")
	}
}
