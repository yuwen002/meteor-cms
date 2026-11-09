<template>
  <div class="file-manage">
    <el-card shadow="never">
      <div class="toolbar">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item>
            <a @click="navigate('')">根目录</a>
          </el-breadcrumb-item>
          <el-breadcrumb-item v-for="seg in pathSegments" :key="seg.path">
            <a @click="navigate(seg.path)">{{ seg.name }}</a>
          </el-breadcrumb-item>
        </el-breadcrumb>

        <div class="actions">
          <el-input
            v-model="keyword"
            placeholder="按文件名搜索"
            clearable
            style="width: 220px"
            @keyup.enter="onSearch"
            @clear="onSearch"
          />
          <el-button type="primary" @click="onSearch">搜索</el-button>
          <el-button :disabled="!currentDir" @click="goUp">返回上级</el-button>
          <el-button @click="load">刷新</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="list" row-key="path" stripe>
        <el-table-column label="名称" min-width="260">
          <template #default="{ row }">
            <span class="name-cell">
              <span class="icon">{{ row.is_dir ? '📁' : '📄' }}</span>
              <a v-if="row.is_dir" class="link" @click="openDir(asRow(row))">{{ row.name }}</a>
              <span v-else>{{ row.name }}</span>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="80">
          <template #default="{ row }">{{ row.is_dir ? '目录' : '文件' }}</template>
        </el-table-column>
        <el-table-column label="原始文件名" min-width="130" show-overflow-tooltip>
          <template #default="{ row }">{{ row.is_dir ? '-' : row.original_name || '-' }}</template>
        </el-table-column>
        <el-table-column label="MIME / 尺寸" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <template v-if="row.is_dir">-</template>
            <template v-else>
              {{ row.mime || '未知' }}<span v-if="row.width"> · {{ row.width }}×{{ row.height }}</span>
            </template>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="110">
          <template #default="{ row }">{{ row.is_dir ? '-' : formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="引用" width="80">
          <template #default="{ row }">
            <el-tag v-if="!row.is_dir && row.ref_count > 0" type="warning" size="small">
              {{ row.ref_count }} 篇
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="修改时间" width="170">
          <template #default="{ row }">{{ formatDate(row.modified) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.is_dir" link type="primary" @click="openDir(asRow(row))">打开</el-button>
            <el-button v-else link type="primary" @click="preview(asRow(row))">预览</el-button>
            <el-button
              v-if="canRename"
              link
              type="primary"
              :disabled="!row.is_dir && row.ref_count > 0"
              :title="row.ref_count > 0 ? '被文章引用，不能重命名' : ''"
              @click="onRename(asRow(row))"
            >
              重命名
            </el-button>
            <el-button
              v-if="canDelete"
              link
              type="danger"
              :disabled="!row.is_dir && row.ref_count > 0"
              :title="row.ref_count > 0 ? '被文章引用，不能删除' : ''"
              @click="onDelete(asRow(row))"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无文件" />
        </template>
      </el-table>

      <el-pagination
        class="pager"
        layout="total, sizes, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        :page-sizes="[10, 20, 50, 100]"
        @current-change="onPageChange"
        @size-change="onSizeChange"
      />
    </el-card>

    <el-dialog v-model="renameVisible" title="重命名" width="420px">
      <el-input v-model="newName" placeholder="请输入新名称" />
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRename">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from '@/utils/element'
import { useUserStore } from '@/stores/user'
import { deleteFile, getFileList, renameFile, type FileItem } from '@/api/file'

// el-table 作用域插槽的 row 为 DefaultRow，这里显式收敛为 FileItem
const asRow = (row: unknown) => row as FileItem

const userStore = useUserStore()
const canRename = computed(() => userStore.hasPermission('system:file:rename'))
const canDelete = computed(() => userStore.hasPermission('system:file:delete'))

const loading = ref(false)
const list = ref<FileItem[]>([])
const total = ref(0)
const currentDir = ref('')
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)

const renameVisible = ref(false)
const renameTarget = ref<FileItem | null>(null)
const newName = ref('')

const pathSegments = computed(() => {
  if (!currentDir.value) return []
  const parts = currentDir.value.split('/').filter(Boolean)
  const segs: { name: string; path: string }[] = []
  let acc = ''
  for (const p of parts) {
    acc = acc ? `${acc}/${p}` : p
    segs.push({ name: p, path: acc })
  }
  return segs
})

async function load() {
  loading.value = true
  try {
    const resp = await getFileList({
      dir: currentDir.value,
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value,
    })
    list.value = resp.list || []
    total.value = resp.total || 0
    currentDir.value = resp.current_dir || currentDir.value
  } catch {
    // 错误已由响应拦截器提示
  } finally {
    loading.value = false
  }
}

function openDir(item: FileItem) {
  currentDir.value = item.path
  page.value = 1
  load()
}

function navigate(path: string) {
  currentDir.value = path
  page.value = 1
  load()
}

function goUp() {
  if (!currentDir.value) return
  const idx = currentDir.value.lastIndexOf('/')
  currentDir.value = idx >= 0 ? currentDir.value.slice(0, idx) : ''
  page.value = 1
  load()
}

function onSearch() {
  page.value = 1
  load()
}

function onPageChange(p: number) {
  page.value = p
  load()
}

function onSizeChange(s: number) {
  pageSize.value = s
  page.value = 1
  load()
}

function preview(item: FileItem) {
  window.open(item.url, '_blank')
}

function onRename(item: FileItem) {
  renameTarget.value = item
  newName.value = item.name
  renameVisible.value = true
}

async function submitRename() {
  if (!renameTarget.value) return
  const name = newName.value.trim()
  if (!name || name.includes('/')) {
    ElMessage.warning('名称不能为空且不能包含 /')
    return
  }
  try {
    await renameFile(renameTarget.value.path, name)
    ElMessage.success('重命名成功')
    renameVisible.value = false
    load()
  } catch {
    // 拦截器已提示
  }
}

async function onDelete(item: FileItem) {
  try {
    await ElMessageBox.confirm(`确定删除「${item.name}」吗？此操作不可恢复。`, '提示', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteFile(item.path)
    ElMessage.success('删除成功')
    load()
  } catch {
    // 拦截器已提示
  }
}

function formatSize(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let val = bytes
  let i = 0
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatDate(unix: number): string {
  if (!unix) return '-'
  return new Date(unix * 1000).toLocaleString()
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
.actions {
  display: flex;
  gap: 8px;
}
.name-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.icon {
  font-size: 16px;
}
.link {
  color: var(--el-color-primary);
  cursor: pointer;
}
.pager {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
