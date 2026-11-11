<template>
  <div class="admin-user">
    <el-card>
      <el-form :inline="true" :model="query" class="search-bar">
        <el-form-item label="关键词">
          <el-input v-model="query.keyword" placeholder="用户名/昵称/邮箱" clearable @keyup.enter="loadList" />
        </el-form-item>
        <el-form-item label="部门">
          <el-select v-model="query.dept_id" placeholder="全部部门" clearable style="width: 180px" @change="loadList">
            <el-option v-for="d in deptFilterOptions" :key="d.id" :label="d.label" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadList">查询</el-button>
          <el-button @click="resetQuery">重置</el-button>
        </el-form-item>
        <el-form-item style="margin-left: auto">
          <el-button type="success" @click="openCreate" v-if="canAdd">新增管理员</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="头像" width="70">
          <template #default="{ row }">
            <el-avatar :size="32" :src="row.avatar || ''">{{ row.username?.charAt(0) }}</el-avatar>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" />
        <el-table-column prop="nickname" label="昵称" />
        <el-table-column prop="email" label="邮箱" />
        <el-table-column prop="phone" label="手机号" />
        <el-table-column prop="dept_name" label="部门" />
        <el-table-column label="超管" width="70">
          <template #default="{ row }">
            <el-tag v-if="row.isSuper" type="danger">是</el-tag>
            <span v-else>否</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.isActive ? 'success' : 'info'">{{ row.isActive ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="350" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(asRow(row))" v-if="canEdit">编辑</el-button>
            <el-button link type="warning" @click="openResetPwd(asRow(row))" v-if="canResetPwd">重置密码</el-button>
            <el-button link type="info" @click="openAssignRole(asRow(row))" v-if="canAssignRole">分配角色</el-button>
            <el-button link type="info" @click="openBindDept(asRow(row))" v-if="canBindDept">绑定部门</el-button>
            <el-button v-if="row.isActive && canDisable" link type="warning" @click="doDisable(asRow(row))">禁用</el-button>
            <el-button v-else-if="canEnable" link type="success" @click="doEnable(asRow(row))">启用</el-button>
            <el-button link type="danger" @click="doDelete(asRow(row))" v-if="canDelete">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        class="pager"
        background
        layout="total, prev, pager, next"
        :total="total"
        :current-page="query.page"
        :page-size="query.page_size"
        @current-change="(p: number) => { query.page = p; loadList() }"
      />
    </el-card>

    <!-- 新增 / 编辑 -->
    <el-dialog :title="dialogTitle" v-model="dialogVisible" width="480px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" :disabled="isEdit" />
        </el-form-item>
        <el-form-item v-if="!isEdit" label="密码">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="form.nickname" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.is_active" />
        </el-form-item>
        <el-form-item label="超级管理员">
          <el-switch v-model="form.is_super" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">确定</el-button>
      </template>
    </el-dialog>

    <!-- 重置密码 -->
    <el-dialog title="重置密码" v-model="resetVisible" width="420px">
      <el-form label-width="90px">
        <el-form-item label="新密码">
          <el-input v-model="resetPwd" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitResetPwd">确定</el-button>
      </template>
    </el-dialog>

    <!-- 分配角色 -->
    <el-dialog title="分配角色" v-model="roleVisible" width="420px">
      <el-select v-model="roleIds" multiple placeholder="请选择角色" style="width: 100%">
        <el-option v-for="r in roleOptions" :key="r.id" :label="r.name" :value="r.id" />
      </el-select>
      <template #footer>
        <el-button @click="roleVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitRoles">确定</el-button>
      </template>
    </el-dialog>

    <!-- 绑定部门：后端要求部门必须存在，故不提供「清空部门」选项 -->
    <el-dialog title="绑定部门" v-model="deptVisible" width="420px">
      <el-select v-model="bindDeptId" placeholder="请选择部门" style="width: 100%">
        <el-option v-for="d in deptOptions" :key="d.id" :label="d.label" :value="d.id" />
      </el-select>
      <template #footer>
        <el-button @click="deptVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitBindDept">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useUserStore } from '@/stores/user'

// 按钮级权限：种子中 Type=2 按钮与 Type=3 接口使用同一标识，授权一次即可同时生效
const userStore = useUserStore()
const canAdd = computed(() => userStore.hasPermission('system:admin_user:add'))
const canEdit = computed(() => userStore.hasPermission('system:admin_user:edit'))
const canDelete = computed(() => userStore.hasPermission('system:admin_user:delete'))
const canDisable = computed(() => userStore.hasPermission('system:admin_user:disable'))
const canEnable = computed(() => userStore.hasPermission('system:admin_user:enable'))
const canResetPwd = computed(() => userStore.hasPermission('system:admin_user:reset_pwd'))
const canAssignRole = computed(() => userStore.hasPermission('system:admin_user:role_assign'))
const canBindDept = computed(() => userStore.hasPermission('system:admin_user:bind_dept'))
import { ElMessage, ElMessageBox } from '@/utils/element'
import {
  getAdminList,
  getAdminDetail,
  createAdmin,
  updateAdmin,
  deleteAdmin,
  disableAdmin,
  enableAdmin,
  resetAdminPassword,
  getAdminRoles,
  assignAdminRoles,
  bindAdminDepartment,
} from '@/api/user'
import { getRoles } from '@/api/role'
import { getDeptTree } from '@/api/dept'
import type { DeptTreeItem } from '@/api/dept'

// el-table 作用域插槽的 row 为 DefaultRow，这里显式收敛为 AdminRow
const asRow = (row: unknown) => row as AdminRow

interface AdminRow {
  id: number
  username: string
  nickname: string
  email: string
  phone: string
  avatar: string
  isSuper: boolean
  isActive: boolean
  dept_id?: number
  dept_name?: string
}

