//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

type operationLogRow struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Module    string `json:"module"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	CreatedAt string `json:"created_at"`
}

func parseOperationLogs(t *testing.T, r apiResp) (int64, []operationLogRow) {
	t.Helper()
	var d struct {
		Total int64             `json:"total"`
		List  []operationLogRow `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		t.Fatalf("parse operation logs: %v (%s)", err, r.Data)
	}
	return d.Total, d.List
}

func listOperationLogs(t *testing.T, token, query string) (int64, []operationLogRow) {
	t.Helper()
	r := call(t, http.MethodGet, "/admin/logs/operations?page=1&page_size=100"+query, token, nil)
	wantCode(t, "list operation logs "+query, 0, r)
	return parseOperationLogs(t, r)
}

func hasStatus(rows []operationLogRow, status int) bool {
	for _, row := range rows {
		if row.Status == status {
			return true
		}
	}
	return false
}

// TestOperationLogQueryAndAudit 覆盖日志查询的三个历史缺陷：
// status=0 被当成「未筛选」、日期型 end_time 漏掉当天、日志查询自身放大日志量，
// 同时验证 action 字段已写入、日志查询接口仍然放行。
func TestOperationLogQueryAndAudit(t *testing.T) {
	waitHealthy(t)
	admin := login(t, "admin", envOr("CMS_E2E_ADMIN_PASSWORD", "123456"))
	// 让其它 E2E 用例产生的异步日志先落盘，便于用最大 ID 隔离本次新增记录。
	time.Sleep(time.Second)

	_, before := listOperationLogs(t, admin, "")
	var maxID int64
	if len(before) > 0 {
		maxID = before[0].ID
	}

	// 制造一条成功日志与一条失败日志
	wantCode(t, "probe success", 0,
		call(t, http.MethodGet, "/admin/roles?page=1&page_size=1", admin, nil))
	fail := call(t, http.MethodGet, "/admin/admin-users/999999999", admin, nil)
	if fail.Code == 0 {
		t.Fatalf("探针请求应返回业务错误，实际 code=0 data=%s", fail.Data)
	}

	var fresh []operationLogRow
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		_, rows := listOperationLogs(t, admin, "")
		fresh = make([]operationLogRow, 0, 2)
		for _, row := range rows {
			if row.ID > maxID {
				fresh = append(fresh, row)
			}
		}
		if hasStatus(fresh, 0) && hasStatus(fresh, 1) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !hasStatus(fresh, 0) || !hasStatus(fresh, 1) {
		t.Fatalf("未等到新增的成功/失败操作日志，新增记录: %+v", fresh)
	}

	// action 不再恒空
	withAction := false
	date := ""
	for _, row := range fresh {
		if row.Action != "" {
			withAction = true
		}
		if date == "" && len(row.CreatedAt) >= 10 {
			date = row.CreatedAt[:10]
		}
	}
	if !withAction {
		t.Errorf("新增操作日志的 action 不应为空: %+v", fresh)
	}

	// status 为三态：不传=不筛选，0=只看成功，1=只看失败
	totalAll, _ := listOperationLogs(t, admin, "")
	totalOK, rowsOK := listOperationLogs(t, admin, "&status=0")
	totalFail, rowsFail := listOperationLogs(t, admin, "&status=1")
	if totalOK == 0 || totalFail == 0 {
		t.Errorf("成功/失败日志都应能筛出，totalOK=%d totalFail=%d", totalOK, totalFail)
	}
	for _, row := range rowsOK {
		if row.Status != 0 {
			t.Errorf("status=0 却返回了 status=%d 的记录（旧版会把 0 当成未筛选）", row.Status)
			break
		}
	}
	for _, row := range rowsFail {
		if row.Status != 1 {
			t.Errorf("status=1 却返回了 status=%d 的记录", row.Status)
			break
		}
	}
	if totalAll < totalOK || totalAll < totalFail {
		t.Errorf("未筛选总数应不小于任一状态筛选结果: all=%d ok=%d fail=%d", totalAll, totalOK, totalFail)
	}

	// 纯日期型 end_time 必须包含当天
	if date == "" {
		t.Fatal("新增日志缺少 created_at，无法验证日期边界")
	}
	totalByDate, _ := listOperationLogs(t, admin, "&end_time="+date)
	if totalByDate == 0 {
		t.Errorf("end_time=%s 应包含当天日志（旧版会按当天 00:00 截断），实际 0 条", date)
	}

	// 日志查询不得自放大：连续查询若干次后总数不变
	stableBefore, _ := listOperationLogs(t, admin, "")
	for i := 0; i < 3; i++ {
		listOperationLogs(t, admin, "&status=0")
	}
	stableAfter, _ := listOperationLogs(t, admin, "")
	if stableAfter != stableBefore {
		t.Errorf("日志查询不应产生新操作日志: before=%d after=%d", stableBefore, stableAfter)
	}

	// 登录日志共用同一套筛选语义
	r := call(t, http.MethodGet, "/admin/logs/logins?page=1&page_size=100&status=0", admin, nil)
	wantCode(t, "list login logs status=0", 0, r)
	var d struct {
		List []struct {
			Status int `json:"status"`
		} `json:"list"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		t.Fatalf("parse login logs: %v (%s)", err, r.Data)
	}
	for _, row := range d.List {
		if row.Status != 0 {
			t.Errorf("登录日志 status=0 却返回了 status=%d 的记录", row.Status)
			break
		}
	}
}
