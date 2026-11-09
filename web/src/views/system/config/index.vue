<template>
  <div class="config-page">
    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="配置键/名称"
          clearable
          style="width: 220px"
        />
        <el-button v-if="canAdd" type="primary" @click="openCreate">新增配置</el-button>
        <el-button
          type="success"
          :disabled="dirtyIds.length === 0 || !canEdit"
          :loading="saving"
          @click="saveBatch"
        >
          保存修改<span v-if="dirtyIds.length">（{{ dirtyIds.length }}）</span>
        </el-button>
        <span class="tip">密文配置显示 ******，保持不变即可；留空可清空配置值</span>
      </div>

      <el-tabs v-model="activeGroup" class="group-tabs">
        <el-tab-pane label="全部" name="" />
        <el-tab-pane v-for="g in groupTabs" :key="g.value" :label="g.label" :name="g.value" />
      </el-tabs>

      <el-table v-loading="loading" :data="shownList" stripe>
        <el-table-column prop="config_key" label="配置键" min-width="170" />
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column label="配置值" min-width="240">
          <template #default="{ row }">
            <el-select
              v-if="asRow(row).value_type === 'bool'"
              v-model="draft[asRow(row).id]"
              :disabled="!canEdit"
              style="width: 100%"
            >
              <el-option v-for="o in yesNoOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
            <el-input
              v-else
              v-model="draft[asRow(row).id]"
              :type="inputType(asRow(row))"
              :rows="2"
              :show-password="asRow(row).is_secret"
              :disabled="!canEdit"
              :placeholder="asRow(row).is_secret ? '输入新值以替换' : ''"
            />
          </template>
        </el-table-column>
        <el-table-column label="分组" width="90">
          <template #default="{ row }">{{ groupLabel(asRow(row).config_group) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="asRow(row).status === 1 ? 'success' : 'info'" size="small">
              {{ asRow(row).status === 1 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canView" link type="primary" @click="openDetail(asRow(row))">详情</el-button>
            <el-button v-if="canView" link type="primary" @click="openVersions(asRow(row))">历史</el-button>
            <el-button v-if="canEdit" link type="primary" @click="openEdit(asRow(row))">编辑</el-button>
            <el-button
              v-if="canDelete"
              link
              type="danger"
              :disabled="asRow(row).is_system"
              @click="onDelete(asRow(row))"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无配置" />
        </template>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px">
      <el-form :model="form" label-width="96px">
        <el-form-item label="配置键" required>
          <el-input v-model="form.config_key" :disabled="isEdit" placeholder="如 site.name" />
        </el-form-item>
        <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="配置值">
          <el-input
            v-model="form.config_value"
            :type="form.is_secret ? 'password' : form.value_type === 'text' || form.value_type === 'json' ? 'textarea' : 'text'"
            :rows="3"
            :show-password="form.is_secret"
            :placeholder="isEdit && form.is_secret ? '保持不变则不修改' : ''"
          />
        </el-form-item>
        <el-form-item label="分组">
          <el-select
            v-model="form.config_group"
            :disabled="isEdit && form.is_system"
            filterable
            allow-create
            style="width: 100%"
          >
            <el-option v-for="g in groupTabs" :key="g.value" :label="g.label" :value="g.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="值类型">
          <el-select v-model="form.value_type" :disabled="isEdit && form.is_system" style="width: 100%">
            <el-option v-for="t in valueTypeOptions" :key="t.value" :label="t.label" :value="t.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="校验正则">
          <el-input
            v-model="form.validation"
            :disabled="isEdit && form.is_system"
            placeholder="可选，如 ^#[0-9A-Fa-f]{6}$"
          />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :disabled="isEdit && form.is_system" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch
            v-model="form.status"
            :active-value="1"
            :inactive-value="0"
            :disabled="isEdit && form.is_system"
          />
        </el-form-item>
        <el-form-item label="加密存储">
          <el-switch v-model="form.is_secret" :disabled="isEdit" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="detailVisible" title="配置详情" width="520px">
      <el-descriptions v-if="detail" :column="1" border>
        <el-descriptions-item label="配置键">{{ detail.config_key }}</el-descriptions-item>
        <el-descriptions-item label="名称">{{ detail.name }}</el-descriptions-item>
        <el-descriptions-item label="配置值">{{ detail.config_value }}</el-descriptions-item>
        <el-descriptions-item label="分组">{{ detail.config_group }}</el-descriptions-item>
        <el-descriptions-item label="值类型">{{ detail.value_type }}</el-descriptions-item>
        <el-descriptions-item label="校验规则">{{ detail.validation || '无' }}</el-descriptions-item>
        <el-descriptions-item label="备注">{{ detail.remark || '无' }}</el-descriptions-item>
        <el-descriptions-item label="内置">{{ detail.is_system ? '是' : '否' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <el-drawer v-model="versionVisible" :title="`变更历史 - ${versionTarget?.name ?? ''}`" size="560px">
      <el-table v-loading="versionLoading" :data="versions" size="small" stripe>
        <el-table-column prop="created_at" label="时间" width="170" />
        <el-table-column prop="operator_name" label="操作人" width="90" />
        <el-table-column label="原值" min-width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ asVersion(row).old_value || '（空）' }}</template>
        </el-table-column>
        <el-table-column label="新值" min-width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ asVersion(row).new_value || '（空）' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canEdit" link type="primary" @click="onRollback(asVersion(row))">回滚</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无变更记录" :image-size="60" />
        </template>
      </el-table>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from '@/utils/element'
import { useUserStore } from '@/stores/user'
import {
  batchUpdateConfigs,
  createConfig,
  deleteConfig,
  getConfigDetail,
  getConfigVersions,
  getConfigs,
  rollbackConfig,
  updateConfig,
  type ConfigItem,
  type ConfigVersionItem,
  type ConfigWritePayload,
} from '@/api/config'
import { useDictOptions, type DictOption } from '@/utils/dictOptions'

// el-table 作用域插槽的 row 为 DefaultRow，这里显式收敛为具体类型
const asRow = (row: unknown) => row as ConfigItem
const asVersion = (row: unknown) => row as ConfigVersionItem

const userStore = useUserStore()
const canView = computed(() => userStore.hasPermission('system:config:list'))
const canAdd = computed(() => userStore.hasPermission('system:config:add'))
const canEdit = computed(() => userStore.hasPermission('system:config:edit'))
const canDelete = computed(() => userStore.hasPermission('system:config:delete'))

const fallbackGroups: DictOption[] = [
  { label: '站点', value: 'site' },
  { label: 'SEO', value: 'seo' },
  { label: '邮件', value: 'mail' },
  { label: '安全', value: 'security' },
  { label: '主题', value: 'theme' },
  { label: '功能', value: 'feature' },
  { label: '通用', value: 'general' },
]
const fallbackValueTypes: DictOption[] = [
  { label: '字符串', value: 'string' },
  { label: '多行文本', value: 'text' },
  { label: '整数', value: 'int' },
  { label: '布尔', value: 'bool' },
  { label: 'JSON', value: 'json' },
  { label: 'URL', value: 'url' },
  { label: '图片', value: 'image' },
]
const groupDict = useDictOptions('config_group', fallbackGroups)
const valueTypeDict = useDictOptions('config_value_type', fallbackValueTypes)
const yesNoDict = useDictOptions('yes_no', [
  { label: '是', value: '1' },
  { label: '否', value: '0' },
])

const groupTabs = computed<DictOption[]>(() => {
  const out = [...groupDict.options.value]
  const seen = new Set(out.map((o) => o.value))
  for (const row of list.value) {
    if (row.config_group && !seen.has(row.config_group)) {
      seen.add(row.config_group)
      out.push({ label: row.config_group, value: row.config_group })
    }
  }
  return out
})
const valueTypeOptions = computed<DictOption[]>(() => valueTypeDict.options.value)
const yesNoOptions = computed<DictOption[]>(() => yesNoDict.options.value)

const loading = ref(false)
const saving = ref(false)
const list = ref<ConfigItem[]>([])
const keyword = ref('')
const activeGroup = ref('')
const draft = reactive<Record<number, string>>({})

const shownList = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return list.value.filter((row) => {
    if (activeGroup.value && row.config_group !== activeGroup.value) return false
    if (!kw) return true
    return row.config_key.toLowerCase().includes(kw) || row.name.toLowerCase().includes(kw)
  })
})

