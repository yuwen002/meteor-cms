<template>
  <div class="role-page">
    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="query" @submit.prevent>
        <el-form-item label="关键词">
          <el-input v-model="query.keyword" placeholder="名称/编码" clearable @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.is_active" placeholder="全部" clearable style="width: 120px">
            <el-option :value="true" label="启用" />
            <el-option :value="false" label="禁用" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">查询</el-button>
          <el-button @click="resetQuery">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never">
      <div class="toolbar">
        <el-button type="primary" @click="openCreate" v-if="canAdd">新增角色</el-button>
      </div>
      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="name" label="名称" />
        <el-table-column prop="code" label="编码" />
        <el-table-column prop="desc" label="描述" show-overflow-tooltip />
        <el-table-column label="数据范围" width="130">
          <template #default="{ row }">{{ dataScopeLabel(row.data_scope) }}</template>
        </el-table-column>
        <el-table-column label="系统内置" width="90">
          <template #default="{ row }">
            <el-tag v-if="row.is_system" type="warning">系统</el-tag>
            <span v-else>否</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'">{{ row.is_active ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="70" />
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openPerm(asRow(row))" v-if="canAssignPerm">分配权限</el-button>
            <el-button link type="primary" @click="openEdit(asRow(row))" v-if="canEdit">编辑</el-button>
            <el-button link type="danger" :disabled="row.is_system" @click="handleDelete(asRow(row))" v-if="canDelete">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination class="pager" v-model:current-page="query.page" v-model:page-size="query.page_size"
        :total="total" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next"
        @current-change="loadData" @size-change="loadData" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="编码" required>
          <el-input v-model="form.code" :disabled="isEdit && !!currentRow?.is_system" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.desc" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="数据范围">
          <el-select v-model="form.data_scope" style="width: 100%">
            <el-option v-for="o in dataScopeOptions" :key="o.value" :value="o.value" :label="o.label" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态"><el-switch v-model="form.is_active" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="permVisible" title="分配权限" width="460px">
      <el-tree ref="permTreeRef" :data="permissionTree" :props="{ label: 'name', children: 'children' }"
        node-key="id" show-checkbox default-expand-all :default-checked-keys="checkedKeys" />
      <template #footer>
        <el-button @click="permVisible = false">取消</el-button>
        <el-button type="primary" :loading="permSubmitting" @click="submitPerm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted, nextTick } from 'vue'
import { useUserStore } from '@/stores/user'

// 按钮级权限：种子中 Type=2 按钮与 Type=3 接口使用同一标识，授权一次即可同时生效
const userStore = useUserStore()
const canAdd = computed(() => userStore.hasPermission('system:role:add'))
const canEdit = computed(() => userStore.hasPermission('system:role:edit'))
const canDelete = computed(() => userStore.hasPermission('system:role:delete'))
const canAssignPerm = computed(() => userStore.hasPermission('system:role:perm_assign'))
import { ElMessage, ElMessageBox } from '@/utils/element'
import {
  getRoles, createRole, updateRole, deleteRole,
  getRolePermissions, assignRolePermissions,
} from '@/api/role'
import { getPermissionTree } from '@/api/permission'
import type { RoleItem, RoleForm } from '@/api/role'
import type { PermissionTreeItem } from '@/api/permission'

// el-table 作用域插槽的 row 为 DefaultRow，这里显式收敛为 RoleItem
const asRow = (row: unknown) => row as RoleItem

// 必须与 internal/pkg/datascope/scope.go 的 ScopeAll..ScopeCustom 保持一致
const dataScopeOptions = [
  { value: 1, label: '全部数据' },
  { value: 2, label: '本部门数据' },
  { value: 3, label: '本部门及子部门' },
  { value: 4, label: '仅本人数据' },
  { value: 5, label: '自定义部门' },
]
function dataScopeLabel(v: number) {
  return dataScopeOptions.find((o) => o.value === v)?.label || `未知(${v})`
}

const list = ref<RoleItem[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ keyword: '', is_active: undefined as boolean | undefined, page: 1, page_size: 20 })

async function loadData() {
  loading.value = true
  try {
    const res = await getRoles({ ...query })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}
function resetQuery() {
  query.keyword = ''
  query.is_active = undefined
  query.page = 1
  loadData()
}

const dialogVisible = ref(false)
const dialogTitle = ref('')
const isEdit = ref(false)
const currentRow = ref<RoleItem | null>(null)
const submitting = ref(false)
const emptyForm: RoleForm = { name: '', code: '', desc: '', data_scope: 1, is_active: true, sort: 0 }
const form = reactive<RoleForm>({ ...emptyForm })

function openCreate() {
  isEdit.value = false
  dialogTitle.value = '新增角色'
  Object.assign(form, emptyForm)
  currentRow.value = null
  dialogVisible.value = true
}
function openEdit(row: RoleItem) {
  isEdit.value = true
  dialogTitle.value = '编辑角色'
  currentRow.value = row
  Object.assign(form, {
    name: row.name, code: row.code, desc: row.desc,
    data_scope: row.data_scope, is_active: row.is_active, sort: row.sort,
  })
  dialogVisible.value = true
}
async function submit() {
  if (!form.name || !form.code) { ElMessage.warning('请填写名称和编码'); return }
  submitting.value = true
  try {
    if (isEdit.value && currentRow.value) {
      await updateRole(currentRow.value.id, { ...form })
      ElMessage.success('更新成功')
    } else {
      await createRole({ ...form })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadData()
  } finally {
    submitting.value = false
  }
}
async function handleDelete(row: RoleItem) {
  try {
    await ElMessageBox.confirm(`确认删除角色「${row.name}」？`, '提示', { type: 'warning' })
  } catch { return }
  await deleteRole(row.id)
  ElMessage.success('删除成功')
  loadData()
}

const permVisible = ref(false)
const permSubmitting = ref(false)
const permissionTree = ref<PermissionTreeItem[]>([])
const checkedKeys = ref<number[]>([])
const permTreeRef = ref()
const permRoleId = ref<number>(0)

async function openPerm(row: RoleItem) {
  permRoleId.value = row.id
  permVisible.value = true
  const [tree, perms] = await Promise.all([getPermissionTree(), getRolePermissions(row.id)])
  permissionTree.value = tree
  checkedKeys.value = perms.permission_ids
  await nextTick()
  permTreeRef.value?.setCheckedKeys(checkedKeys.value)
}
async function submitPerm() {
  const checked = (permTreeRef.value?.getCheckedKeys() || []) as number[]
  const halfChecked = (permTreeRef.value?.getHalfCheckedKeys() || []) as number[]
  // 三级权限树的半选父节点也必须提交，否则动态菜单缺少父级节点
  const keys = Array.from(new Set([...checked, ...halfChecked]))
  permSubmitting.value = true
  try {
    await assignRolePermissions(permRoleId.value, keys)
    ElMessage.success('权限已保存')
    permVisible.value = false
  } finally {
    permSubmitting.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.toolbar { margin-bottom: 12px; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
