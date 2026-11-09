<template>
  <div class="dict-page">
    <el-row :gutter="12">
      <el-col :span="10">
        <el-card shadow="never">
          <div class="toolbar">
            <el-input
              v-model="keyword"
              placeholder="编码/名称"
              clearable
              style="width: 160px"
              @keyup.enter="onSearch"
              @clear="onSearch"
            />
            <el-select v-model="status" placeholder="状态" clearable style="width: 100px" @change="onSearch">
              <el-option label="启用" :value="1" />
              <el-option label="禁用" :value="0" />
            </el-select>
            <el-select v-model="group" placeholder="分组" clearable style="width: 110px" @change="onSearch">
              <el-option v-for="g in groupOptions" :key="g.value" :label="g.label" :value="g.value" />
            </el-select>
            <el-button type="primary" @click="onSearch">搜索</el-button>
            <el-button v-if="canAdd" type="primary" @click="openCreate">新增字典</el-button>
          </div>

          <el-table
            v-loading="loading"
            :data="list"
            highlight-current-row
            stripe
            @current-change="onSelectDict"
          >
            <el-table-column prop="code" label="编码" min-width="110" />
            <el-table-column prop="name" label="名称" min-width="100" />
            <el-table-column label="分组" width="80">
              <template #default="{ row }">{{ groupLabel(asDict(row).dict_group) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="72">
              <template #default="{ row }">
                <el-tag :type="asDict(row).status === 1 ? 'success' : 'info'" size="small">
                  {{ asDict(row).status === 1 ? '启用' : '禁用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="内置" width="64">
              <template #default="{ row }">
                <el-tag v-if="asDict(row).is_system" type="warning" size="small">内置</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="sort" label="排序" width="64" />
            <el-table-column label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <el-button v-if="canEdit" link type="primary" @click="openEdit(asDict(row))">编辑</el-button>
                <el-button
                  v-if="canDelete"
                  link
                  type="danger"
                  :disabled="asDict(row).is_system"
                  @click="onDeleteDict(asDict(row))"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="暂无字典" :image-size="60" />
            </template>
          </el-table>

          <el-pagination
            class="pager"
            layout="total, prev, pager, next"
            :total="total"
            :page-size="pageSize"
            :current-page="page"
            @current-change="onPageChange"
          />
        </el-card>
      </el-col>

      <el-col :span="14">
        <el-card shadow="never">
          <div class="toolbar">
            <span class="title">字典项{{ current ? `（${current.name}）` : '' }}</span>
            <div class="actions">
              <el-button :disabled="!current" @click="loadItems">刷新</el-button>
              <el-button v-if="canItemAdd" type="primary" :disabled="!current" @click="openItemCreate">
                新增字典项
              </el-button>
            </div>
          </div>

          <el-table v-loading="itemLoading" :data="items" stripe>
            <el-table-column prop="label" label="标签" min-width="100" />
            <el-table-column prop="value" label="值" min-width="100" />
            <el-table-column label="父项" min-width="90">
              <template #default="{ row }">{{ parentLabel(asDictItem(row).parent_id) }}</template>
            </el-table-column>
            <el-table-column label="默认" width="64">
              <template #default="{ row }">
                <el-tag v-if="asDictItem(row).is_default" type="success" size="small">默认</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="72">
              <template #default="{ row }">
                <el-tag :type="asDictItem(row).status === 1 ? 'success' : 'info'" size="small">
                  {{ asDictItem(row).status === 1 ? '启用' : '禁用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="sort" label="排序" width="64" />
            <el-table-column label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <el-button v-if="canItemEdit" link type="primary" @click="openItemEdit(asDictItem(row))">
                  编辑
                </el-button>
                <el-button v-if="canItemDelete" link type="danger" @click="onDeleteItem(asDictItem(row))">
                  删除
                </el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty :description="current ? '暂无字典项' : '请先选择左侧字典'" :image-size="60" />
            </template>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-dialog v-model="dictVisible" :title="dictTitle" width="480px">
      <el-form :model="dictForm" label-width="90px">
        <el-form-item label="编码" required>
          <el-input v-model="dictForm.code" :disabled="isEdit" placeholder="如 user_status" />
        </el-form-item>
        <el-form-item label="名称" required><el-input v-model="dictForm.name" /></el-form-item>
        <el-form-item label="分组">
          <el-select v-model="dictForm.dict_group" filterable allow-create style="width: 100%">
            <el-option v-for="g in groupOptions" :key="g.value" :label="g.label" :value="g.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="dictForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="dictForm.sort" :min="0" /></el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="dictForm.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dictVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitDict">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="itemVisible" :title="itemTitle" width="480px">
      <el-form :model="itemForm" label-width="90px">
        <el-form-item label="标签" required><el-input v-model="itemForm.label" /></el-form-item>
        <el-form-item label="值" required><el-input v-model="itemForm.value" /></el-form-item>
        <el-form-item label="父项">
          <el-select v-model="itemForm.parent_id" clearable placeholder="无" style="width: 100%">
            <el-option label="（无）" :value="0" />
            <el-option v-for="p in parentOptions" :key="p.id" :label="p.label" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="默认项"><el-switch v-model="itemForm.is_default" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="itemForm.remark" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="itemForm.sort" :min="0" /></el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="itemForm.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="itemVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitItem">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from '@/utils/element'
import { useUserStore } from '@/stores/user'
import {
  createDict,
  createDictItem,
  deleteDict,
  deleteDictItem,
  getDictItems,
  getDicts,
  updateDict,
  updateDictItem,
  type DictDataItem,
  type DictItem,
} from '@/api/dict'
import { invalidateDictOptions, useDictOptions, type DictOption } from '@/utils/dictOptions'

// el-table 作用域插槽的 row 为 DefaultRow，这里显式收敛为具体类型
const asDict = (row: unknown) => row as DictItem
const asDictItem = (row: unknown) => row as DictDataItem

const userStore = useUserStore()
const canAdd = computed(() => userStore.hasPermission('system:dict:add'))
const canEdit = computed(() => userStore.hasPermission('system:dict:edit'))
const canDelete = computed(() => userStore.hasPermission('system:dict:delete'))
const canItemAdd = computed(() => userStore.hasPermission('system:dict_item:add'))
const canItemEdit = computed(() => userStore.hasPermission('system:dict_item:edit'))
const canItemDelete = computed(() => userStore.hasPermission('system:dict_item:delete'))

const fallbackGroups: DictOption[] = [
  { label: '内容', value: 'content' },
  { label: '站点', value: 'site' },
  { label: '系统', value: 'system' },
]
const groupDict = useDictOptions('dict_group', fallbackGroups)
const groupOptions = computed<DictOption[]>(() => groupDict.options.value)

const loading = ref(false)
const itemLoading = ref(false)
const list = ref<DictItem[]>([])
const items = ref<DictDataItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const keyword = ref('')
const status = ref<number | undefined>(undefined)
const group = ref<string | undefined>(undefined)
const current = ref<DictItem | null>(null)

function groupLabel(value: string): string {
  return groupOptions.value.find((g) => g.value === value)?.label || value || '未分组'
}

function parentLabel(id: number): string {
  if (!id) return ''
  return items.value.find((i) => i.id === id)?.label || `#${id}`
}

async function loadDicts() {
  loading.value = true
  try {
    const resp = await getDicts({
      keyword: keyword.value,
      status: status.value,
      group: group.value,
      page: page.value,
      page_size: pageSize.value,
    })
    list.value = resp.list || []
    total.value = resp.total || 0
    // 当前选中的字典若已不在列表中则清空右侧
    if (current.value && !list.value.some((d) => d.id === current.value?.id)) {
      current.value = null
      items.value = []
    }
  } finally {
    loading.value = false
  }
}

async function loadItems() {
  if (!current.value) {
    items.value = []
    return
  }
  itemLoading.value = true
  try {
    const resp = await getDictItems({ dict_code: current.value.code })
    items.value = resp.list || []
  } finally {
    itemLoading.value = false
  }
}

function onSelectDict(row: DictItem | null) {
  current.value = row ?? null
  void loadItems()
}

function onSearch() {
  page.value = 1
  void loadDicts()
}

function onPageChange(p: number) {
  page.value = p
  void loadDicts()
}

const dictVisible = ref(false)
const dictTitle = ref('')
const isEdit = ref(false)
const editId = ref(0)
const submitting = ref(false)
const emptyDictForm = { code: '', name: '', description: '', sort: 0, enabled: true, dict_group: 'system' }
const dictForm = reactive({ ...emptyDictForm })

function openCreate() {
  isEdit.value = false
  dictTitle.value = '新增字典'
  Object.assign(dictForm, emptyDictForm)
  dictVisible.value = true
}

function openEdit(item: DictItem) {
  isEdit.value = true
  dictTitle.value = '编辑字典'
  Object.assign(dictForm, {
    code: item.code,
    name: item.name,
    description: item.description,
    sort: item.sort,
    enabled: item.status === 1,
    dict_group: item.dict_group || 'system',
  })
  editId.value = item.id
  dictVisible.value = true
}

async function submitDict() {
  if (!dictForm.code.trim() || !dictForm.name.trim()) {
    ElMessage.warning('编码与名称不能为空')
    return
  }
  submitting.value = true
  try {
    const statusVal = dictForm.enabled ? 1 : 0
    if (isEdit.value) {
      await updateDict(editId.value, {
        name: dictForm.name,
        description: dictForm.description,
        status: statusVal,
        sort: dictForm.sort,
        dict_group: dictForm.dict_group,
      })
      ElMessage.success('更新成功')
    } else {
      await createDict({
        code: dictForm.code.trim(),
        name: dictForm.name.trim(),
        description: dictForm.description,
        status: statusVal,
        sort: dictForm.sort,
        dict_group: dictForm.dict_group,
      })
      ElMessage.success('创建成功')
    }
    // 字典类型启停会影响公开下拉，清掉前端缓存
    invalidateDictOptions()
    dictVisible.value = false
    await loadDicts()
  } finally {
    submitting.value = false
  }
}

async function onDeleteDict(item: DictItem) {
  if (item.is_system) {
    ElMessage.warning('系统内置字典不可删除')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定删除字典「${item.name}」吗？其下所有字典项会一并删除，且不可恢复。`,
      '提示',
      { type: 'warning' },
    )
  } catch {
    return
  }
  await deleteDict(item.id)
  invalidateDictOptions()
  ElMessage.success('删除成功')
  if (current.value?.id === item.id) {
    current.value = null
    items.value = []
  }
  await loadDicts()
}

const itemVisible = ref(false)
const itemTitle = ref('')
const isItemEdit = ref(false)
const itemEditId = ref(0)
const emptyItemForm = {
  label: '',
  value: '',
  sort: 0,
  enabled: true,
  parent_id: 0,
  is_default: false,
  remark: '',
}
const itemForm = reactive({ ...emptyItemForm })
const parentOptions = computed(() => items.value.filter((i) => i.id !== itemEditId.value))

function openItemCreate() {
  if (!current.value) return
  isItemEdit.value = false
  itemTitle.value = '新增字典项'
  Object.assign(itemForm, emptyItemForm)
  itemEditId.value = 0
  itemVisible.value = true
}

function openItemEdit(item: DictDataItem) {
  isItemEdit.value = true
  itemTitle.value = '编辑字典项'
  Object.assign(itemForm, {
    label: item.label,
    value: item.value,
    sort: item.sort,
    enabled: item.status === 1,
    parent_id: item.parent_id || 0,
    is_default: item.is_default,
    remark: item.remark || '',
  })
  itemEditId.value = item.id
  itemVisible.value = true
}

async function submitItem() {
  if (!current.value) return
  if (!itemForm.label.trim() || !itemForm.value.trim()) {
    ElMessage.warning('标签与值不能为空')
    return
  }
  submitting.value = true
  try {
    const statusVal = itemForm.enabled ? 1 : 0
    const extra = {
      status: statusVal,
      sort: itemForm.sort,
      parent_id: itemForm.parent_id,
      is_default: itemForm.is_default,
      remark: itemForm.remark,
    }
    if (isItemEdit.value) {
      await updateDictItem(itemEditId.value, {
        label: itemForm.label,
        value: itemForm.value,
        ...extra,
      })
      ElMessage.success('更新成功')
    } else {
      await createDictItem({
        dict_code: current.value.code,
        label: itemForm.label.trim(),
        value: itemForm.value.trim(),
        ...extra,
      })
      ElMessage.success('创建成功')
    }
    invalidateDictOptions(current.value.code)
    itemVisible.value = false
    await loadItems()
  } finally {
    submitting.value = false
  }
}

async function onDeleteItem(item: DictDataItem) {
  try {
    await ElMessageBox.confirm(`确定删除字典项「${item.label}」吗？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  await deleteDictItem(item.id)
  if (current.value) invalidateDictOptions(current.value.code)
  ElMessage.success('删除成功')
  await loadItems()
}

onMounted(() => {
  void groupDict.load()
  void loadDicts()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.actions {
  display: flex;
  gap: 8px;
}
.title {
  font-weight: 600;
}
.pager {
  margin-top: 12px;
  justify-content: flex-end;
}
</style>
