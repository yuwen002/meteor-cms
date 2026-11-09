<template>
  <div class="portal" :style="{ '--site-primary': site.primary_color || '#409EFF' }">
    <header class="site-header">
      <div class="header-inner">
        <router-link to="/articles" class="brand">
          <img v-if="site.logo" :src="site.logo" :alt="site.name" class="brand-logo" />
          <template v-else>{{ site.name }}</template>
        </router-link>
        <nav class="nav-links">
          <router-link to="/articles">全部文章</router-link>
          <a href="/api/rss" target="_blank" rel="noopener noreferrer">RSS</a>
          <router-link to="/login">管理后台</router-link>
        </nav>
      </div>
    </header>

    <main class="container">
      <el-breadcrumb class="crumb" separator="/">
        <el-breadcrumb-item><router-link to="/articles">文章</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>{{ article?.title ?? '详情' }}</el-breadcrumb-item>
      </el-breadcrumb>

      <div v-loading="loading" class="detail-card">
        <template v-if="article">
          <h1 class="detail-title">{{ article.title }}</h1>
          <div class="detail-meta">
            <span v-if="article.category_name" class="chip">{{ article.category_name }}</span>
            <span v-if="article.author_name">{{ article.author_name }}</span>
            <span>{{ formatDate(article.published_at) }}</span>
            <span>{{ article.view_count }} 次阅读</span>
          </div>
          <img v-if="article.cover" :src="article.cover" class="detail-cover" :alt="article.title" />
          <!-- renderMarkdown 先转义 HTML 再套用有限语法，链接协议已做白名单过滤 -->
          <div class="article-body" v-html="rendered" />
          <div v-if="article.tag_names.length" class="detail-tags">
            <span v-for="t in article.tag_names" :key="t" class="tag"># {{ t }}</span>
          </div>

          <nav v-if="article.prev || article.next" class="adjacent-nav">
            <a v-if="article.prev" class="adjacent-link" @click="openArticle(article.prev)">
              <span>← 上一篇</span>
              <strong>{{ article.prev.title }}</strong>
            </a>
            <span v-else />
            <a v-if="article.next" class="adjacent-link right" @click="openArticle(article.next)">
              <span>下一篇 →</span>
              <strong>{{ article.next.title }}</strong>
            </a>
          </nav>
        </template>
        <el-empty v-else-if="!loading" description="文章不存在或尚未发布" :image-size="90" />
      </div>

      <section v-if="relatedList.length" class="related">
        <h2 class="related-title">相关文章</h2>
        <div class="related-grid">
          <a v-for="r in relatedList" :key="r.id" class="related-card" @click="openArticle(r)">
            <img v-if="r.cover" :src="r.cover" :alt="r.title" class="related-cover" />
            <div class="related-body">
              <strong>{{ r.title }}</strong>
              <p>{{ r.summary || '暂无摘要' }}</p>
              <span>{{ formatDate(r.published_at) }}</span>
            </div>
          </a>
        </div>
      </section>

      <div class="back-row">
        <el-button @click="goBack">← 返回文章列表</el-button>
      </div>
    </main>

    <section v-if="site.comment_enabled" class="comment-panel">
      <div class="container">
        <h3>评论<span v-if="comments.length">（{{ comments.length }}）</span></h3>
        <div v-for="c in comments" :key="c.id" class="comment-item">
          <div class="comment-head">
            <strong>{{ c.author }}</strong><span>{{ c.created_at }}</span>
          </div>
          <p>{{ c.content }}</p>
        </div>
        <el-empty v-if="!comments.length" description="还没有评论，来抢沙发" :image-size="60" />
        <div class="comment-form">
          <div class="comment-fields">
            <el-input v-model="commentForm.author" placeholder="昵称" style="width: 180px" />
            <el-input v-model="commentForm.email" placeholder="邮箱（选填）" style="width: 220px" />
          </div>
          <el-input v-model="commentForm.content" type="textarea" :rows="3" placeholder="说点什么…" />
          <el-button type="primary" :loading="commentSubmitting" @click="submitComment">发表评论</el-button>
        </div>
      </div>
    </section>

    <footer class="site-footer">
      <div>{{ site.footer || 'Powered by Meteor CMS' }}</div>
      <div v-if="site.icp" class="site-icp">{{ site.icp }}</div>
      <div class="footer-links">
        <a href="/api/rss" target="_blank" rel="noopener noreferrer">RSS</a> ·
        <a href="/sitemap.xml" target="_blank" rel="noopener noreferrer">Sitemap</a>
      </div>
      <div v-if="siteLinks.length" class="friend-links">
        友情链接：
        <a v-for="l in siteLinks" :key="l.id" :href="l.url" target="_blank" rel="noopener noreferrer">
          {{ l.name }}
        </a>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from '@/utils/element'
