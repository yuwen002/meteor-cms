<template>
  <el-card>
    <el-form :inline="true" :model="query" class="search-bar">
      <el-form-item label="用户名">
        <el-input v-model="query.username" placeholder="操作人" clearable @keyup.enter="handleSearch" />
      </el-form-item>
      <el-form-item label="模块">
        <el-select v-model="query.module" placeholder="全部模块" clearable style="width: 150px">
          <el-option v-for="m in moduleOptions" :key="m" :label="m" :value="m" />
        </el-select>
      </el-form-item>
      <el-form-item label="路径">
        <el-input v-model="query.path" placeholder="请求路径" clearable @keyup.enter="handleSearch" />
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
        <el-button v-if="canStats" @click="openStats">统计</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="list" v-loading="loading" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="操作人" width="120" />
      <el-table-column prop="module" label="模块" width="120" />
      <el-table-column prop="action" label="操作" width="120" />
      <el-table-column prop="method" label="方法" width="80" />
      <el-table-column prop="path" label="路径" min-width="220" show-overflow-tooltip />
      <el-table-column prop="ip" label="IP" width="130" />
      <el-table-column label="结果" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 0 ? 'success' : 'danger'">{{ row.status === 0 ? '成功' : '失败' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="duration_ms" label="耗时(ms)" width="100" />
      <el-table-column prop="created_at" label="时间" width="180" />
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

    <el-drawer v-model="detailVisible" title="操作日志详情" size="560px">
      <template v-if="current">
        <div class="detail-list">
          <div v-for="item in detailRows" :key="item.label" class="detail-row">
            <span class="detail-label">{{ item.label }}</span>
            <span class="detail-value">{{ item.value || '-' }}</span>
          </div>
        </div>
        <div class="detail-section">
          <div class="detail-label">请求参数（已脱敏）</div>
          <pre class="detail-pre">{{ formattedParams || '-' }}</pre>
        </div>
        <div class="detail-section">
          <div class="detail-label">Query 参数（已脱敏）</div>
          <pre class="detail-pre">{{ current.query || '-' }}</pre>
        </div>
      </template>
    </el-drawer>

    <el-drawer v-model="statsVisible" title="操作日志统计（近 7 天）" size="520px">
      <div v-loading="statsLoading">
        <template v-if="stats">
          <div class="stat-summary">
            <div class="stat-item"><span>总操作</span><strong>{{ stats.total }}</strong></div>
            <div class="stat-item"><span>成功</span><strong>{{ stats.success }}</strong></div>
            <div class="stat-item"><span>失败</span><strong>{{ stats.failed }}</strong></div>
          </div>
          <h4>按天趋势</h4>
          <el-table :data="stats.by_day" size="small" border>
            <el-table-column prop="date" label="日期" />
            <el-table-column prop="total" label="总数" />
            <el-table-column prop="failed" label="失败" />
          </el-table>
          <h4>按模块</h4>
          <el-table :data="stats.by_module" size="small" border>
            <el-table-column prop="module" label="模块" />
            <el-table-column prop="total" label="总数" />
          </el-table>
          <h4>失败业务码</h4>
          <el-table :data="stats.by_biz_code" size="small" border>
            <el-table-column prop="biz_code" label="业务码" />
            <el-table-column prop="total" label="次数" />
          </el-table>
        </template>
      </div>
    </el-drawer>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  downloadLogExport,
  getLogStats,
  getOperationLogDetail,
  getOperationLogs,
} from '@/api/log'
import type { LogStatsResp, OperationLogItem, OperationLogQuery } from '@/api/log'
import { useUserStore } from '@/stores/user'
import { ElMessage } from '@/utils/element'

const userStore = useUserStore()
// 详情复用列表权限，导出/统计使用独立权限码，便于按岗位授权
const canViewDetail = computed(() => userStore.hasPermission('system:log:operation_list'))
const canExport = computed(() => userStore.hasPermission('system:log:export'))
const canStats = computed(() => userStore.hasPermission('system:log:stats'))

const moduleOptions = [
  '管理员管理',
  '部门管理',
  '角色管理',
  '权限管理',
  '个人中心',
  '文章管理',
  '分类管理',
  '标签管理',
  '回收站',
  '文件管理',
  '字典管理',
  '配置管理',
  '控制台',
  '其他',
]

const loading = ref(false)
const exporting = ref(false)
const list = ref<OperationLogItem[]>([])
const total = ref(0)
const timeRange = ref<[string, string]>(['', ''])
const detailVisible = ref(false)
const current = ref<OperationLogItem | null>(null)
const statsVisible = ref(false)
const statsLoading = ref(false)
const stats = ref<LogStatsResp | null>(null)
const query = reactive<OperationLogQuery>({
  page: 1,
  page_size: 10,
  username: '',
  module: '',
  path: '',
  status: undefined,
  start_time: '',
  end_time: '',
})

const detailRows = computed(() => {
  const r = current.value
  if (!r) return []
  return [
    { label: '日志 ID', value: String(r.id) },
    { label: '操作人', value: r.username ? `${r.username}（ID: ${r.user_id}）` : String(r.user_id) },
    { label: '业务模块', value: r.module },
    { label: '操作', value: r.action },
    { label: '请求', value: `${r.method} ${r.path}` },
    { label: '来源 IP', value: r.ip },
    { label: '结果', value: r.status === 0 ? '成功' : '失败' },
    { label: '业务码', value: String(r.biz_code) },
    { label: 'HTTP 状态', value: String(r.http_status) },
    { label: '链路 ID', value: r.request_id },
    { label: '失败原因', value: r.error_msg },
    { label: '耗时', value: `${r.duration_ms} ms` },
    { label: '客户端', value: r.user_agent },
    { label: '发生时间', value: r.created_at },
  ]
})

const formattedParams = computed(() => {
  const raw = current.value?.params
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
})

function syncTimeRange() {
  query.start_time = timeRange.value?.[0] || ''
  query.end_time = timeRange.value?.[1] || ''
}

async function loadList() {
  loading.value = true
  try {
    syncTimeRange()
    const res = await getOperationLogs(query)
    list.value = res.list || []
    total.value = res.total || 0
    // 后端可能按上限裁剪分页参数，回写避免分页器与真实数据不一致
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
  query.module = ''
  query.path = ''
  query.status = undefined
  timeRange.value = ['', '']
  query.page = 1
  loadList()
}

async function openDetail(row: unknown) {
  // el-table 的插槽行类型为 DefaultRow，这里显式收窄为接口类型
  const item = row as OperationLogItem
  current.value = item
  detailVisible.value = true
  try {
    current.value = await getOperationLogDetail(item.id)
  } catch {
    // 详情接口不可用时沿用列表行数据，抽屉仍可查看
  }
}

async function handleExport() {
  exporting.value = true
  try {
    syncTimeRange()
    await downloadLogExport({
      type: 'operation',
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

async function openStats() {
  statsVisible.value = true
  statsLoading.value = true
  try {
    stats.value = await getLogStats(7)
  } finally {
    statsLoading.value = false
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
.detail-section {
  margin-top: 18px;
}
.detail-pre {
  margin: 8px 0 0;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 4px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 320px;
  overflow: auto;
}
.stat-summary {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.stat-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 4px;
  font-size: 12px;
  color: #909399;
}
.stat-item strong {
  font-size: 20px;
  color: #303133;
}
h4 {
  margin: 16px 0 8px;
  font-size: 13px;
}
</style>
