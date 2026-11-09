import { ref } from 'vue'
import { getSiteInfo, type SiteInfo } from '@/api/public'
import { getPublicLinks, type LinkItem } from '@/api/siteext'

// 站点信息由 /api/site 白名单输出，接口不可用时退回内置默认值，前台绝不因此白屏。
const fallback: SiteInfo = {
  name: 'Meteor CMS',
  logo: '',
  description: '',
  keywords: '',
  icp: '',
  footer: '',
  language: 'zh-CN',
  title_template: '{title} - {site_name}',
  theme: 'light',
  primary_color: '#409EFF',
  comment_enabled: false,
}

export const siteInfo = ref<SiteInfo>({ ...fallback })
export const siteLinks = ref<LinkItem[]>([])

let pending: Promise<SiteInfo> | null = null
let linksPending: Promise<LinkItem[]> | null = null

function setMeta(name: string, content: string): void {
  let el = document.head.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!content) {
    el?.remove()
    return
  }
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute('name', name)
    document.head.appendChild(el)
  }
  el.setAttribute('content', content)
}

/** 把站点级元信息同步到 document（非文章页使用）。 */
export function applySiteMeta(site: SiteInfo = siteInfo.value): void {
  document.title = site.name || fallback.name
  setMeta('description', site.description)
  setMeta('keywords', site.keywords)
  if (site.language) {
    document.documentElement.lang = site.language
  }
}

/** 站点信息全局只拉一次；失败时清空 pending 以便下个页面重试。 */
export function loadSiteInfo(): Promise<SiteInfo> {
  if (!pending) {
    pending = getSiteInfo()
      .then((data) => {
        siteInfo.value = { ...fallback, ...data }
        applySiteMeta(siteInfo.value)
        return siteInfo.value
      })
      .catch(() => {
        pending = null
        return siteInfo.value
      })
  }
  return pending
}

/** 公开友链全局只拉一次；接口不可用或开关关闭时页脚自动隐藏该区域。 */
export function loadSiteLinks(): Promise<LinkItem[]> {
  if (!linksPending) {
    linksPending = getPublicLinks()
      .then((resp) => {
        siteLinks.value = resp.list || []
        return siteLinks.value
      })
      .catch(() => {
        linksPending = null
        siteLinks.value = []
        return siteLinks.value
      })
  }
  return linksPending
}

/** 文章标题模板，支持 {title} / {site_name} / {site} 占位符。 */
export function articleTitle(title: string): string {
  const site = siteInfo.value
  const tpl = site.title_template || fallback.title_template
  if (!tpl.includes('{title}')) {
    return title
  }
  return tpl
    .replace(/\{title\}/g, title)
    .replace(/\{site_name\}/g, site.name)
    .replace(/\{site\}/g, site.name)
}