import { getPublicArticle, type PublicArticle, type PublicArticleDetail } from '@/api/public'
import { createPublicComment, getPublicComments, type PublicCommentItem } from '@/api/siteext'
import { plainSummary, renderMarkdown } from '@/utils/markdown'
import {
  applySiteMeta,
  articleTitle,
  loadSiteInfo,
  loadSiteLinks,
  siteInfo,
  siteLinks as siteLinksRef,
} from '@/utils/site'

const route = useRoute()
const router = useRouter()
const site = computed(() => siteInfo.value)
const siteLinks = computed(() => siteLinksRef.value)
const slug = computed(() => String(route.params.slug ?? ''))
const article = ref<PublicArticleDetail | null>(null)
const loading = ref(false)
const rendered = computed(() => renderMarkdown(article.value?.content ?? ''))
const relatedList = computed<PublicArticle[]>(() => article.value?.related ?? [])
const originalTitle = document.title

const comments = ref<PublicCommentItem[]>([])
const commentForm = reactive({ author: '', email: '', content: '' })
const commentSubmitting = ref(false)

async function loadComments() {
  if (!slug.value) return
  try {
    comments.value = (await getPublicComments(slug.value)).list || []
  } catch {
    comments.value = []
  }
}

async function submitComment() {
  if (!commentForm.author.trim() || !commentForm.content.trim()) {
    ElMessage.warning('请填写昵称与评论内容')
    return
  }
  commentSubmitting.value = true
  try {
    const res = await createPublicComment({
      slug: slug.value,
      author: commentForm.author.trim(),
      email: commentForm.email.trim(),
      content: commentForm.content.trim(),
    })
    ElMessage.success(res.need_review ? '评论已提交，等待审核' : '评论已发布')
    commentForm.content = ''
    await loadComments()
  } catch (e) {
    ElMessage.error((e as Error).message || '提交失败')
  } finally {
    commentSubmitting.value = false
  }
}

interface MetaSnapshot {
  name: string
  content: string | null
  el: HTMLMetaElement | null
}
const metaSnapshots: MetaSnapshot[] = []
let canonicalSnapshot: { content: string | null; el: HTMLLinkElement | null } | null = null

// 服务端 SPAHandler 已注入首屏 SEO 标签；这里只负责 SPA 内切换文章时同步更新。
function syncMeta(name: string, content: string) {
  let el = document.head.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!metaSnapshots.some((m) => m.name === name)) {
    metaSnapshots.push({ name, content: el?.getAttribute('content') ?? null, el })
  }
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute('name', name)
    document.head.appendChild(el)
  }
  el.setAttribute('content', content)
}

function syncCanonical(url: string) {
  let el = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!canonicalSnapshot) {
    canonicalSnapshot = { content: el?.getAttribute('href') ?? null, el }
  }
  if (!el) {
    el = document.createElement('link')
    el.setAttribute('rel', 'canonical')
    document.head.appendChild(el)
  }
  el.setAttribute('href', url)
}

function syncPageMeta(a: PublicArticleDetail) {
  document.title = articleTitle(a.seo_title || a.title)
  syncMeta(
    'description',
    a.seo_description || a.summary || plainSummary(a.content) || site.value.description,
  )
  syncMeta('keywords', a.seo_keywords || a.tag_names.join(', ') || site.value.keywords)
  syncCanonical(a.canonical_url || `${window.location.origin}/articles/${a.slug}`)
}

