package metrics

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestWritePrometheusFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePrometheus(&buf); err != nil {
		t.Fatalf("写出指标失败: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"# HELP meteor_cms_requests_total ",
		"# TYPE meteor_cms_requests_total counter",
		"# TYPE meteor_cms_requests_in_flight gauge",
		"# TYPE meteor_cms_go_gc_total counter",
		"meteor_cms_build_info{version=",
		"meteor_cms_audit_queue_dropped_total ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("指标输出缺少 %q", want)
		}
	}

	// 每个非注释行都应是「指标名[可选 label] 数值」，数值必须是合法数字。
	valueRe := regexp.MustCompile(`^meteor_cms_[a-z_]+\{?[^ ]* [0-9.eE+-]+$`)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !valueRe.MatchString(line) {
			t.Errorf("指标行格式异常: %s", line)
		}
	}

	if got := strings.Count(out, "# HELP "); got != 11 {
		t.Errorf("HELP 行数 = %d，期望 11", got)
	}
	if got := strings.Count(out, "# TYPE "); got != 11 {
		t.Errorf("TYPE 行数 = %d，期望 11", got)
	}
}

func TestQuoteLabelEscapesSpecials(t *testing.T) {
	got := quoteLabel("a\"b\\c\nd")
	want := `"a\"b\\c\nd"`
	if got != want {
		t.Fatalf("label 转义结果 = %s，期望 %s", got, want)
	}
}
