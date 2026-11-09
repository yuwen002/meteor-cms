import request from '@/utils/request'

export interface DictItem {
  id: number
  code: string
  name: string
  description: string
  status: number
  sort: number
  dict_group: string
  is_system: boolean
  created_at: string
}

export interface DictListResp {
  list: DictItem[]
  total: number
}

export interface DictDataItem {
  id: number
  dict_code: string
  label: string
  value: string
  status: number
  sort: number
  parent_id: number
  is_default: boolean
  remark: string
}

export function getDicts(params: {
  keyword?: string
  status?: number
  group?: string
  page?: number
  page_size?: number
}) {
  return request({ url: '/admin/dicts', method: 'get', params }) as Promise<DictListResp>
}

export function createDict(data: {
  code: string
  name: string
  description?: string
  status?: number
  sort?: number
  dict_group?: string
}) {
  return request({ url: '/admin/dicts', method: 'post', data })
}

export function updateDict(
  id: number,
  data: {
    name?: string
    description?: string
    status?: number
    sort?: number
    dict_group?: string
  },
) {
  return request({ url: `/admin/dicts/${id}`, method: 'put', data })
}

export function deleteDict(id: number) {
  return request({ url: `/admin/dicts/${id}`, method: 'delete' })
}

export function getDictItems(params: { dict_code: string; keyword?: string; status?: number }) {
  return request({
    url: '/admin/dicts/items',
    method: 'get',
    params,
  }) as Promise<{ list: DictDataItem[] }>
}

export function createDictItem(data: {
  dict_code: string
  label: string
  value: string
  status?: number
  sort?: number
  parent_id?: number
  is_default?: boolean
  remark?: string
}) {
  return request({ url: '/admin/dicts/items', method: 'post', data })
}

export function updateDictItem(
  id: number,
  data: {
    label?: string
    value?: string
    status?: number
    sort?: number
    parent_id?: number
    is_default?: boolean
    remark?: string
  },
) {
  return request({ url: `/admin/dicts/items/${id}`, method: 'put', data })
}

export function deleteDictItem(id: number) {
  return request({ url: `/admin/dicts/items/${id}`, method: 'delete' })
}
