import request from '@/utils/request'

export interface ConfigItem {
  id: number
  config_key: string
  config_value: string
  name: string
  remark: string
  is_system: boolean
  config_group: string
  value_type: string
  validation: string
  sort: number
  status: number
  is_secret: boolean
  created_at: string
}

export interface ConfigVersionItem {
  id: number
  config_id: number
  config_key: string
  old_value: string
  new_value: string
  operator_id: number
  operator_name: string
  created_at: string
}

export interface ConfigWritePayload {
  config_key?: string
  config_value?: string
  name?: string
  remark?: string
  config_group?: string
  value_type?: string
  validation?: string
  sort?: number
  status?: number
  is_secret?: boolean
}

export function getConfigs(params: { keyword?: string; group?: string } = {}) {
  return request({ url: '/admin/configs', method: 'get', params }) as Promise<{
    list: ConfigItem[]
  }>
}

export function getConfigByKey(key: string) {
  return request({ url: `/admin/configs/key/${encodeURIComponent(key)}`, method: 'get' }) as Promise<ConfigItem>
}

export function getConfigDetail(id: number) {
  return request({ url: `/admin/configs/${id}`, method: 'get' }) as Promise<ConfigItem>
}

export function createConfig(data: ConfigWritePayload) {
  return request({ url: '/admin/configs', method: 'post', data })
}

export function updateConfig(id: number, data: ConfigWritePayload) {
  return request({ url: `/admin/configs/${id}`, method: 'put', data })
}

/** 一次事务保存多条配置值；密文值传 ****** 表示不修改。 */
export function batchUpdateConfigs(items: { id: number; config_value: string }[]) {
  return request({ url: '/admin/configs/batch', method: 'post', data: { items } })
}

export function getConfigVersions(id: number) {
  return request({ url: `/admin/config-versions/${id}`, method: 'get' }) as Promise<{
    list: ConfigVersionItem[]
  }>
}

export function rollbackConfig(id: number, versionId: number) {
  return request({
    url: `/admin/config-versions/${id}/rollback`,
    method: 'post',
    data: { version_id: versionId },
  })
}

export function deleteConfig(id: number) {
  return request({ url: `/admin/configs/${id}`, method: 'delete' })
}
