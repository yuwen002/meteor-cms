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
      <section class="hero">
        <h1>文章</h1>
        <p>{{ site.description || '记录、分享与沉淀' }}</p>
        <div class="search-bar">
          <el-input
            v-model="keyword"
            placeholder="搜索文章标题或摘要…"
            clearable
            size="large"
            @keyup.enter="onSearch"
            @clear="onSearch"
          />
          <el-button type="primary" size="large" @click="onSearch">搜索</el-button>
        </div>
      </section>

      <div class="layout">
        <div class="main-col">
          <div v-loading="loading" class="article-grid">
            <article v-for="a in list" :key="a.id" class="card" @click="openDetail(a)">
              <img v-if="a.cover" :src="a.cover" class="card-cover" :alt="a.title" />
              <div class="card-body">
                <div class="card-meta">
                  <el-tag v-if="a.is_top" size="small" type="warning" effect="dark">置顶</el-tag>
                  <span v-if="a.category_name" class="chip">{{ a.category_name }}</span>
                  <span class="date">{{ formatDate(a.published_at) }}</span>
                </div>
                <h2 class="card-title">{{ a.title }}</h2>
                <p class="summary">{{ a.summary || '暂无摘要' }}</p>
                <div class="card-footer">
                  <span v-for="t in a.tag_names" :key="t" class="tag"># {{ t }}</span>
                  <span v-if="a.author_name" class="author">{{ a.author_name }}</span>
                  <span class="views">{{ a.view_count }} 次阅读</span>
                </div>
              </div>
            </article>
          </div>
          <el-empty
            v-if="!loading && list.length === 0"
            description="还没有已发布的文章"
            :image-size="90"
          />
          <el-pagination
            v-if="total > query.page_size"
            class="pager"
            layout="total, prev, pager, next"
            :total="total"
            :page-size="query.page_size"
            :current-page="query.page"
            @current-change="onPageChange"
          />
        </div>

        <aside class="side-col">
          <section class="side-card">
            <h3>分类</h3>
            <a class="side-link" :class="{ active: query.category === '' }" @click="pickCategory('')">
              全部文章
            </a>
            <a
              v-for="c in categoryTree"
              :key="c.id"
              class="side-link"
              :class="{ active: query.category === c.slug }"
              :style="{ paddingLeft: `${10 + c.depth * 14}px` }"
              @click="pickCategory(c.slug)"
            >
              <span><span v-if="c.depth" class="tree-prefix">└ </span>{{ c.name }}</span>
              <span>{{ c.article_count }}</span>
            </a>
            <p v-if="taxonomy.categories.length === 0" class="side-empty">暂无分类</p>
            <p class="side-hint">选择父级分类会同时包含其子分类下的文章</p>
          </section>

          <section class="side-card">
            <h3>标签</h3>
            <div class="tag-cloud">
              <span
                v-for="t in taxonomy.tags"
                :key="t.id"
                class="cloud-tag"
                :class="{ active: query.tag === t.slug }"
                @click="pickTag(t.slug)"
              >
                # {{ t.name }}
              </span>
              <p v-if="taxonomy.tags.length === 0" class="side-empty">暂无标签</p>
            </div>
          </section>
        </aside>
      </div>
    </main>

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
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  getPublicArticles,
  getPublicSearch,
  getPublicTaxonomy,
  type PublicArticle,
  type PublicCategory,
  type PublicTaxonomyResp,
} from '@/api/public'
import { loadSiteInfo, loadSiteLinks, siteInfo, siteLinks as siteLinksRef } from '@/utils/site'

type CategoryNode = PublicCategory & { depth: number }

const router = useRouter()
const site = computed(() => siteInfo.value)
const siteLinks = computed(() => siteLinksRef.value)
const list = ref<PublicArticle[]>([])
const total = ref(0)
const loading = ref(false)
const keyword = ref('')
const query = reactive({ page: 1, page_size: 9, category: '', tag: '' })
const taxonomy = ref<PublicTaxonomyResp>({ categories: [], tags: [] })

// 公开分类接口返回的是带 parent_id 的扁平列表，这里按父子关系排成树序，
// 父级筛选会由后端自动包含所有子孙分类的文章。
const categoryTree = computed<CategoryNode[]>(() => {
  const cats = taxonomy.value.categories
  const byParent = new Map<number, PublicCategory[]>()
  for (const c of cats) {
    const parent = c.parent_id || 0
    const group = byParent.get(parent) ?? []
    group.push(c)
    byParent.set(parent, group)
  }
  const out: CategoryNode[] = []
  const seen = new Set<number>()
  const visit = (parent: number, depth: number) => {
    for (const c of byParent.get(parent) ?? []) {
      if (seen.has(c.id)) continue
      seen.add(c.id)
      out.push({ ...c, depth })
      visit(c.id, Math.min(depth + 1, 5))
    }
  }
  visit(0, 0)
  for (const c of cats) {
    if (!seen.has(c.id)) {
      seen.add(c.id)
      out.push({ ...c, depth: 0 })
    }
  }
  return out
})