const dirtyIds = computed(() =>
  list.value.filter((row) => draft[row.id] !== row.config_value).map((row) => row.id),
)

function groupLabel(value: string): string {
  return groupTabs.value.find((g) => g.value === value)?.label || value || '通用'
}

function inputType(item: ConfigItem): string {
  if (item.is_secret) return 'password'
  if (item.value_type === 'text' || item.value_type === 'json') return 'textarea'
  return 'text'
}

async function load() {
  loading.value = true
  try {
    const resp = await getConfigs()
    list.value = resp.list || []
    for (const key of Object.keys(draft)) delete draft[Number(key)]
    for (const row of list.value) draft[row.id] = row.config_value
  } finally {
    loading.value = false
  }
}

async function saveBatch() {
  if (dirtyIds.value.length === 0) return
  saving.value = true
  try {
    await batchUpdateConfigs(
      dirtyIds.value.map((id) => ({ id, config_value: draft[id] ?? '' })),
    )
    ElMessage.success('配置已保存')
    await load()
  } finally {
    saving.value = false
  }
}

const dialogVisible = ref(false)
const dialogTitle = ref('')
const isEdit = ref(false)
const editId = ref(0)
const submitting = ref(false)
interface ConfigForm {
  config_key: string
  name: string
  config_value: string
  remark: string
  config_group: string
  value_type: string
  validation: string
  sort: number
  status: number
  is_secret: boolean
  is_system: boolean
}
const emptyForm: ConfigForm = {
  config_key: '',
  name: '',
  config_value: '',
  remark: '',
  config_group: 'general',
  value_type: 'string',
  validation: '',
  sort: 0,
  status: 1,
  is_secret: false,
  is_system: false,
}
const form = reactive<ConfigForm>({ ...emptyForm })