const loading = ref(false)
const submitting = ref(false)
const list = ref<AdminRow[]>([])
const total = ref(0)
const query = reactive({
  page: 1,
  page_size: 10,
  keyword: '',
  dept_id: undefined as number | undefined,
})

const dialogVisible = ref(false)
const isEdit = ref(false)
const dialogTitle = ref('')
const editingId = ref<number>()
const form = reactive({
  username: '',
  password: '',
  nickname: '',
  email: '',
  phone: '',
  is_active: true,
  is_super: false,
})

const resetVisible = ref(false)
const resetPwd = ref('')
const resetId = ref<number>()

const roleVisible = ref(false)
const roleIds = ref<number[]>([])
const roleOptions = ref<{ id: number; name: string }[]>([])
const roleAdminId = ref<number>()

async function loadList() {
  loading.value = true
  try {
    const res = await getAdminList(query)
    list.value = res.list || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.keyword = ''
  query.dept_id = undefined
  query.page = 1
  loadList()
}

function openCreate() {
  isEdit.value = false
  dialogTitle.value = '新增管理员'
  Object.assign(form, {
    username: '',
    password: '',
    nickname: '',
    email: '',
    phone: '',
    is_active: true,
    is_super: false,
  })
  dialogVisible.value = true
}

function openEdit(row: AdminRow) {
  isEdit.value = true
  dialogTitle.value = '编辑管理员'
  editingId.value = row.id
  Object.assign(form, {
    username: row.username,
    password: '',
    nickname: row.nickname,
    email: row.email,
    phone: row.phone,
    is_active: row.isActive,
    is_super: row.isSuper,
  })
  dialogVisible.value = true
}

async function submitForm() {
  submitting.value = true
  try {
    if (isEdit.value && editingId.value) {
      await updateAdmin(editingId.value, { ...form })
      ElMessage.success('更新成功')
    } else {
      await createAdmin({ ...form })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadList()
  } finally {
    submitting.value = false
  }
}

function doDisable(row: AdminRow) {
  ElMessageBox.confirm(`确认禁用 ${row.username}？`, '提示', { type: 'warning' }).then(async () => {
    await disableAdmin(row.id)
    ElMessage.success('已禁用')
    loadList()
  })
}

function doEnable(row: AdminRow) {
  ElMessageBox.confirm(`确认启用 ${row.username}？`, '提示', { type: 'warning' }).then(async () => {
    await enableAdmin(row.id)
    ElMessage.success('已启用')
    loadList()
  })
}

function doDelete(row: AdminRow) {
  ElMessageBox.confirm(`确认删除 ${row.username}？`, '提示', { type: 'warning' }).then(async () => {
    await deleteAdmin(row.id)
    ElMessage.success('已删除')
    loadList()
  })
}

function openResetPwd(row: AdminRow) {
  resetId.value = row.id
  resetPwd.value = ''
  resetVisible.value = true
}

async function submitResetPwd() {
  if (!resetId.value) return
  submitting.value = true
  try {
    await resetAdminPassword(resetId.value, resetPwd.value)
    ElMessage.success('密码已重置')
    resetVisible.value = false
  } finally {
    submitting.value = false
  }
}

async function openAssignRole(row: AdminRow) {
  roleAdminId.value = row.id
  const [roles, current] = await Promise.all([getRoles({ page: 1, page_size: 100 }), getAdminRoles(row.id)])
  roleOptions.value = roles.list || []
  roleIds.value = (current.list || []).map((r: { id: number }) => r.id)
  roleVisible.value = true
}

async function submitRoles() {
  if (!roleAdminId.value) return
  submitting.value = true
  try {
    await assignAdminRoles(roleAdminId.value, roleIds.value)
    ElMessage.success('角色已更新')
    roleVisible.value = false
  } finally {
    submitting.value = false
  }
}

const deptVisible = ref(false)
const deptTree = ref<DeptTreeItem[]>([])
const bindDeptId = ref<number>()
const deptAdminId = ref<number>()

// 部门树铺平：绑定弹窗用带缩进的层级名，查询栏用不带缩进的纯名称
const deptOptions = computed(() => flattenDeptTree(deptTree.value))
const deptFilterOptions = computed(() => flattenDeptTree(deptTree.value, 0, false))
function flattenDeptTree(nodes: DeptTreeItem[], depth = 0, indent = true): { id: number; label: string }[] {
  const out: { id: number; label: string }[] = []
  for (const n of nodes) {
    out.push({ id: n.id, label: `${indent ? '　'.repeat(depth) : ''}${n.name}` })
    if (n.children?.length) out.push(...flattenDeptTree(n.children, depth + 1, indent))
  }
  return out
}

async function loadDeptTree() {
  if (deptTree.value.length > 0) return
  deptTree.value = await getDeptTree()
}

async function openBindDept(row: AdminRow) {
  deptAdminId.value = row.id
  const [tree, detail] = await Promise.all([getDeptTree(), getAdminDetail(row.id)])
  deptTree.value = tree
  bindDeptId.value = detail.dept_id || undefined
  deptVisible.value = true
}

async function submitBindDept() {
  if (!deptAdminId.value || !bindDeptId.value) { ElMessage.warning('请选择部门'); return }
  submitting.value = true
  try {
    await bindAdminDepartment(deptAdminId.value, bindDeptId.value)
    ElMessage.success('部门已绑定')
    deptVisible.value = false
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadList()
  void loadDeptTree()
})
</script>

<style scoped>
.search-bar {
  margin-bottom: 12px;
}
.pager {
  margin-top: 12px;
  justify-content: flex-end;
}
</style>