function restoreMeta() {
  for (const snap of metaSnapshots) {
    if (!snap.content) {
      snap.el?.remove()
    } else {
      snap.el?.setAttribute('content', snap.content)
    }
  }
  if (canonicalSnapshot) {
    if (!canonicalSnapshot.content) {
      canonicalSnapshot.el?.remove()
    } else {
      canonicalSnapshot.el?.setAttribute('href', canonicalSnapshot.content)
    }
  }
}

function formatDate(s: string): string {
  return s ? s.replace('T', ' ').slice(0, 16) : ''
}

function openArticle(a: PublicArticle) {
  void router.push(`/articles/${a.slug}`)
}

async function load() {
  if (!slug.value) return
  loading.value = true
  try {
    // 站点信息必须先就绪，标题模板与默认 meta 才能正确套用
    const [a] = await Promise.all([getPublicArticle(slug.value), loadSiteInfo()])
    article.value = a
    syncPageMeta(a)
    void loadComments()
    window.scrollTo({ top: 0 })
  } catch {
    article.value = null
  } finally {
    loading.value = false
  }
}

function goBack() {
  void router.push('/articles')
}

onMounted(() => {
  void loadSiteInfo()
  void loadSiteLinks()
  void load()
})
watch(slug, () => {
  void load()
})
onUnmounted(() => {
  restoreMeta()
  applySiteMeta(site.value)
  if (!site.value.name) document.title = originalTitle
})
</script>

<style scoped>
.portal {
  min-height: 100vh;
  background: #f6f8fb;
  color: #1f2937;
}
.site-header {
  background: linear-gradient(120deg, #1d4ed8, #0ea5e9);
  color: #fff;
}
.header-inner {
  max-width: 860px;
  margin: 0 auto;
  padding: 18px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.brand {
  font-size: 21px;
  font-weight: 700;
  color: #fff;
  text-decoration: none;
}
.brand span {
  opacity: 0.75;
  margin-left: 4px;
  font-weight: 400;
}
.nav-links {
  display: flex;
  gap: 22px;
}
.nav-links a {
  color: rgba(255, 255, 255, 0.88);
  text-decoration: none;
  font-size: 14px;
}
.container {
  max-width: 860px;
  margin: 0 auto;
  padding: 0 24px 48px;
}
.crumb {
  margin: 26px 0 18px;
}
.crumb a {
  color: #2563eb;
  text-decoration: none;
}
.detail-card {
  background: #fff;
  border-radius: 10px;
  padding: 36px 40px 40px;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06);
  min-height: 260px;
}
.detail-title {
  margin: 0 0 14px;
  font-size: 30px;
  line-height: 1.4;
  color: #0f172a;
}
.detail-meta {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 13px;
  color: #94a3b8;
  padding-bottom: 22px;
  border-bottom: 1px solid #f1f5f9;
}
.chip {
  color: #2563eb;
  background: #eff6ff;
  padding: 2px 10px;
  border-radius: 10px;
}
.detail-cover {
  width: 100%;
  max-height: 360px;
  object-fit: cover;
  border-radius: 8px;
  margin: 24px 0;
}
.detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 32px;
  padding-top: 20px;
  border-top: 1px solid #f1f5f9;
}
.tag {
  font-size: 13px;
  color: #2563eb;
  background: #eff6ff;
  padding: 3px 10px;
  border-radius: 12px;
}
.adjacent-nav {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-top: 28px;
}
.adjacent-link {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 46%;
  padding: 12px 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  cursor: pointer;
  color: #2563eb;
  font-size: 12px;
  text-decoration: none;
}
.adjacent-link:hover {
  background: #eff6ff;
}
.adjacent-link.right {
  text-align: right;
}
.adjacent-link strong {
  font-size: 14px;
  font-weight: 600;
  color: #334155;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.related {
  margin-top: 32px;
}
.related-title {
  margin: 0 0 14px;
  font-size: 18px;
  color: #0f172a;
}
.related-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 14px;
}
.related-card {
  display: flex;
  flex-direction: column;
  background: #fff;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06);
  cursor: pointer;
  text-decoration: none;
  transition: transform 0.18s ease;
}
.related-card:hover {
  transform: translateY(-2px);
}
.related-cover {
  width: 100%;
  height: 110px;
  object-fit: cover;
}
.related-body {
  padding: 12px 14px 14px;
}
.related-body strong {
  font-size: 14px;
  color: #0f172a;
}
.related-body p {
  margin: 6px 0 8px;
  font-size: 12px;
  line-height: 1.6;
  color: #64748b;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.related-body span {
  font-size: 11px;
  color: #94a3b8;
}
.back-row {
  margin-top: 22px;
}
.site-footer {
  text-align: center;
  padding: 28px;
  color: #94a3b8;
  font-size: 12px;
}
.site-footer a {
  color: #64748b;
  text-decoration: none;
}

