// Code scaffolded manually. Safe to edit.
// 运行监控 / 全文检索新增响应类型单独放这里，避免 goctl 重生成时被覆盖。

package types

// MonitorResp 是 /admin/monitor 的运行状态快照。
type MonitorResp struct {
	Status              string  `json:"status"`
	Database            string  `json:"database"`
	Redis               string  `json:"redis"`
	UptimeSeconds       int64   `json:"uptime_seconds"`
	GoVersion           string  `json:"go_version"`
	Goroutines          int     `json:"goroutines"`
	HeapAllocMB         float64 `json:"heap_alloc_mb"`
	SysMB               float64 `json:"sys_mb"`
	NumGC               uint32  `json:"num_gc"`
	RequestsTotal       int64   `json:"requests_total"`
	RequestsServerError int64   `json:"requests_server_error"`
	RequestsInFlight    int64   `json:"requests_in_flight"`
	AvgQPS              float64 `json:"avg_qps"`
	AuditQueueDepth     int     `json:"audit_queue_depth"`
	AuditQueueDropped   int64   `json:"audit_queue_dropped"`
}

// SearchResp 是全文检索结果；列表项与公开文章列表保持一致。
type SearchResp struct {
	Keyword  string              `json:"keyword"`
	TookMs   int64               `json:"took_ms"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	List     []PublicArticleItem `json:"list"`
}
