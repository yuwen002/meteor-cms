<template>
  <div class="siteext-page">
    <el-card shadow="never">
      <el-tabs v-model="tab" @tab-change="loadCurrent">
        <el-tab-pane label="友情链接" name="links">
          <div class="toolbar">
            <el-input v-model="linkKeyword" placeholder="名称/链接" clearable style="width: 220px" @keyup.enter="loadLinks" @clear="loadLinks" />
            <el-button @click="loadLinks">搜索</el-button>
            <el-button type="primary" @click="openLinkCreate">新增友链</el-button>
          </div>
          <el-table v-loading="loading" :data="links" stripe>
            <el-table-column prop="name" label="站点名称" min-width="140" />
            <el-table-column prop="url" label="链接" min-width="200" show-overflow-tooltip />
            <el-table-column prop="link_type" label="类型" width="100" />
            <el-table-column prop="sort" label="排序" width="80" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="asLink(row).status === 1 ? 'success' : 'info'" size="small">
                  {{ asLink(row).status === 1 ? '启用' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="140" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openLinkEdit(asLink(row))">编辑</el-button>
                <el-button link type="danger" @click="onDeleteLink(asLink(row))">删除</el-button>
              </template>
            </el-table-column>
            <template #empty><el-empty description="暂无友链" :image-size="60" /></template>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="评论审核" name="comments">
          <div class="toolbar">
            <el-select v-model="commentStatus" placeholder="全部状态" clearable style="width: 140px" @change="loadComments">
              <el-option label="待审核" :value="0" />
              <el-option label="已通过" :value="1" />
              <el-option label="已拒绝" :value="2" />
            </el-select>
            <el-input v-model="commentKeyword" placeholder="昵称/内容" clearable style="width: 220px" @keyup.enter="loadComments" @clear="loadComments" />
            <el-button @click="loadComments">搜索</el-button>
          </div>
          <el-table v-loading="loading" :data="comments" stripe>
            <el-table-column prop="author" label="昵称" width="120" />
            <el-table-column prop="content" label="内容" min-width="240" show-overflow-tooltip />
            <el-table-column prop="email" label="邮箱" width="170" show-overflow-tooltip />
            <el-table-column prop="ip" label="IP" width="130" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="commentTag(asComment(row).status)" size="small">{{ commentLabel(asComment(row).status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" width="170" />
            <el-table-column label="操作" width="180" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="setCommentStatus(asComment(row), 1)">通过</el-button>
                <el-button link type="warning" @click="setCommentStatus(asComment(row), 2)">拒绝</el-button>
                <el-button link type="danger" @click="onDeleteComment(asComment(row))">删除</el-button>
              </template>
            </el-table-column>
            <template #empty><el-empty description="暂无评论" :image-size="60" /></template>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="邮件订阅" name="subscribers">
          <div class="toolbar">
            <el-input v-model="subscriberKeyword" placeholder="邮箱" clearable style="width: 220px" @keyup.enter="loadSubscribers" @clear="loadSubscribers" />
            <el-button @click="loadSubscribers">搜索</el-button>
          </div>
          <el-table v-loading="loading" :data="subscribers" stripe>
            <el-table-column prop="email" label="邮箱" min-width="220" />
            <el-table-column prop="source" label="来源" width="120" />
            <el-table-column prop="created_at" label="订阅时间" width="180" />
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button link type="danger" @click="onDeleteSubscriber(asSubscriber(row))">移除</el-button>
              </template>
            </el-table-column>
            <template #empty><el-empty description="暂无订阅" :image-size="60" /></template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog v-model="linkVisible" :title="linkEditId ? '编辑友链' : '新增友链'" width="520px">
      <el-form :model="linkForm" label-width="90px">
        <el-form-item label="站点名称" required><el-input v-model="linkForm.name" /></el-form-item>
        <el-form-item label="链接" required><el-input v-model="linkForm.url" placeholder="https://..." /></el-form-item>
        <el-form-item label="Logo"><el-input v-model="linkForm.logo" /></el-form-item>
        <el-form-item label="简介"><el-input v-model="linkForm.description" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="linkForm.link_type" style="width: 100%">
            <el-option v-for="o in linkTypeOptions" :key="o.value" :label="o.label" :value="o.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序"><el-input-number v-model="linkForm.sort" :min="0" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="linkForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="linkVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitLink">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from '@/utils/element'
import { useDictOptions } from '@/utils/dictOptions'
import {
  createLink,
  deleteComment,
  deleteLink,
  deleteSubscriber,
  getAdminComments,
  getAdminLinks,
  getAdminSubscribers,
  updateCommentStatus,
  updateLink,
  type CommentItem,
  type LinkItem,
  type SubscriberItem,
} from '@/api/siteext'

const asLink = (row: unknown) => row as LinkItem
const asComment = (row: unknown) => row as CommentItem
const asSubscriber = (row: unknown) => row as SubscriberItem

const tab = ref('links')
const loading = ref(false)
const submitting = ref(false)
const links = ref<LinkItem[]>([])
const comments = ref<CommentItem[]>([])
const subscribers = ref<SubscriberItem[]>([])
const linkKeyword = ref('')
const commentKeyword = ref('')
const commentStatus = ref<number | undefined>(undefined)
const subscriberKeyword = ref('')

const linkTypeDict = useDictOptions('link_type', [{ label: '友链', value: 'friend' }])
const linkTypeOptions = computed(() => linkTypeDict.options.value)

async function loadLinks() {
  loading.value = true
  try {
    links.value = (await getAdminLinks({ keyword: linkKeyword.value })).list || []
  } finally {
    loading.value = false
  }
}

async function loadComments() {
  loading.value = true
  try {
    comments.value = (
      await getAdminComments({ keyword: commentKeyword.value, status: commentStatus.value })
    ).list || []
  } finally {
    loading.value = false
  }
}

async function loadSubscribers() {
  loading.value = true
  try {
    subscribers.value = (await getAdminSubscribers({ keyword: subscriberKeyword.value })).list || []
  } finally {
    loading.value = false
  }
}

function loadCurrent() {
  if (tab.value === 'links') void loadLinks()
  else if (tab.value === 'comments') void loadComments()
  else void loadSubscribers()
}

const linkVisible = ref(false)
const linkEditId = ref(0)
const linkForm = reactive({
  name: '', url: '', logo: '', description: '', link_type: 'friend', sort: 0, enabled: true,
})

function openLinkCreate() {
  linkEditId.value = 0
  Object.assign(linkForm, { name: '', url: '', logo: '', description: '', link_type: 'friend', sort: 0, enabled: true })
  linkVisible.value = true
}

function openLinkEdit(item: LinkItem) {
  linkEditId.value = item.id
  Object.assign(linkForm, {
    name: item.name, url: item.url, logo: item.logo, description: item.description,
    link_type: item.link_type || 'friend', sort: item.sort, enabled: item.status === 1,
  })
  linkVisible.value = true
}

async function submitLink() {
  if (!linkForm.name.trim() || !/^https?:\/\//.test(linkForm.url)) {
    ElMessage.warning('请填写站点名称与 http(s) 链接')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: linkForm.name.trim(),
      url: linkForm.url,
      logo: linkForm.logo,
      description: linkForm.description,
      link_type: linkForm.link_type,
      sort: linkForm.sort,
      status: linkForm.enabled ? 1 : 0,
    }
    if (linkEditId.value) await updateLink(linkEditId.value, payload)
    else await createLink(payload)
    ElMessage.success('保存成功')
    linkVisible.value = false
    await loadLinks()
  } finally {
    submitting.value = false
  }
}

async function onDeleteLink(item: LinkItem) {
  try {
    await ElMessageBox.confirm(`确定删除友链「${item.name}」吗？`, '提示', { type: 'warning' })
  } catch { return }
  await deleteLink(item.id)
  ElMessage.success('删除成功')
  await loadLinks()
}

function commentLabel(status: number): string {
  return status === 1 ? '已通过' : status === 2 ? '已拒绝' : '待审核'
}
function commentTag(status: number): 'success' | 'danger' | 'warning' {
  return status === 1 ? 'success' : status === 2 ? 'danger' : 'warning'
}

async function setCommentStatus(item: CommentItem, status: number) {
  await updateCommentStatus(item.id, status)
  ElMessage.success('状态已更新')
  await loadComments()
}

async function onDeleteComment(item: CommentItem) {
  try {
    await ElMessageBox.confirm('确定删除这条评论吗？', '提示', { type: 'warning' })
  } catch { return }
  await deleteComment(item.id)
  ElMessage.success('删除成功')
  await loadComments()
}

async function onDeleteSubscriber(item: SubscriberItem) {
  try {
    await ElMessageBox.confirm(`确定移除订阅 ${item.email} 吗？`, '提示', { type: 'warning' })
  } catch { return }
  await deleteSubscriber(item.id)
  ElMessage.success('已移除')
  await loadSubscribers()
}

onMounted(() => {
  void linkTypeDict.load()
  void loadLinks()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
</style>
