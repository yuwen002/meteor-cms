package logic

import (
	"strings"
	"testing"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
)

func TestNormalizeSlug(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"英文标题", "Hello World", "hello-world"},
		{"大小写与下划线", "  Foo_Meteor  Bar ", "foo-meteor-bar"},
		{"压缩连续分隔符", "a---b   c", "a-b-c"},
		{"纯中文返回空", "文章发布", ""},
		{"中文混排保留英文", "Go 语言入门", "go"},
		{"去首尾连字符", "--abc--", "abc"},
		{"空输入", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeSlug(c.raw); got != c.want {
				t.Fatalf("NormalizeSlug(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

func TestNormalizeSlugTruncates(t *testing.T) {
	got := NormalizeSlug(strings.Repeat("a", 250))
	if len(got) != maxSlugLen {
		t.Fatalf("slug length = %d, want %d", len(got), maxSlugLen)
	}
}

func TestGenerateSlug(t *testing.T) {
	now := time.Unix(1700000000, 123456789)
	if got := GenerateSlug("My Custom Slug", "标题", now); got != "my-custom-slug" {
		t.Fatalf("user slug = %q", got)
	}
	if got := GenerateSlug("", "Hello World", now); got != "hello-world" {
		t.Fatalf("title derived slug = %q", got)
	}
	if got := GenerateSlug("", "纯中文标题", now); got != "post-1700000000123456789" {
		t.Fatalf("fallback slug = %q", got)
	}
}

func TestIsValidSlug(t *testing.T) {
	for _, s := range []string{"hello-world", "post-123", "a"} {
		if !IsValidSlug(s) {
			t.Errorf("expect valid slug: %q", s)
		}
	}
	for _, s := range []string{"", "Hello", "a_b", "-abc", "abc-", "中文", "a--b"} {
		if IsValidSlug(s) {
			t.Errorf("expect invalid slug: %q", s)
		}
	}
}

func TestIsPubliclyVisible(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	if IsPubliclyVisible(ArticleStatusPublished, &past, now) != true {
		t.Error("已发布且发布时间已到，应可见")
	}
	if IsPubliclyVisible(ArticleStatusPublished, &future, now) != false {
		t.Error("定时发布未到点，应不可见")
	}
	if IsPubliclyVisible(ArticleStatusPublished, nil, now) != false {
		t.Error("已发布但没有发布时间，应不可见")
	}
	if IsPubliclyVisible(ArticleStatusDraft, &past, now) != false {
		t.Error("草稿不应可见")
	}
	if IsPubliclyVisible(ArticleStatusOffline, &past, now) != false {
		t.Error("已下线不应可见")
	}
	if IsPubliclyVisible(ArticleStatusPending, &past, now) != false {
		t.Error("待审核不应可见")
	}
}

func TestParsePublishTime(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	if got, err := ParsePublishTime("", now); err != nil || !got.Equal(now) {
		t.Fatalf("empty should mean publish now, got %v err %v", got, err)
	}
	got, err := ParsePublishTime("2026-11-06 09:30:00", now)
	if err != nil || got.Year() != 2026 || got.Hour() != 9 || got.Minute() != 30 {
		t.Fatalf("parse datetime failed: %v err %v", got, err)
	}
	got, err = ParsePublishTime("2026-11-06T09:30", now)
	if err != nil || got.Hour() != 9 || got.Minute() != 30 {
		t.Fatalf("parse datetime-local failed: %v err %v", got, err)
	}
	if _, err = ParsePublishTime("2026/11/06", now); err == nil {
		t.Fatal("unsupported format should return error")
	}
}

func TestArticleStatus(t *testing.T) {
	valid := []int{ArticleStatusDraft, ArticleStatusPublished, ArticleStatusOffline, ArticleStatusPending}
	for _, s := range valid {
		if !IsValidArticleStatus(s) {
			t.Fatalf("status %d should be valid", s)
		}
	}
	if IsValidArticleStatus(-1) || IsValidArticleStatus(4) {
		t.Fatal("out of range status should be invalid")
	}
	cases := map[int]string{
		ArticleStatusDraft:     "草稿",
		ArticleStatusPublished: "已发布",
		ArticleStatusOffline:   "已下线",
		ArticleStatusPending:   "待审核",
		99:                     "未知",
	}
	for status, want := range cases {
		if got := ArticleStatusText(status); got != want {
			t.Errorf("ArticleStatusText(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestArticleDisplayStatus(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	if got := ArticleDisplayStatus(ArticleStatusPublished, &future, now); got != "定时中" {
		t.Errorf("future publish should display 定时中, got %q", got)
	}
	if got := ArticleDisplayStatus(ArticleStatusPublished, &past, now); got != "已发布" {
		t.Errorf("past publish should display 已发布, got %q", got)
	}
	if got := ArticleDisplayStatus(ArticleStatusPending, &future, now); got != "待审核" {
		t.Errorf("pending should not be shown as 定时中, got %q", got)
	}
}

func TestTrashSlug(t *testing.T) {
	now := time.Unix(1700000000, 123456789)
	s := TrashSlug(42, now)
	if !IsTrashSlug(s) || s == "" {
		t.Fatalf("unexpected trash slug %q", s)
	}
	if IsTrashSlug("normal-slug") {
		t.Fatal("normal slug should not be detected as trash slug")
	}
}

func TestExtractMarkdownImageURLs(t *testing.T) {
	md := "![a](/uploads/2026/11/a.png) ![a again](/uploads/2026/11/a.png) " +
		"![external](https://example.com/x.png) ![protocol](//evil.com/t.gif) " +
		"[link](/uploads/not-image.png)"
	got := ExtractMarkdownImageURLs(md)
	if len(got) != 1 || got[0] != "/uploads/2026/11/a.png" {
		t.Fatalf("ExtractMarkdownImageURLs = %v", got)
	}
	if len(ExtractMarkdownImageURLs("no images")) != 0 {
		t.Fatal("should be empty for content without images")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := FirstNonEmpty("", "  ", "hello", "world"); got != "hello" {
		t.Fatalf("FirstNonEmpty = %q", got)
	}
	if got := FirstNonEmpty("", ""); got != "" {
		t.Fatalf("FirstNonEmpty all empty = %q", got)
	}
}

func TestReviewActionText(t *testing.T) {
	cases := map[int]string{
		ReviewActionSubmit:  "提交审核",
		ReviewActionApprove: "审核通过",
		ReviewActionReject:  "驳回",
		99:                  "未知",
	}
	for action, want := range cases {
		if got := ReviewActionText(action); got != want {
			t.Errorf("ReviewActionText(%d) = %q, want %q", action, got, want)
		}
	}
}

func TestNormalizeBatchIDs(t *testing.T) {
	ids, err := normalizeBatchIDs([]int64{3, 0, -1, 3, 2})
	if err != nil {
		t.Fatalf("normalizeBatchIDs failed: %v", err)
	}
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 2 {
		t.Fatalf("应去重并过滤非正数，got %v", ids)
	}
	if _, err := normalizeBatchIDs(nil); err == nil {
		t.Error("空 ID 列表应报错")
	}
	tooMany := make([]int64, maxBatchArticles+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	if _, err := normalizeBatchIDs(tooMany); err == nil {
		t.Error("超过单次上限应报错")
	}
}

// Redis 降级到进程内去重表后，过期项必须能被回收，否则内存会随访客 IP 单调增长。
func TestViewDedupMemoryFallback(t *testing.T) {
	l := &PublicArticleLogic{svcCtx: &svc.ServiceContext{}}
	key := "test:view:dedup"
	defer viewDedupMem.Delete(key)
	l.releaseViewDedup(key)

	if !l.acquireViewDedup(key) {
		t.Fatal("首次访问应放行")
	}
	if l.acquireViewDedup(key) {
		t.Fatal("同一去重键在窗口内应被拦截")
	}
	l.releaseViewDedup(key)
	if !l.acquireViewDedup(key) {
		t.Fatal("落库失败释放占位后应可重新计数")
	}

	now := time.Now().Unix()
	viewDedupMem.Store("test:expired", now-1)
	viewDedupMem.Store("test:alive", now+100)
	viewDedupCleanAt.Store(0)
	cleanupViewDedupMem(now)
	if _, ok := viewDedupMem.Load("test:expired"); ok {
		t.Error("过期去重项未被清理")
	}
	if _, ok := viewDedupMem.Load("test:alive"); !ok {
		t.Error("未过期的去重项被误删")
	}
	viewDedupMem.Delete("test:alive")
}

// MySQL datetime(0) 会把「立即发布」的纳秒时间四舍五入到下一秒，
// 容差保证刚发布的文章不会被判成定时中 / 不可见。
func TestPublishTimeTolerance(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	justRounded := now.Add(500 * time.Millisecond)
	farFuture := now.Add(2 * time.Second)

	if got := ArticleDisplayStatus(ArticleStatusPublished, &justRounded, now); got != "已发布" {
		t.Errorf("秒级误差内应显示已发布，got %q", got)
	}
	if !IsPubliclyVisible(ArticleStatusPublished, &justRounded, now) {
		t.Error("秒级误差内应公开可见")
	}
	if got := ArticleDisplayStatus(ArticleStatusPublished, &farFuture, now); got != "定时中" {
		t.Errorf("超过容差的未来时间仍应显示定时中，got %q", got)
	}
	if IsPubliclyVisible(ArticleStatusPublished, &farFuture, now) {
		t.Error("超过容差的未来时间仍应不可见")
	}
}
