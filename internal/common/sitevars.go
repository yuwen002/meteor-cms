package common

import (
	"strconv"
	"strings"
	"time"
)

// ExpandSiteText 展开站点文本模板中的动态占位符。
//
// 支持 {year}（当前年份）、{site_name} / {site}（站点名称）。
// 只做一次替换、不递归展开：运营填写的内容里若含占位符字面量，不会被二次解释，
// 也不会因站名本身包含占位符而产生替换循环。
// 未知占位符原样保留，便于后台直接显示与编辑原始模板。
func ExpandSiteText(text string, now time.Time, siteName string) string {
	if text == "" || !strings.Contains(text, "{") {
		return text
	}
	return strings.NewReplacer(
		"{year}", strconv.Itoa(now.Year()),
		"{site_name}", siteName,
		"{site}", siteName,
	).Replace(text)
}
