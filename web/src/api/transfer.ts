import request from '@/utils/request'
import { getToken } from '@/utils/auth'

export interface ContentImportResult {
  categories_created: number
  categories_skipped: number
  tags_created: number
  tags_skipped: number
  articles_created: number
  articles_updated: number
  articles_skipped: number
  configs_created: number
  configs_skipped: number
  dicts_created: number
  dicts_skipped: number
  dict_items_created: number
  dict_items_skipped: number
}

/** 导出是文件下载，必须绕过统一 { code, msg, data } 响应拦截器。 */
export async function downloadContentBundle(): Promise<void> {
  const token = getToken()
  const res = await fetch('/admin/content/export', {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!res.ok) throw new Error(`导出失败 (${res.status})`)
  if (!(res.headers.get('Content-Type') || '').includes('application/json')) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.msg || '导出失败')
  }
  const url = URL.createObjectURL(await res.blob())
  const link = document.createElement('a')
  link.href = url
  link.download = `cms-content-${new Date().toISOString().slice(0, 10)}.json`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

/** 导入请求体即内容包 JSON 本体；overwrite=true 时覆盖同 slug 文章。 */
export function importContentBundle(bundle: unknown, overwrite = false) {
  return request({
    url: '/admin/content/import',
    method: 'post',
    data: bundle,
    params: { overwrite: overwrite ? 'true' : undefined },
  }) as Promise<ContentImportResult>
}
