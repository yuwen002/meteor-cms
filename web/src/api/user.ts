import request from '@/utils/request'

export interface MyMenuItem {
  id: number
  name: string
  parent_id: number
  path: string
  component: string
  icon: string
  permission: string
  sort: number
  children: MyMenuItem[]
}

export interface MyPermissionResp {
  is_super: boolean
  menus: MyMenuItem[]
  permissions: string[]
}

export function getProfile() {
  return request({ url: '/admin/profile', method: 'get' })
}

export function updateProfile(data: {
  nickname?: string
  email?: string
  phone?: string
  avatar?: string
}) {
  return request({ url: '/admin/profile', method: 'put', data })
}

export function getAdminList(params: {
  page?: number
  page_size?: number
  keyword?: string
  active?: number
  dept_id?: number
}) {
  return request({ url: '/admin/admin-users', method: 'get', params })
}

export function getAdminDetail(id: number) {
  return request({ url: `/admin/admin-users/${id}`, method: 'get' }) as Promise<{ dept_id: number }>
}

// 后端要求部门必须存在，因此不提供「清空部门」选项
export function bindAdminDepartment(id: number, deptId: number) {
  return request({
    url: `/admin/admin-users/${id}/bind-department`,
    method: 'put',
    data: { dept_id: deptId },
  })
}

export function createAdmin(data: Record<string, unknown>) {
  return request({ url: '/admin/admin-users/create', method: 'post', data })
}

export function updateAdmin(id: number, data: Record<string, unknown>) {
  return request({ url: `/admin/admin-users/${id}`, method: 'put', data })
}

export function deleteAdmin(id: number) {
  return request({ url: `/admin/admin-users/${id}`, method: 'delete' })
}

export function disableAdmin(id: number) {
  return request({ url: `/admin/admin-users/${id}/disable`, method: 'put' })
}

export function enableAdmin(id: number) {
  return request({ url: `/admin/admin-users/${id}/enable`, method: 'put' })
}

export function resetAdminPassword(id: number, newPassword: string) {
  return request({
    url: '/admin/admin-users/reset-password',
    method: 'put',
    data: { id, new_password: newPassword },
  })
}

export function getAdminRoles(id: number) {
  return request({ url: `/admin/admin-users/${id}/roles`, method: 'get' })
}

export function assignAdminRoles(id: number, roleIds: number[]) {
  return request({
    url: `/admin/admin-users/${id}/roles`,
    method: 'put',
    data: { role_ids: roleIds },
  })
}

// 登录后拉取「当前用户可见的菜单树 + 可用权限标识」，用于动态菜单与按钮级权限控制
export function getMyPermissions() {
  return request({ url: '/admin/profile/permissions', method: 'get' }) as Promise<MyPermissionResp>
}
