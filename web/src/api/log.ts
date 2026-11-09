import request from '@/utils/request'
import { getToken } from '@/utils/auth'

/** 日志列表通用查询参数 */
export interface LogPageQuery {
  page: number
  page_size: number
  username?: string
  /** 0 成功 / 1 失败；不传表示不筛选 */
  status?: number
  start_time?: string
  end_time?: string
}

export interface OperationLogQuery extends LogPageQuery {
  /** 仅超管生效，非超管会被后端忽略 */
  user_id?: number
  module?: string
  path?: string
}

export interface LoginLogQuery extends LogPageQuery {
  /** 仅超管生效，非超管会被后端忽略 */
  user_id?: number
}

export interface OperationLogItem {
  id: number
  user_id: number
  username: string
  module: string
  action: string
  method: string
  path: string
  ip: string
  user_agent: string
  status: number
  error_msg: string
  duration_ms: number
  params: string
  /** URL query / 表单参数快照（已脱敏） */
  query: string
  /** 请求链路标识，可与运行日志串联 */
  request_id: string
  /** 业务响应码，0 表示成功 */
  biz_code: number
  /** HTTP 响应状态码 */
  http_status: number
  created_at: string
}

export interface LoginLogItem {
  id: number
  user_id: number
  username: string
  ip: string
  user_agent: string
  status: number
  fail_reason: string
  request_id: string
  login_at: string
}

export interface LogListResp<T> {
  total: number
  page: number
  page_size: number
  list: T[]
}

export function getOperationLogs(params: OperationLogQuery) {
  return request<LogListResp<OperationLogItem>>({
    url: '/admin/logs/operations',
    method: 'get',
    params,
  })
}

export function getLoginLogs(params: LoginLogQuery) {
  return request<LogListResp<LoginLogItem>>({
    url: '/admin/logs/logins',
    method: 'get',
    params,
  })
}

export function getOperationLogDetail(id: number) {
  return request<OperationLogItem>({ url: `/admin/logs/operations/${id}`, method: 'get' })
}

export function getLoginLogDetail(id: number) {
  return request<LoginLogItem>({ url: `/admin/logs/logins/${id}`, method: 'get' })
}

export interface LogDayStat {
  date: string
  total: number
  failed: number
}

export interface LogModuleStat {
  module: string
  total: number
}

export interface LogBizCodeStat {
  biz_code: number
  total: number
}

export interface LogStatsResp {
  days: number
  total: number
  success: number
  failed: number
  by_day: LogDayStat[]
  by_module: LogModuleStat[]
  by_biz_code: LogBizCodeStat[]
}

export function getLogStats(days = 7) {
  return request<LogStatsResp>({ url: '/admin/logs/stats', method: 'get', params: { days } })
}

export interface LogExportQuery {
  type?: 'operation' | 'login'
  status?: number
  start_time?: string
  end_time?: string
}

/** CSV 导出绕过 axios 拦截器（拦截器假定所有响应都是 JSON），自行携带 token 下载 */
export async function downloadLogExport(params: LogExportQuery) {
  const qs = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') qs.set(key, String(value))
  })
  const token = getToken()
  const res = await fetch(`/admin/logs/export?${qs.toString()}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!res.ok) throw new Error(`导出失败 (${res.status})`)
  if (!(res.headers.get('Content-Type') || '').includes('text/csv')) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.msg || '导出失败')
  }
  const url = URL.createObjectURL(await res.blob())
  const link = document.createElement('a')
  link.href = url
  link.download = `${params.type || 'operation'}-logs.csv`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
