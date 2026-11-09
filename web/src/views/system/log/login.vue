<template>
  <el-card>
    <el-form :inline="true" :model="query" class="search-bar">
      <el-form-item label="用户名">
        <el-input v-model="query.username" placeholder="账号" clearable @keyup.enter="handleSearch" />
      </el-form-item>
      <el-form-item label="结果">
        <el-select v-model="query.status" placeholder="全部" clearable style="width: 120px">
          <el-option label="成功" :value="0" />
          <el-option label="失败" :value="1" />
        </el-select>
      </el-form-item>
      <el-form-item label="时间">
        <el-date-picker
          v-model="timeRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          range-separator="至"
          start-placeholder="开始"
          end-placeholder="结束"
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="handleSearch">查询</el-button>
        <el-button @click="resetQuery">重置</el-button>
        <el-button v-if="canExport" :loading="exporting" @click="handleExport">导出 CSV</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="list" v-loading="loading" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="账号" width="140" />
      <el-table-column prop="ip" label="IP" width="150" />
      <el-table-column label="结果" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 0 ? 'success' : 'danger'">{{ row.status === 0 ? '成功' : '失败' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="fail_reason" label="失败原因" min-width="180" show-overflow-tooltip />
      <el-table-column prop="login_at" label="登录时间" width="200" />
      <el-table-column label="操作" width="80" fixed="right">
        <template #default="{ row }">
          <el-button v-if="canViewDetail" link type="primary" size="small" @click="openDetail(row)">
            详情
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager"
      background
      layout="total, sizes, prev, pager, next, jumper"
      :total="total"
      :current-page="query.page"
      :page-size="query.page_size"
      :page-sizes="[10, 20, 50, 100]"
      @current-change="handlePageChange"
      @size-change="handleSizeChange"
    />

    <el-drawer v-model="detailVisible" title="登录日志详情" size="520px">
      <template v-if="current">
        <div class="detail-list">
          <div v-for="item in detailRows" :key="item.label" class="detail-row">
            <span class="detail-label">{{ item.label }}</span>
            <span class="detail-value">{{ item.value || '-' }}</span>
          </div>
        </div>
      </template>
    </el-drawer>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { downloadLogExport, getLoginLogDetail, getLoginLogs } from '@/api/log'
import type { LoginLogItem, LoginLogQuery } from '@/api/log'
import { useUserStore } from '@/stores/user'
import { ElMessage } from '@/utils/element'

const userStore = useUserStore()
// 详情复用列表权限，导出使用独立权限码
const canViewDetail = computed(() => userStore.hasPermission('system:log:login_list'))
const canExport = computed(() => userStore.hasPermission('system:log:export'))

const loading = ref(false)
const exporting = ref(false)
const list = ref<LoginLogItem[]>([])
const total = ref(0)
const timeRange = ref<[string, string]>(['', ''])
const detailVisible = ref(false)
const current = ref<LoginLogItem | null>(null)
const query = reactive<LoginLogQuery>({
  page: 1,
  page_size: 10,
  username: '',
  status: undefined,
  start_time: '',
  end_time: '',
})

const detailRows = computed(() => {
  const r = current.value
  if (!r) return []
  return [
    { label: '日志 ID', value: String(r.id) },
    { label: '账号', value: r.username ? `${r.username}（ID: ${r.user_id}）` : String(r.user_id) },
    { label: '来源 IP', value: r.ip },
    { label: '结果', value: r.status === 0 ? '成功' : '失败' },
    { label: '失败原因', value: r.fail_reason },
    { label: '链路 ID', value: r.request_id },
    { label: '客户端', value: r.user_agent },
    { label: '登录时间', value: r.login_at },
  ]
})

function syncTimeRange() {
  query.start_time = timeRange.value?.[0] || ''
  query.end_time = timeRange.value?.[1] || ''
}

async function loadList() {
  loading.value = true
  try {
    syncTimeRange()
    const res = await getLoginLogs(query)
    list.value = res.list || []
    total.value = res.total || 0
    query.page = res.page || query.page
    query.page_size = res.page_size || query.page_size
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  loadList()
}

function handlePageChange(page: number) {
  query.page = page
  loadList()
}

function handleSizeChange(size: number) {
  query.page_size = size
  query.page = 1
  loadList()
}

function resetQuery() {
  query.username = ''
  query.status = undefined
  timeRange.value = ['', '']
  query.page = 1
  loadList()
}

async function openDetail(row: unknown) {
  // el-table 的插槽行类型为 DefaultRow，这里显式收窄为接口类型
  const item = row as LoginLogItem
  current.value = item
  detailVisible.value = true
  try {
    current.value = await getLoginLogDetail(item.id)
  } catch {
    // 详情接口不可用时沿用列表行数据
  }
}

async function handleExport() {
  exporting.value = true
  try {
    syncTimeRange()
    await downloadLogExport({
      type: 'login',
      status: query.status,
      start_time: query.start_time || undefined,
      end_time: query.end_time || undefined,
    })
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '导出失败')
  } finally {
    exporting.value = false
  }
}

onMounted(loadList)
</script>

<style scoped>
.search-bar {
  margin-bottom: 12px;
}
.pager {
  margin-top: 12px;
  justify-content: flex-end;
}
.detail-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.detail-row {
  display: flex;
  gap: 12px;
  font-size: 13px;
  line-height: 1.6;
}
.detail-label {
  width: 88px;
  flex: none;
  color: #909399;
}
.detail-value {
  flex: 1;
  word-break: break-all;
}
</style>
