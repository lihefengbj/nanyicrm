<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="角色名">
          <el-input v-model="query.name" placeholder="模糊搜索" clearable style="width: 180px" @keyup.enter="load" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="load">查询</el-button>
          <el-button v-if="store.hasPerm('system:role:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增角色
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column prop="name" label="角色名" width="160" />
        <el-table-column prop="code" label="标识" width="160" />
        <el-table-column prop="sort" label="排序" width="80" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="200" show-overflow-tooltip />
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('system:role:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('system:role:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" class="pager"
        @current-change="load" @size-change="load" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑角色' : '新增角色'" width="560px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="角色名" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="标识" prop="code">
          <el-input v-model="form.code" :disabled="editingCode === 'admin'" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="菜单权限">
          <el-tree ref="menuTreeRef" :data="menuOptions" show-checkbox node-key="id"
            :props="{ label: 'title', children: 'children' }" style="width: 100%; max-height: 280px; overflow: auto" />
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
import { nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Search, Plus } from '@element-plus/icons-vue'
import { listRoles, createRole, updateRole, deleteRole, menuTree } from '@/api/system'
import type { Menu, Role } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const loading = ref(false)
const rows = ref<Role[]>([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, name: '' })

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const editingCode = ref('')
const formRef = ref<FormInstance>()
const menuTreeRef = ref()
const form = reactive({ name: '', code: '', sort: 0, status: 1, remark: '' })

const formRules: FormRules = {
  name: [{ required: true, message: '请输入角色名', trigger: 'blur' }],
  code: [{ required: true, message: '请输入角色标识', trigger: 'blur' }],
}

const menuOptions = ref<Menu[]>([])

async function load() {
  loading.value = true
  try {
    const data = await listRoles(query)
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function openDialog(row?: Role) {
  editingId.value = row?.id ?? null
  editingCode.value = row?.code ?? ''
  form.name = row?.name ?? ''
  form.code = row?.code ?? ''
  form.sort = row?.sort ?? 0
  form.status = row?.status ?? 1
  form.remark = row?.remark ?? ''
  dialogVisible.value = true
  await nextTick()
  const checked = (row?.menus ?? []).map((m) => m.id)
  // Only check leaf nodes; parent nodes follow automatically.
  const leafIds = collectLeafIds(menuOptions.value, new Set(checked))
  menuTreeRef.value?.setCheckedKeys(leafIds)
}

function collectLeafIds(menus: Menu[], checked: Set<number>): number[] {
  const ids: number[] = []
  const walk = (list: Menu[]) => {
    for (const m of list) {
      if (m.children?.length) {
        walk(m.children)
      } else if (checked.has(m.id)) {
        ids.push(m.id)
      }
    }
  }
  walk(menus)
  return ids
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  const menuIds: number[] = [
    ...(menuTreeRef.value?.getCheckedKeys() ?? []),
    ...(menuTreeRef.value?.getHalfCheckedKeys() ?? []),
  ]
  saving.value = true
  try {
    if (editingId.value) {
      await updateRole(editingId.value, { ...form, menuIds })
      ElMessage.success('已更新')
    } else {
      await createRole({ ...form, menuIds })
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

async function onDelete(row: Role) {
  await ElMessageBox.confirm(`确认删除角色「${row.name}」？`, '提示', { type: 'warning' })
  await deleteRole(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(async () => {
  load()
  try {
    menuOptions.value = await menuTree()
  } catch {
    menuOptions.value = []
  }
})
</script>

<style scoped>
.pager {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
