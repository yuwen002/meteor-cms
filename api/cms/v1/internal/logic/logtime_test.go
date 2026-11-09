package logic

import (
	"testing"
	"time"
)

// 回归：日期型结束时间必须包含当天，否则 end_time=YYYY-MM-DD 会把当天日志全部排除。
func TestParseLogEndTime(t *testing.T) {
	dateOnly, ok := parseLogTime("2026-11-08")
	if !ok {
		t.Fatal("日期型起始时间解析失败")
	}
	want := dateOnly.AddDate(0, 0, 1).Add(-time.Nanosecond)

	got, ok := parseLogEndTime("2026-11-08")
	if !ok {
		t.Fatal("日期型结束时间解析失败")
	}
	if !got.Equal(want) {
		t.Fatalf("日期型结束时间应为当天 23:59:59.999999999，实际 %s", got.Format(time.RFC3339Nano))
	}
	if got.Day() != 8 {
		t.Fatalf("结束时间不应跨到次日，实际 %s", got.Format(time.RFC3339Nano))
	}
}

func TestParseLogEndTimeKeepsPreciseTime(t *testing.T) {
	cases := []string{
		"2026-11-08 10:30:00",
		"2026-11-08T10:30:00+08:00",
	}
	for _, input := range cases {
		want, ok := parseLogTime(input)
		if !ok {
			t.Fatalf("起始时间解析失败: %s", input)
		}
		got, ok := parseLogEndTime(input)
		if !ok {
			t.Fatalf("结束时间解析失败: %s", input)
		}
		if !got.Equal(want) {
			t.Fatalf("带时分秒的结束时间不应被扩展，输入 %s，期望 %s，实际 %s",
				input, want.Format(time.RFC3339Nano), got.Format(time.RFC3339Nano))
		}
	}
}

func TestParseLogEndTimeInvalid(t *testing.T) {
	for _, input := range []string{"", "  ", "not-a-time"} {
		if _, ok := parseLogEndTime(input); ok {
			t.Fatalf("非法输入 %q 不应解析成功", input)
		}
	}
}
