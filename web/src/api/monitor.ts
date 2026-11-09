import request from '@/utils/request'

export interface MonitorInfo {
  status: string
  database: string
  redis: string
  uptime_seconds: number
  go_version: string
  goroutines: number
  heap_alloc_mb: number
  sys_mb: number
  num_gc: number
  requests_total: number
  requests_server_error: number
  requests_in_flight: number
  avg_qps: number
  audit_queue_depth: number
  audit_queue_dropped: number
}

export function getMonitor() {
  return request({ url: '/admin/monitor', method: 'get' }) as Promise<MonitorInfo>
}
