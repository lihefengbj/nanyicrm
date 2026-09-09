<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item>
          <el-button v-if="store.hasPerm('system:dept:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增根部门
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" row-key="id" border default-expand-all>
        <el-table-column prop="name" label="部门名称" min-width="220" />
        <el-table-column prop="leader" label="负责人" width="140" />
        <el-table-column prop="sort" label="排序" width="80" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('system:dept:create')" link type="primary" @click="openDialog(undefined, row.id)">
              新增子部门
            </el-button>
            <el-button v-if="store.hasPerm('system:dept:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('system:dept:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑部门' : '新增部门'" width="480px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="上级部门">
          <el-tree-select v-model="form.parentId" :data="deptOptions" check-strictly :render-after-expand="false"
            :props="{ label: 'name', value: 'id', children: 'children' }" clearable placeholder="留空为根部门"
            style="width: 100%" />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="负责人">
          <el-input v-model="form.leader" />
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
import { Plus } from '@element-plus/icons-vue'
import { deptTree, createDept, updateDept, deleteDept } from '@/api/system'
import type { Dept } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const loading = ref(false)
const rows = ref<Dept[]>([])
const deptOptions = ref<Dept[]>([])

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ parentId: undefined as number | undefined, name: '', leader: '', sort: 0, status: 1 })

const formRules: FormRules = {
  name: [{ required: true, message: '请输入部门名称', trigger: 'blur' }],
}

async function load() {
  loading.value = true
  try {
    rows.value = await deptTree()
    deptOptions.value = rows.value
  } finally {
    loading.value = false
  }
}

function openDialog(row?: Dept, parentId?: number) {
  editingId.value = row?.id ?? null
  form.parentId = row?.parentId ?? parentId ?? undefined
  if (form.parentId === 0) form.parentId = undefined
  form.name = row?.name ?? ''
  form.leader = row?.leader ?? ''
  form.sort = row?.sort ?? 0
  form.status = row?.status ?? 1
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  const payload = { ...form, parentId: form.parentId ?? 0 }
  saving.value = true
  try {
    if (editingId.value) {
      await updateDept(editingId.value, payload)
      ElMessage.success('已更新')
    } else {
      await createDept(payload)
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

async function onDelete(row: Dept) {
  await ElMessageBox.confirm(`确认删除部门「${row.name}」？`, '提示', { type: 'warning' })
  await deleteDept(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