/* v-html 注入的节点不带 scoped 属性，必须用 :deep 命中 */
.article-body {
  margin-top: 24px;
  line-height: 1.85;
  font-size: 15px;
  word-break: break-word;
}
.article-body :deep(h1),
.article-body :deep(h2),
.article-body :deep(h3) {
  margin: 28px 0 12px;
  font-weight: 600;
  line-height: 1.4;
  color: #0f172a;
}
.article-body :deep(h1) { font-size: 26px; }
.article-body :deep(h2) { font-size: 22px; }
.article-body :deep(h3) { font-size: 18px; }
.article-body :deep(p) { margin: 12px 0; }
.article-body :deep(ul),
.article-body :deep(ol) { padding-left: 26px; margin: 12px 0; }
.article-body :deep(blockquote) {
  margin: 16px 0;
  padding: 8px 16px;
  border-left: 4px solid #60a5fa;
  color: #64748b;
  background: #f8fafc;
}
.article-body :deep(pre) {
  padding: 14px 16px;
  overflow: auto;
  border-radius: 6px;
  background: #0f172a;
  color: #e2e8f0;
}
.article-body :deep(code) {
  padding: 2px 5px;
  border-radius: 3px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  background: #f1f5f9;
}
.article-body :deep(pre code) { padding: 0; background: transparent; }
.article-body :deep(img) { max-width: 100%; border-radius: 6px; }
.article-body :deep(a) { color: #2563eb; text-decoration: none; }
.article-body :deep(a:hover) { text-decoration: underline; }
.article-body :deep(hr) { margin: 24px 0; border: 0; border-top: 1px solid #e2e8f0; }
.article-body :deep(del) { color: #94a3b8; }
.article-body :deep(table) {
  width: 100%;
  margin: 16px 0;
  border-collapse: collapse;
  font-size: 14px;
}
.article-body :deep(th),
.article-body :deep(td) {
  border: 1px solid #e2e8f0;
  padding: 8px 12px;
  text-align: left;
}
.article-body :deep(th) { background: #f8fafc; font-weight: 600; }
.article-body :deep(.task-list) { list-style: none; padding-left: 4px; }
.article-body :deep(.task-item) { margin: 4px 0; }
.article-body :deep(.task-item input) { margin-right: 6px; }

@media (max-width: 640px) {
  .detail-card { padding: 24px 20px; }
  .detail-title { font-size: 24px; }
}

/* 站点配置：Logo、主题主色与页脚文本 */
.brand-logo {
  height: 26px;
  max-width: 160px;
  object-fit: contain;
  vertical-align: middle;
}
.site-header {
  background: linear-gradient(120deg, var(--site-primary, #1d4ed8), #0ea5e9);
}
.crumb a,
.chip,
.tag,
.adjacent-link,
.article-body :deep(a) {
  color: var(--site-primary, #2563eb);
}
.site-icp {
  margin-top: 6px;
}
.footer-links {
  margin-top: 6px;
}
.friend-links {
  margin-top: 8px;
}
.friend-links a {
  margin: 0 6px;
}
.comment-panel {
  padding: 8px 0 28px;
}
.comment-panel h3 {
  margin: 0 0 16px;
  font-size: 18px;
}
.comment-item {
  padding: 12px 0;
  border-bottom: 1px solid #e2e8f0;
}
.comment-head {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #94a3b8;
}
.comment-item p {
  margin: 8px 0 0;
  color: #334155;
  line-height: 1.7;
  white-space: pre-wrap;
}
.comment-form {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  margin-top: 18px;
}
.comment-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