function openCreate() {
  isEdit.value = false
  dialogTitle.value = '新增配置'
  Object.assign(form, emptyForm)
  dialogVisible.value = true
}

function openEdit(item: ConfigItem) {
  isEdit.value = true
  dialogTitle.value = '编辑配置'
  Object.assign(form, {
    config_key: item.config_key,
    name: item.name,
    config_value: item.config_value,
    remark: item.remark,
    config_group: item.config_group,
    value_type: item.value_type,
    validation: item.validation,
    sort: item.sort,
    status: item.status,
    is_secret: item.is_secret,
    is_system: item.is_system,
  })
  editId.value = item.id
  dialogVisible.value = true
}

async function submit() {
  if (!form.config_key.trim() || !form.name.trim()) {
    ElMessage.warning('配置键与名称不能为空')
    return
  }
  submitting.value = true
  try {
    const common: ConfigWritePayload = {
      name: form.name.trim(),
      remark: form.remark,
      config_group: form.config_group,
      value_type: form.value_type,
      validation: form.validation,
      sort: form.sort,
      status: form.status,
    }
    if (isEdit.value) {
      const payload: ConfigWritePayload = { ...common }
      // 密文项保持掩码时不提交，避免把 ****** 当成真实值写入
      if (!(form.is_secret && form.config_value === '******')) {
        payload.config_value = form.config_value
      }
      await updateConfig(editId.value, payload)
      ElMessage.success('更新成功')
    } else {
      await createConfig({
        ...common,
        config_key: form.config_key.trim(),
        config_value: form.config_value,
        is_secret: form.is_secret,
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await load()
  } finally {
    submitting.value = false
  }
}

async function onDelete(item: ConfigItem) {
  if (item.is_system) {
    ElMessage.warning('系统内置配置不可删除')
    return
  }
  try {
    await ElMessageBox.confirm(`确定删除配置「${item.name}」吗？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  await deleteConfig(item.id)
  ElMessage.success('删除成功')
  await load()
}

const detailVisible = ref(false)
const detail = ref<ConfigItem | null>(null)

async function openDetail(item: ConfigItem) {
  detail.value = await getConfigDetail(item.id)
  detailVisible.value = true
}

const versionVisible = ref(false)
const versionLoading = ref(false)
const versions = ref<ConfigVersionItem[]>([])
const versionTarget = ref<ConfigItem | null>(null)

async function loadVersions(id: number) {
  versionLoading.value = true
  try {
    versions.value = (await getConfigVersions(id)).list || []
  } finally {
    versionLoading.value = false
  }
}

async function openVersions(item: ConfigItem) {
  versionTarget.value = item
  versionVisible.value = true
  await loadVersions(item.id)
}

async function onRollback(version: ConfigVersionItem) {
  if (!versionTarget.value) return
  try {
    await ElMessageBox.confirm('确定把该配置回滚到此版本吗？回滚本身也会记录一条历史。', '提示', {
      type: 'warning',
    })
  } catch {
    return
  }
  const id = versionTarget.value.id
  await rollbackConfig(id, version.id)
  ElMessage.success('已回滚')
  await load()
  await loadVersions(id)
}

onMounted(() => {
  void groupDict.load()
  void valueTypeDict.load()
  void yesNoDict.load()
  void load()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.group-tabs {
  margin-bottom: 8px;
}
.tip {
  color: #909399;
  font-size: 12px;
}
</style>
