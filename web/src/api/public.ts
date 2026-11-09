import request from '@/utils/request'

export interface PublicArticle {
  id: number
  title: string
  slug: string
  summary: string
  cover: string
  author_name: string
  category_id: number
  category_name: string
  category_slug: string
  tag_names: string[]
  is_top: boolean
  view_count: number
  published_at: string
}

export interface PublicArticleListResp {
  list: PublicArticle[]
  total: number
  page: number
  page_size: number
}

export interface PublicArticleDetail extends PublicArticle {
  content: string
  seo_title: string
  seo_description: string
  seo_keywords: string
  canonical_url: string
  related: PublicArticle[]
  prev: PublicArticle | null
  next: PublicArticle | null
}

export interface PublicCategory {
  id: number
  name: string
  slug: string
  parent_id: number
  article_count: number
}

export interface PublicTag {
  id: number
  name: string
  slug: string
  article_count: number
}

export interface PublicTaxonomyResp {
  categories: PublicCategory[]
  tags: PublicTag[]
}

export interface PublicArticleParams {
  page?: number
  page_size?: number
  keyword?: string
  category?: string
  tag?: string
}

export function getPublicArticles(params: PublicArticleParams) {
  return request({ url: '/api/articles', method: 'get', params }) as Promise<PublicArticleListResp>
}

export function getPublicArticle(slug: string) {
  return request({
    url: `/api/articles/${encodeURIComponent(slug)}`,
    method: 'get',
  }) as Promise<PublicArticleDetail>
}

export function getPublicTaxonomy() {
  return request({ url: '/api/taxonomy', method: 'get' }) as Promise<PublicTaxonomyResp>
}

export interface SiteInfo {
  name: string
  logo: string
  description: string
  keywords: string
  icp: string
  footer: string
  language: string
  title_template: string
  theme: string
  primary_color: string
  comment_enabled: boolean
}

export interface PublicDictItem {
  label: string
  value: string
  is_default: boolean
  parent_id: number
  sort: number
}

/** 站点级只读信息，字段白名单输出，不含任何密钥配置 */
export function getSiteInfo() {
  return request({ url: '/api/site', method: 'get' }) as Promise<SiteInfo>
}

export interface SearchResp extends PublicArticleListResp {
  keyword: string
  took_ms: number
}

/** 全文检索：标题 / 摘要 / 正文一起匹配，仅返回已发布文章 */
export function getPublicSearch(params: PublicArticleParams & { keyword: string }) {
  return request({ url: '/api/search', method: 'get', params }) as Promise<SearchResp>
}

/** 公开字典：只返回启用项，按 sort 排序 */
export function getPublicDict(code: string) {
  return request({ url: `/api/dicts/${encodeURIComponent(code)}`, method: 'get' }) as Promise<{
    code: string
    items: PublicDictItem[]
  }>
}
