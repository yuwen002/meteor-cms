package metrics

import (
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"

	"github.com/yuwen002/meteor-cms/internal/pkg/audit"
	"github.com/yuwen002/meteor-cms/internal/version"
)

// PrometheusContentType 是 Prometheus exposition format 0.0.4 的响应类型。
const PrometheusContentType = "text/plain; version=0.0.4; charset=utf-8"

// 指标名统一加前缀，避免与其它应用被同一个 job 抓取时互相覆盖。
const metricPrefix = "meteor_cms_"

// WritePrometheus 以 Prometheus 文本格式写出当前进程的瞬时指标快照。
// 不引入客户端依赖：counter / gauge 都直接来自已有的原子计数、runtime 与审计队列快照；
// 输出顺序固定，便于抓取端做增量计算，也便于测试断言。
func WritePrometheus(w io.Writer) error {
	total, serverErrors, inFlight, uptime := Snapshot()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	depth, dropped := audit.Stats()

	var b strings.Builder
	write := func(name, kind, help, value, labels string) {
		full := metricPrefix + name
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n%s%s %s\n",
			full, help, full, kind, full, labels, value)
	}

	write("requests_total", "counter", "进程启动以来处理的 HTTP 请求总数。",
		strconv.FormatInt(total, 10), "")
	write("requests_server_errors_total", "counter", "进程启动以来返回 5xx 的 HTTP 请求数。",
		strconv.FormatInt(serverErrors, 10), "")
	write("requests_in_flight", "gauge", "当前在途的 HTTP 请求数。",
		strconv.FormatInt(inFlight, 10), "")
	write("uptime_seconds", "gauge", "进程运行时长（秒）。",
		strconv.FormatFloat(uptime.Seconds(), 'f', 3, 64), "")
	write("go_goroutines", "gauge", "当前 goroutine 数量。",
		strconv.Itoa(runtime.NumGoroutine()), "")
	write("go_heap_alloc_bytes", "gauge", "Go 堆已分配字节数。",
		strconv.FormatUint(mem.HeapAlloc, 10), "")
	write("go_sys_bytes", "gauge", "Go 运行时向操作系统申请的总字节数。",
		strconv.FormatUint(mem.Sys, 10), "")
	write("go_gc_total", "counter", "已完成的 GC 次数。",
		strconv.FormatUint(uint64(mem.NumGC), 10), "")
	write("audit_queue_depth", "gauge", "审计写入队列当前积压任务数。",
		strconv.Itoa(depth), "")
	write("audit_queue_dropped_total", "counter", "审计写入队列累计丢弃任务数。",
		strconv.FormatInt(dropped, 10), "")

	labels := fmt.Sprintf("{version=%s,commit=%s,build_time=%s,schema_version=%s,go_version=%s}",
		quoteLabel(version.Version), quoteLabel(version.Commit),
		quoteLabel(version.BuildTime), quoteLabel(version.SchemaVersion),
		quoteLabel(runtime.Version()))
	write("build_info", "gauge", "构建与运行时版本信息，值恒为 1。", "1", labels)

	_, err := io.WriteString(w, b.String())
	return err
}

// quoteLabel 按 exposition format 转义 label value 中的反斜杠、双引号与换行。
func quoteLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}
