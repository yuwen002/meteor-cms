// Code scaffolded manually. Safe to edit.
// 日志模块新增接口的类型单独放这里，避免重跑 goctl gen-api 时被 types.go 覆盖。

package types

type LogIDReq struct {
	ID int64 `path:"id"`
}

type LogExportReq struct {
	Type      string `form:"type,default=operation"`
	Status    *int   `form:"status,optional"`
	StartTime string `form:"start_time,optional"`
	EndTime   string `form:"end_time,optional"`
}

type LogStatsReq struct {
	Days int `form:"days,default=7"`
}

type LogDayStat struct {
	Date   string `json:"date"`
	Total  int64  `json:"total"`
	Failed int64  `json:"failed"`
}

type LogModuleStat struct {
	Module string `json:"module"`
	Total  int64  `json:"total"`
}

type LogBizCodeStat struct {
	BizCode int   `json:"biz_code"`
	Total   int64 `json:"total"`
}

type LogStatsResp struct {
	Days      int              `json:"days"`
	Total     int64            `json:"total"`
	Success   int64            `json:"success"`
	Failed    int64            `json:"failed"`
	ByDay     []LogDayStat     `json:"by_day"`
	ByModule  []LogModuleStat  `json:"by_module"`
	ByBizCode []LogBizCodeStat `json:"by_biz_code"`
}
