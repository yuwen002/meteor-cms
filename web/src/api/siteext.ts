import request from '@/utils/request'

export interface LinkItem {
  id: number
  name: string
  url: string
  logo: string
  description: string
  link_type: string
  sort: number
  status: number
  created_at: string
}

export interface LinkWrite {
  name: string
  url: string
  logo?: string
  description?: string
  link_type?: string
  sort?: number
  status?: number
}

export interface CommentItem {
  id: number
  article_id: number
  parent_id: number
  author: string
  email: string
  content: string
  status: number
  ip: string
  created_at: string
}

export interface PublicCommentItem {
  id: number
  parent_id: number
  author: string
  content: string
  created_at: string
}

export interface SubscriberItem {
  id: number
  email: string
  status: number
  source: string
  created_at: string
}

export function getAdminLinks(params: { keyword?: string; status?: number } = {}) {
  return request({ url: '/admin/links', method: 'get', params }) as Promise<{
    list: LinkItem[]
    total: number
  }>
}

export function createLink(data: LinkWrite) {
  return request({ url: '/admin/links', method: 'post', data })
}

export function updateLink(id: number, data: LinkWrite) {
  return request({ url: `/admin/links/${id}`, method: 'put', data })
}

export function deleteLink(id: number) {
  return request({ url: `/admin/links/${id}`, method: 'delete' })
}

export function getAdminComments(params: { keyword?: string; status?: number } = {}) {
  return request({ url: '/admin/comments', method: 'get', params }) as Promise<{
    list: CommentItem[]
    total: number
  }>
}

export function updateCommentStatus(id: number, status: number) {
  return request({ url: `/admin/comments/${id}/status`, method: 'put', data: { status } })
}

export function deleteComment(id: number) {
  return request({ url: `/admin/comments/${id}`, method: 'delete' })
}

export function getAdminSubscribers(params: { keyword?: string } = {}) {
  return request({ url: '/admin/subscribers', method: 'get', params }) as Promise<{
    list: SubscriberItem[]
    total: number
  }>
}

export function deleteSubscriber(id: number) {
  return request({ url: `/admin/subscribers/${id}`, method: 'delete' })
}

export function getPublicLinks() {
  return request({ url: '/api/links', method: 'get' }) as Promise<{ list: LinkItem[] }>
}

export function getPublicComments(slug: string) {
  return request({ url: `/api/comments/${encodeURIComponent(slug)}`, method: 'get' }) as Promise<{
    list: PublicCommentItem[]
  }>
}

export function createPublicComment(data: {
  slug: string
  author: string
  email?: string
  content: string
  parent_id?: number
}) {
  return request({ url: '/api/comments', method: 'post', data }) as Promise<{
    id: number
    status: number
    need_review: boolean
  }>
}

export function subscribe(email: string) {
  return request({ url: '/api/subscribe', method: 'post', data: { email, source: 'portal' } })
}
