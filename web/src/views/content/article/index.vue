<template>
  <div class="article-page">
    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="query.keyword"
          placeholder="标题 / 摘要"
          clearable
          style="width: 200px"
          @keyup.enter="onSearch"
          @clear="onSearch"
        />
        <el-select v-model="query.status" placeholder="状态" clearable style="width: 120px" @change="onSearch">
          <el-option label="草稿" :value="0" />
          <el-option label="已发布" :value="1" />
          <el-option label="已下线" :value="2" />
          <el-option label="待审核" :value="3" />
        </el-select>
        <el-select v-model="query.category_id" placeholder="分类" clearable style="width: 150px" @change="onSearch">
          <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>
        <el-select v-model="query.tag_id" placeholder="标签" clearable style="width: 140px" @change="onSearch">
          <el-option v-for="t in tags" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
        <el-select v-model="sortPreset" placeholder="排序" style="width: 150px" @change="onSortChange">
          <el-option label="置顶优先" value="default" />
          <el-option label="发布时间 ↓" value="published_at-desc" />
          <el-option label="发布时间 ↑" value="published_at-asc" />
          <el-option label="浏览量 ↓" value="view_count-desc" />
          <el-option label="更新时间 ↓" value="updated_at-desc" />
          <el-option label="创建时间 ↓" value="created_at-desc" />
        </el-select>
        <el-button @click="onSearch">搜索</el-button>
        <el-button v-if="canAdd" type="primary" @click="openCreate">写文章</el-button>
        <el-button v-if="canWithdraw" :disabled="selected.length === 0" @click="onBatchOffline">
          批量下线<span v-if="selected.length"> ({{ selected.length }})</span>
        </el-button>
        <el-button v-if="canDelete" :disabled="selected.length === 0" type="danger" plain @click="onBatchDelete">
          批量删除<span v-if="selected.length"> ({{ selected.length }})</span>
        </el-button>
        <el-button v-if="canTrash" @click="openTrash">回收站</el-button>
        <el-button v-if="canExport" :loading="exporting" @click="onExport">导出内容</el-button>
        <el-button v-if="canImport" :loading="importing" @click="pickImportFile">导入内容</el-button>
        <input
          ref="fileInput"
          type="file"
          accept="application/json,.json"
          class="file-input"
          @change="onImportFile"
        />
      </div>

      <el-table v-loading="loading" :data="list" stripe @selection-change="onSelectionChange">
        <el-table-column type="selection" width="46" />
        <el-table-column label="标题" min-width="240">
          <template #default="{ row }">
            <span class="article-title">{{ asRow(row).title }}</span>
            <el-tag v-if="asRow(row).is_top" type="warning" size="small" effect="plain">置顶</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="95">
          <template #default="{ row }">
            <el-tag :type="statusType(asRow(row))" size="small">
              {{ asRow(row).status_text }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="作者" width="110">
          <template #default="{ row }">{{ asRow(row).author_name || '—' }}</template>
        </el-table-column>
        <el-table-column prop="category_name" label="分类" min-width="110" />
        <el-table-column label="标签" min-width="150">
          <template #default="{ row }">
            <el-tag
              v-for="t in asRow(row).tag_names"
              :key="t"
              size="small"
              effect="plain"
              class="tag-chip"
            >
              {{ t }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="view_count" label="浏览" width="80" />
        <el-table-column label="发布时间" width="170">
          <template #default="{ row }">{{ asRow(row).published_at || '—' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="420" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canEdit" link type="primary" @click="openEdit(asRow(row))">编辑</el-button>
            <el-button
              v-if="canPublish && asRow(row).status !== 3"
              link
              type="success"
              @click="openPublish(asRow(row))"
            >
              发布
            </el-button>
            <el-button
              v-if="canSubmit && asRow(row).status === 0"
              link
              type="primary"
              @click="onSubmit(asRow(row))"
            >
              提交审核
            </el-button>
            <el-button
              v-if="canApprove && asRow(row).status === 3"
              link
              type="success"
              @click="openApprove(asRow(row))"
            >
              通过
            </el-button>
            <el-button
              v-if="canReject && asRow(row).status === 3"
              link
              type="warning"
              @click="onReject(asRow(row))"
            >
              驳回
            </el-button>
            <el-button
              v-if="canWithdraw && asRow(row).status === 1"
              link
              type="warning"
              @click="onWithdraw(asRow(row))"
            >
              撤回
            </el-button>
            <el-button v-if="canTop" link @click="onToggleTop(asRow(row))">
              {{ asRow(row).is_top ? '取消置顶' : '置顶' }}
            </el-button>
            <el-button v-if="canRevision" link @click="openRevisions(asRow(row))">修订</el-button>
            <el-button link @click="onPreview(asRow(row))">预览</el-button>
            <el-button v-if="canDelete" link type="danger" @click="onDelete(asRow(row))">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="暂无文章" :image-size="60" /></template>
      </el-table>

      <el-pagination
        class="pager"
        layout="total, sizes, prev, pager, next"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        :page-size="query.page_size"
        :current-page="query.page"
        @size-change="onSizeChange"
        @current-change="onPageChange"
      />
    </el-card>

    <el-dialog v-model="publishVisible" title="发布文章" width="440px">
      <p class="publish-tip">
        「{{ publishTarget?.title }}」
        <br />
        留空表示立即发布；选择未来时间则到点后自动对前台可见。
      </p>
      <el-date-picker
        v-model="publishAt"
        type="datetime"
        placeholder="发布时间（可留空）"
        value-format="YYYY-MM-DD HH:mm:ss"
        style="width: 100%"
      />
      <template #footer>
        <el-button @click="publishVisible = false">取消</el-button>
        <el-button type="primary" :loading="publishing" @click="doPublish">确定发布</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="approveVisible" title="审核通过" width="440px">
      <p class="publish-tip">
        「{{ reviewTarget?.title }}」审核通过后即可对前台可见。留空表示立即发布，也可指定定时发布时间。
      </p>
      <el-date-picker
        v-model="approveAt"
        type="datetime"
        placeholder="发布时间（可留空）"
        value-format="YYYY-MM-DD HH:mm:ss"
        style="width: 100%"
      />
      <template #footer>
        <el-button @click="approveVisible = false">取消</el-button>
        <el-button type="primary" :loading="reviewing" @click="doApprove">确定通过</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from '@/utils/element'
import { useUserStore } from '@/stores/user'
import {
  approveArticle,
  batchDeleteArticles,
  batchOfflineArticles,
  deleteArticle,
  getArticles,
  getCategories,
  getTags,
  publishArticle,
  rejectArticle,
  submitArticle,
  toggleArticleTop,
  withdrawArticle,
  type ArticleItem,
  type CategoryItem,
  type TagItem,
} from '@/api/content'
import { downloadContentBundle, importContentBundle } from '@/api/transfer'

type TagType = 'primary' | 'success' | 'info' | 'warning' | 'danger'

const router = useRouter()
const userStore = useUserStore()
const canExport = computed(() => userStore.hasPermission('cms:content:export'))
const canImport = computed(() => userStore.hasPermission('cms:content:import'))
const canAdd = computed(() => userStore.hasPermission('cms:article:add'))
const canEdit = computed(() => userStore.hasPermission('cms:article:edit'))
const canDelete = computed(() => userStore.hasPermission('cms:article:delete'))
const canPublish = computed(() => userStore.hasPermission('cms:article:publish'))
const canWithdraw = computed(() => userStore.hasPermission('cms:article:withdraw'))
const canTop = computed(() => userStore.hasPermission('cms:article:toggle_top'))
const canSubmit = computed(() => userStore.hasPermission('cms:article:submit'))
const canApprove = computed(() => userStore.hasPermission('cms:article:approve'))
const canReject = computed(() => userStore.hasPermission('cms:article:reject'))
const canRevision = computed(() => userStore.hasPermission('cms:article:revision'))
const canTrash = computed(() => userStore.hasPermission('cms:trash:list'))

const list = ref<ArticleItem[]>([])
const categories = ref<CategoryItem[]>([])
const tags = ref<TagItem[]>([])
const selected = ref<ArticleItem[]>([])
const sortPreset = ref('default')
const total = ref(0)
const loading = ref(false)
const query = reactive({
  page: 1,
  page_size: 10,
  keyword: '',
  status: undefined as number | undefined,
  category_id: undefined as number | undefined,
  tag_id: undefined as number | undefined,
  sort: undefined as string | undefined,
  order: undefined as 'asc' | 'desc' | undefined,
})

const publishVisible = ref(false)
const publishing = ref(false)
const publishTarget = ref<ArticleItem | null>(null)
const publishAt = ref('')

const approveVisible = ref(false)
const reviewing = ref(false)
const reviewTarget = ref<ArticleItem | null>(null)
const approveAt = ref('')

const asRow = (row: unknown) => row as ArticleItem
const statusType = (row: ArticleItem): TagType => {
  if (row.status_text === '定时中') return 'primary'
  if (row.status === 1) return 'success'
  if (row.status === 2) return 'info'
  if (row.status === 3) return 'danger'
  return 'warning'
}

async function loadList() {
  loading.value = true
  try {
    const res = await getArticles({
      page: query.page,
      page_size: query.page_size,
      keyword: query.keyword || undefined,
      status: query.status,
      category_id: query.category_id,
      tag_id: query.tag_id,
      sort: query.sort,
      order: query.order,
    })
    list.value = res.list ?? []
    total.value = res.total ?? 0
  } catch {
    list.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function loadCategories() {
  try {
    const res = await getCategories()
    categories.value = res.list ?? []
  } catch {
    categories.value = []
  }
}

async function loadTags() {
  try {
    const res = await getTags()
    tags.value = res.list ?? []
  } catch {
    tags.value = []
  }
}

function onSearch() {
  query.page = 1
  void loadList()
}

function onPageChange(p: number) {
  query.page = p
  void loadList()
}

function onSizeChange(size: number) {
  query.page_size = size
  query.page = 1
  void loadList()
}

function onSortChange(preset: string) {
  if (preset === 'default') {
    query.sort = undefined
    query.order = undefined
  } else {
    const [sort, order] = preset.split('-')
    query.sort = sort
    query.order = order === 'asc' ? 'asc' : 'desc'
  }
  query.page = 1
  void loadList()
}

function onSelectionChange(rows: ArticleItem[]) {
  selected.value = rows
}

function openCreate() {
  void router.push('/content/articles/new')
}

function openEdit(row: ArticleItem) {
  void router.push(`/content/articles/${row.id}/edit`)
}

function openRevisions(row: ArticleItem) {
  void router.push({ path: `/content/articles/${row.id}/edit`, query: { tab: 'revisions' } })
}

function openTrash() {
  void router.push('/content/trash')
}

function onPreview(row: ArticleItem) {
  if (row.status !== 1 || row.status_text === '定时中') {
    ElMessage.info('文章尚未对前台可见，请先发布或等待到达定时发布时间')
    return
  }
  window.open(`/articles/${row.slug}`, '_blank')
}

function openPublish(row: ArticleItem) {
  publishTarget.value = row
  publishAt.value = ''
  publishVisible.value = true
}

async function doPublish() {
  if (!publishTarget.value) return
  publishing.value = true
  try {
    await publishArticle(publishTarget.value.id, publishAt.value || undefined)
    ElMessage.success(publishAt.value ? '已设置定时发布' : '已发布')
    publishVisible.value = false
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  } finally {
    publishing.value = false
  }
}

function openApprove(row: ArticleItem) {
  reviewTarget.value = row
  approveAt.value = ''
  approveVisible.value = true
}

async function doApprove() {
  if (!reviewTarget.value) return
  reviewing.value = true
  try {
    await approveArticle(reviewTarget.value.id, approveAt.value || undefined)
    ElMessage.success(approveAt.value ? '已通过并设置定时发布' : '审核通过，已发布')
    approveVisible.value = false
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  } finally {
    reviewing.value = false
  }
}

async function onSubmit(row: ArticleItem) {
  try {
    await ElMessageBox.confirm(`把「${row.title}」提交审核？提交后将等待有审核权限的管理员处理。`, '提示', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await submitArticle(row.id)
    ElMessage.success('已提交审核')
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  }
}

async function onReject(row: ArticleItem) {
  let remark = ''
  try {
    const res = await ElMessageBox.prompt(`驳回「${row.title}」，文章将退回草稿。`, '驳回审核', {
      inputPlaceholder: '驳回原因（可选）',
      inputType: 'textarea',
      type: 'warning',
    })
    remark = res.value ?? ''
  } catch {
    return
  }
  try {
    await rejectArticle(row.id, remark)
    ElMessage.success('已驳回，文章退回草稿')
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  }
}

async function onWithdraw(row: ArticleItem) {
  try {
    await ElMessageBox.confirm(`确定撤回「${row.title}」吗？撤回后前台将不再展示。`, '提示', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await withdrawArticle(row.id)
    ElMessage.success('已撤回')
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  }
}

async function onToggleTop(row: ArticleItem) {
  try {
    await toggleArticleTop(row.id)
    ElMessage.success(row.is_top ? '已取消置顶' : '已置顶')
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  }
}

async function onDelete(row: ArticleItem) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.title}」吗？删除后可在回收站恢复。`, '提示', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteArticle(row.id)
    ElMessage.success('已移入回收站')
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  }
}

async function onBatchOffline() {
  const ids = selected.value.map((r) => r.id)
  if (ids.length === 0) return
  try {
    await ElMessageBox.confirm(
      `把选中的 ${ids.length} 篇文章中「已发布」的下线？其它状态保持不变。`,
      '批量下线',
      { type: 'warning' },
    )
  } catch {
    return
  }
  try {
    const res = await batchOfflineArticles(ids)
    ElMessage.success(res?.count ? `已下线 ${res.count} 篇` : '所选文章中没有已发布文章')
    selected.value = []
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  }
}

async function onBatchDelete() {
  const ids = selected.value.map((r) => r.id)
  if (ids.length === 0) return
  try {
    await ElMessageBox.confirm(
      `把选中的 ${ids.length} 篇文章移入回收站？可在回收站恢复。`,
      '批量删除',
      { type: 'warning' },
    )
  } catch {
    return
  }
  try {
    const res = await batchDeleteArticles(ids)
    ElMessage.success(`已移入回收站 ${res?.count ?? 0} 篇`)
    selected.value = []
    await loadList()
  } catch {
    // 统一拦截器已提示错误
  }
}

const exporting = ref(false)
const importing = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

async function onExport() {
  exporting.value = true
  try {
    await downloadContentBundle()
    ElMessage.success('内容包已导出')
  } catch (e) {
    ElMessage.error((e as Error).message || '导出失败')
  } finally {
    exporting.value = false
  }
}

function pickImportFile() {
  fileInput.value?.click()
}

async function onImportFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return

  let bundle: unknown
  try {
    bundle = JSON.parse(await file.text())
  } catch {
    ElMessage.error('文件不是合法的 JSON 内容包')
    return
  }

  let overwrite = false
  try {
    await ElMessageBox.confirm(
      '「覆盖并导入」会用文件中的文章覆盖同 slug 文章；「仅新增」只补齐缺失内容。',
      '导入内容包',
      {
        confirmButtonText: '覆盖并导入',
        cancelButtonText: '仅新增',
        distinguishCancelAndClose: true,
        type: 'warning',
      },
    )
    overwrite = true
  } catch (action) {
    if (action !== 'cancel') return
  }

  importing.value = true
  try {
    const res = await importContentBundle(bundle, overwrite)
    ElMessage.success(
      `导入完成：文章新建 ${res.articles_created} / 覆盖 ${res.articles_updated} / 跳过 ${res.articles_skipped}`,
    )
    await Promise.all([loadList(), loadCategories(), loadTags()])
  } catch (e) {
    ElMessage.error((e as Error).message || '导入失败')
  } finally {
    importing.value = false
  }
}

onMounted(() => {
  void loadList()
  void loadCategories()
  void loadTags()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}
.file-input {
  display: none;
}
.article-title {
  margin-right: 6px;
}
.tag-chip {
  margin-right: 4px;
}
.pager {
  margin-top: 14px;
  justify-content: flex-end;
}
.publish-tip {
  margin: 0 0 14px;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}
</style>
