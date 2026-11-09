<template>
  <div class="dashboard">
    <el-row :gutter="16">
      <el-col :span="6" v-for="card in cards" :key="card.label">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-label">{{ card.label }}</div>
          <div class="stat-value">{{ card.value }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="welcome-card" v-loading="loading">
      <template #header>欢迎</template>
      <p>
        你好，{{ userStore.userInfo?.nickname || userStore.userInfo?.username }}（{{
          isSuper ? '超级管理员' : '普通管理员'
        }}）。
      </p>
      <p class="sub">账号状态：{{ userStore.userInfo?.isActive ? '已启用' : '已禁用' }}</p>
    </el-card>

    <el-card v-if="canMonitor" class="monitor-card" v-loading="monitorLoading">
      <template #header>
        <div class="monitor-header">
          <span>
            运行监控
            <el-tag :type="monitorTagType" size="small" effect="plain">{{ monitor?.status || '-' }}</el-tag>
          </span>
          <el-button size="small" @click="loadMonitor">刷新</el-button>
        </div>
      </template>
      <el-descriptions v-if="monitor" :column="3" border>
        <el-descriptions-item label="运行时长">{{ formatUptime(monitor.uptime_seconds) }}</el-descriptions-item>
        <el-descriptions-item label="数据库 / Redis">
          {{ monitor.database }} / {{ monitor.redis }}
        </el-descriptions-item>
        <el-descriptions-item label="Go 版本">{{ monitor.go_version }}</el-descriptions-item>
        <el-descriptions-item label="Goroutine">{{ monitor.goroutines }}</el-descriptions-item>
        <el-descriptions-item label="堆内存 / 系统内存">
          {{ monitor.heap_alloc_mb }} MB / {{ monitor.sys_mb }} MB
        </el-descriptions-item>
        <el-descriptions-item label="GC 次数">{{ monitor.num_gc }}</el-descriptions-item>
        <el-descriptions-item label="请求总数">{{ monitor.requests_total }}</el-descriptions-item>
        <el-descriptions-item label="5xx / 在途">
          {{ monitor.requests_server_error }} / {{ monitor.requests_in_flight }}
        </el-descriptions-item>
        <el-descriptions-item label="平均 QPS">{{ monitor.avg_qps }}</el-descriptions-item>
        <el-descriptions-item label="审计队列积压">{{ monitor.audit_queue_depth }}</el-descriptions-item>
        <el-descriptions-item label="审计丢弃累计">{{ monitor.audit_queue_dropped }}</el-descriptions-item>
      </el-descriptions>
      <el-empty v-else-if="!monitorLoading" description="暂无监控数据" :image-size="60" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useUserStore } from '@/stores/user'
import { getDashboardStats } from '@/api/dashboard'
import type { DashboardStats } from '@/api/dashboard'
import { getMonitor, type MonitorInfo } from '@/api/monitor'

const userStore = useUserStore()
const isSuper = computed(() => userStore.userInfo?.isSuper || false)
const canMonitor = computed(() => userStore.hasPermission('system:monitor:view'))
const loading = ref(false)
const stats = ref<DashboardStats | null>(null)
const monitorLoading = ref(false)
const monitor = ref<MonitorInfo | null>(null)

const cards = computed(() => [
  { label: '管理员总数', value: stats.value?.admin_count ?? '-' },
  { label: '部门总数', value: stats.value?.dept_count ?? '-' },
  { label: '角色总数', value: stats.value?.role_count ?? '-' },
  { label: '权限总数', value: stats.value?.permission_count ?? '-' },
  { label: '今日登录', value: stats.value?.today_login_count ?? '-' },
  { label: '今日失败登录', value: stats.value?.today_fail_login ?? '-' },
  { label: '近7天操作数', value: stats.value?.week_op_count ?? '-' },
])

const monitorTagType = computed<'success' | 'warning' | 'danger'>(() => {
  if (monitor.value?.status === 'unhealthy') return 'danger'
  if (monitor.value?.status === 'degraded') return 'warning'
  return 'success'
})

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return h > 0 ? `${h} 小时 ${m} 分` : `${m} 分 ${seconds % 60} 秒`
}

async function loadMonitor() {
  if (!canMonitor.value) return
  monitorLoading.value = true
  try {
    monitor.value = await getMonitor()
  } catch {
    monitor.value = null
  } finally {
    monitorLoading.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    stats.value = await getDashboardStats()
  } finally {
    loading.value = false
  }
  void loadMonitor()
})
</script>

<style scoped>
.stat-card { margin-bottom: 16px; }
.stat-label { color: #909399; font-size: 13px; }
.stat-value { font-size: 26px; font-weight: 600; margin-top: 8px; }
.welcome-card, .monitor-card { margin-top: 8px; }
.monitor-header { display: flex; align-items: center; justify-content: space-between; }
.monitor-header .el-tag { margin-left: 8px; }
.sub { color: #909399; font-size: 13px; margin-top: 4px; }
</style>