function formatDate(s: string): string {
  return s ? s.slice(0, 10) : ''
}

async function load() {
  loading.value = true
  try {
    const params = {
      page: query.page,
      page_size: query.page_size,
      category: query.category || undefined,
      tag: query.tag || undefined,
    }
    // 有关键词时走全文检索（标题/摘要/正文），无关键词时走普通列表
    const kw = keyword.value.trim()
    const res = kw
      ? await getPublicSearch({ ...params, keyword: kw })
      : await getPublicArticles(params)
    list.value = res.list ?? []
    total.value = res.total ?? 0
  } catch {
    list.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function loadTaxonomy() {
  try {
    taxonomy.value = await getPublicTaxonomy()
  } catch {
    taxonomy.value = { categories: [], tags: [] }
  }
}

function onSearch() {
  query.page = 1
  void load()
}

function onPageChange(p: number) {
  query.page = p
  void load()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function pickCategory(slug: string) {
  query.category = slug
  query.page = 1
  void load()
}

function pickTag(slug: string) {
  query.tag = query.tag === slug ? '' : slug
  query.page = 1
  void load()
}

function openDetail(a: PublicArticle) {
  void router.push(`/articles/${a.slug}`)
}

onMounted(() => {
  void loadSiteInfo()
  void loadSiteLinks()
  void load()
  void loadTaxonomy()
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
  max-width: 1120px;
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
  letter-spacing: 0.5px;
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
.nav-links a:hover {
  color: #fff;
}
.container {
  max-width: 1120px;
  margin: 0 auto;
  padding: 0 24px 48px;
}
.hero {
  padding: 44px 0 30px;
}
.hero h1 {
  margin: 0;
  font-size: 34px;
  letter-spacing: -0.5px;
}
.hero p {
  margin: 8px 0 24px;
  color: #64748b;
}
.search-bar {
  display: flex;
  gap: 10px;
  max-width: 520px;
}
.layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 240px;
  gap: 24px;
  align-items: start;
}
.article-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 18px;
  min-height: 120px;
}
.card {
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06);
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
  display: flex;
  flex-direction: column;
}
.card:hover {
  transform: translateY(-3px);
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.1);
}
.card-cover {
  width: 100%;
  height: 160px;
  object-fit: cover;
}
.card-body {
  padding: 16px 18px 18px;
  display: flex;
  flex-direction: column;
  flex: 1;
}
.card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}
.chip {
  font-size: 12px;
  color: #2563eb;
  background: #eff6ff;
  padding: 2px 8px;
  border-radius: 10px;
}
.date {
  font-size: 12px;
  color: #94a3b8;
}
.card-title {
  margin: 0 0 8px;
  font-size: 17px;
  line-height: 1.45;
  color: #0f172a;
}
.summary {
  margin: 0 0 16px;
  font-size: 13px;
  line-height: 1.7;
  color: #64748b;
  flex: 1;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.card-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  border-top: 1px solid #f1f5f9;
  padding-top: 12px;
}
.tag {
  font-size: 12px;
  color: #64748b;
}
.author {
  font-size: 12px;
  color: #475569;
}
.views {
  margin-left: auto;
  font-size: 12px;
  color: #94a3b8;
}
.pager {
  margin-top: 24px;
  justify-content: center;
}
.side-col {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.side-card {
  background: #fff;
  border-radius: 10px;
  padding: 18px;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06);
}
.side-card h3 {
  margin: 0 0 14px;
  font-size: 15px;
  color: #0f172a;
}
.side-link {
  display: flex;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: 6px;
  font-size: 13px;
  color: #475569;
  cursor: pointer;
}
.side-link:hover,
.side-link.active {
  background: #eff6ff;
  color: #2563eb;
}
.side-link span {
  color: #94a3b8;
}
.side-link > span:first-child {
  color: inherit;
}
.tree-prefix {
  opacity: 0.5;
}
.side-empty {
  font-size: 12px;
  color: #94a3b8;
  margin: 6px 0 0;
}
.side-hint {
  margin: 12px 0 0;
  font-size: 11px;
  line-height: 1.6;
  color: #94a3b8;
}
.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.cloud-tag {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 12px;
  background: #f1f5f9;
  color: #475569;
  cursor: pointer;
}
.cloud-tag:hover,
.cloud-tag.active {
  background: #2563eb;
  color: #fff;
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
@media (max-width: 860px) {
  .layout {
    grid-template-columns: 1fr;
  }
  .side-col {
    order: -1;
  }
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
.chip,
.side-link:hover,
.side-link.active {
  color: var(--site-primary, #2563eb);
}
.cloud-tag:hover,
.cloud-tag.active {
  background: var(--site-primary, #2563eb);
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
</style>
