<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="关键词">
          <el-input v-model="query.keyword" clearable placeholder="名称 / 类型" style="width: 180px" @keyup.enter="load(1)" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load(1)">查询</el-button>
          <el-button v-if="store.hasPerm('system:dict:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增字典
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="name" label="字典名称" min-width="140" />
        <el-table-column prop="type" label="类型标识" min-width="160" />
        <el-table-column label="字典项" width="90">
          <template #default="{ row }">{{ row.items?.length ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="140" show-overflow-tooltip />
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('system:dict:update')" link type="primary" @click="openItems(row)">字典项</el-button>
            <el-button v-if="store.hasPerm('system:dict:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('system:dict:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        layout="total, prev, pager, next" style="margin-top: 12px; justify-content: flex-end"
        @current-change="load()" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑字典' : '新增字典'" width="480px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="类型标识" prop="type">
          <el-input v-model="form.type" placeholder="如 customer_source" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="itemsVisible" :title="`字典项 - ${currentDict?.name ?? ''}`" size="480px">
      <el-form inline @submit.prevent>
        <el-form-item>
          <el-button type="primary" plain :icon="Plus" @click="openItemDialog()">新增字典项</el-button>
        </el-form-item>
      </el-form>
      <el-table :data="currentDict?.items ?? []" border size="small">
        <el-table-column prop="label" label="标签" min-width="100" />
        <el-table-column prop="value" label="值" min-width="80" />
        <el-table-column prop="sort" label="排序" width="60" />
        <el-table-column label="状态" width="70">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110">
          <template #default="{ row }">
            <el-button link type="primary" @click="openItemDialog(row)">编辑</el-button>
            <el-button link type="danger" @click="onDeleteItem(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-dialog v-model="itemDialogVisible" :title="editingItemId ? '编辑字典项' : '新增字典项'" width="420px" append-to-body destroy-on-close>
        <el-form :model="itemForm" label-width="70px">
          <el-form-item label="标签" required>
            <el-input v-model="itemForm.label" />
          </el-form-item>
          <el-form-item label="值" required>
            <el-input v-model="itemForm.value" />
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="itemForm.sort" :min="0" />
          </el-form-item>
          <el-form-item label="状态">
            <el-radio-group v-model="itemForm.status">
              <el-radio :value="1">启用</el-radio>
              <el-radio :value="0">停用</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="itemDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="onSaveItem">保存</el-button>
        </template>
      </el-dialog>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'SystemDict' })
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import {
  listDicts, createDict, updateDict, deleteDict,
  createDictItem, updateDictItem, deleteDictItem,
} from '@/api/system'
import type { Dict, DictItem } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const loading = ref(false)
const rows = ref<Dict[]>([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '' })

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ name: '', type: '', status: 1, remark: '' })

const formRules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  type: [{ required: true, message: '请输入类型标识', trigger: 'blur' }],
}

const itemsVisible = ref(false)
const currentDict = ref<Dict | null>(null)
const itemDialogVisible = ref(false)
const editingItemId = ref<number | null>(null)
const itemForm = reactive({ label: '', value: '', sort: 0, status: 1 })

async function load(page?: number) {
  if (page) query.pageNum = page
  loading.value = true
  try {
    const data = await listDicts(query)
    rows.value = data.records
    total.value = data.total
    if (currentDict.value) {
      currentDict.value = data.records.find((d) => d.id === currentDict.value!.id) ?? null
    }
  } finally {
    loading.value = false
  }
}

function openDialog(row?: Dict) {
  editingId.value = row?.id ?? null
  form.name = row?.name ?? ''
  form.type = row?.type ?? ''
  form.status = row?.status ?? 1
  form.remark = row?.remark ?? ''
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (editingId.value) {
      await updateDict(editingId.value, form)
      ElMessage.success('已更新')
    } else {
      await createDict(form)
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

async function onDelete(row: Dict) {
  await ElMessageBox.confirm(`确认删除字典「${row.name}」及其全部字典项？`, '提示', { type: 'warning' })
  await deleteDict(row.id)
  ElMessage.success('已删除')
  load()
}

function openItems(row: Dict) {
  currentDict.value = row
  itemsVisible.value = true
}

function openItemDialog(row?: DictItem) {
  editingItemId.value = row?.id ?? null
  itemForm.label = row?.label ?? ''
  itemForm.value = row?.value ?? ''
  itemForm.sort = row?.sort ?? 0
  itemForm.status = row?.status ?? 1
  itemDialogVisible.value = true
}

async function onSaveItem() {
  if (!itemForm.label || !itemForm.value) {
    ElMessage.warning('请填写标签和值')
    return
  }
  saving.value = true
  try {
    const payload = { ...itemForm, dictId: currentDict.value!.id }
    if (editingItemId.value) {
      await updateDictItem(editingItemId.value, payload)
    } else {
      await createDictItem(payload)
    }
    itemDialogVisible.value = false
    ElMessage.success('已保存')
    load()
  } catch {
    // interceptor shows the message
  } finally {
    saving.value = false
  }
}

async function onDeleteItem(row: DictItem) {
  await ElMessageBox.confirm(`确认删除字典项「${row.label}」？`, '提示', { type: 'warning' })
  await deleteDictItem(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
