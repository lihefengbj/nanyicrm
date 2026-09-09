<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="用户名">
          <el-input v-model="query.username" placeholder="模糊搜索" clearable style="width: 180px" @keyup.enter="load" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部" clearable style="width: 120px">
            <el-option label="启用" value="1" />
            <el-option label="停用" value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="load">查询</el-button>
          <el-button v-if="store.hasPerm('system:user:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增用户
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column prop="nickname" label="昵称" width="140" />
        <el-table-column label="部门" width="140">
          <template #default="{ row }">{{ row.dept?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="角色" min-width="160">
          <template #default="{ row }">
            <el-tag v-for="r in row.roles || []" :key="r.id" size="small" class="role-tag">{{ r.name }}</el-tag>
            <span v-if="!row.roles?.length">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="phone" label="手机号" width="130" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('system:user:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('system:user:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" class="pager"
        @current-change="load" @size-change="load" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑用户' : '新增用户'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" :disabled="!!editingId" />
        </el-form-item>
        <el-form-item v-if="!editingId" label="密码" prop="pwd">
          <el-input v-model="form.pwd" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item v-else label="重置密码">
          <el-input v-model="form.pwd" type="password" show-password placeholder="留空则不修改" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="form.nickname" />
        </el-form-item>
        <el-form-item label="部门">
          <el-tree-select v-model="form.deptId" :data="deptOptions" check-strictly :render-after-expand="false"
            :props="{ label: 'name', value: 'id', children: 'children' }" clearable style="width: 100%" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.roleIds" multiple style="width: 100%">
            <el-option v-for="r in roleOptions" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Plus } from '@element-plus/icons-vue'
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
  listAllRoles,
  deptTree,
} from '@/api/system'
import type { Dept, Role, User } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const loading = ref(false)
const rows = ref<User[]>([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, username: '', status: '' })

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({
  username: '',
  pwd: '',
  nickname: '',
  email: '',
  phone: '',
  deptId: undefined as number | undefined,
  status: 1,
  remark: '',
  roleIds: [] as number[],
})

const formRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 64, message: '长度 2-64', trigger: 'blur' },
  ],
  pwd: [{ required: true, min: 6, message: '密码至少 6 位', trigger: 'blur' }],
  nickname: [{ required: true, message: '请输入昵称', trigger: 'blur' }],
}

const roleOptions = ref<Role[]>([])
const deptOptions = ref<Dept[]>([])

async function load() {
  loading.value = true
  try {
    const data = await listUsers(query)
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  try {
    roleOptions.value = await listAllRoles()
  } catch {
    roleOptions.value = []
  }
  try {
    deptOptions.value = await deptTree()
  } catch {
    deptOptions.value = []
  }
}

function openDialog(row?: User) {
  editingId.value = row?.id ?? null
  form.username = row?.username ?? ''
  form.pwd = ''
  form.nickname = row?.nickname ?? ''
  form.email = row?.email ?? ''
  form.phone = row?.phone ?? ''
  form.deptId = row?.deptId ?? undefined
  form.status = row?.status ?? 1
  form.remark = row?.remark ?? ''
  form.roleIds = row?.roles?.map((r) => r.id) ?? []
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (editingId.value) {
      await updateUser(editingId.value, { ...form })
      ElMessage.success('已更新')
    } else {
      await createUser({ ...form })
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    load()
  } catch {
    // interceptor shows the message
  } finally {
    saving.value = false
  }
}

async function onDelete(row: User) {
  await ElMessageBox.confirm(`确认删除用户「${row.username}」？`, '提示', { type: 'warning' })
  await deleteUser(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(() => {
  load()
  loadOptions()
})
</script>

<style scoped>
.role-tag {
  margin-right: 4px;
}
.pager {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
