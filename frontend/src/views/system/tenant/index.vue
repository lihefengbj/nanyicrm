<template>
  <div>
    <el-form inline>
      <el-form-item label="关键词">
        <el-input v-model="query.keyword" placeholder="编码 / 名称" clearable @keyup.enter="load" />
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="query.status" placeholder="全部" clearable style="width: 120px">
          <el-option label="正常" value="1" />
          <el-option label="停用" value="0" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="load">查询</el-button>
        <el-button type="primary" plain @click="openCreate">新增租户</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="rows" v-loading="loading" border>
      <el-table-column prop="code" label="租户编码" width="140" />
      <el-table-column prop="name" label="租户名称" min-width="160" />
      <el-table-column prop="contact" label="联系人" width="110" />
      <el-table-column prop="phone" label="联系电话" width="130" />
      <el-table-column label="过期时间" width="130">
        <template #default="{ row }">
          <span v-if="row.expireAt">{{ formatDate(row.expireAt) }}</span>
          <el-tag v-else type="success" size="small">永久</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '正常' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="140" show-overflow-tooltip />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-popconfirm title="确定删除该租户？有数据时将被拒绝" @confirm="onDelete(row)">
            <template #reference>
              <el-button link type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="query.pageNum"
      v-model:page-size="query.pageSize"
      :total="total"
      layout="total, prev, pager, next"
      style="margin-top: 12px; justify-content: flex-end"
      @current-change="load"
    />

    <el-dialog v-model="dialog.visible" :title="dialog.isEdit ? '编辑租户' : '新增租户'" width="520px">
      <el-form ref="formRef" :model="dialog.form" :rules="rules" label-width="90px">
        <el-form-item label="租户编码" prop="code">
          <el-input v-model="dialog.form.code" placeholder="如 acme" :disabled="dialog.isEdit" />
        </el-form-item>
        <el-form-item label="租户名称" prop="name">
          <el-input v-model="dialog.form.name" />
        </el-form-item>
        <el-form-item label="联系人">
          <el-input v-model="dialog.form.contact" />
        </el-form-item>
        <el-form-item label="联系电话">
          <el-input v-model="dialog.form.phone" />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="dialog.form.expireAt"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="留空表示永久"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="dialog.form.status">
            <el-radio :value="1">正常</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="dialog.form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="dialog.saving" @click="onSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'SystemTenant' })
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  listTenants,
  createTenant,
  updateTenant,
  deleteTenant,
} from '@/api/system'
import type { Tenant, TenantSavePayload } from '@/types/api'

const loading = ref(false)
const rows = ref<Tenant[]>([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '' })

async function load() {
  loading.value = true
  try {
    const res = await listTenants(query)
    rows.value = res.records
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function formatDate(iso: string) {
  return iso.slice(0, 10)
}

const formRef = ref<FormInstance>()
const dialog = reactive({
  visible: false,
  isEdit: false,
  saving: false,
  editId: 0,
  form: { code: '', name: '', contact: '', phone: '', expireAt: '', status: 1, remark: '' } as TenantSavePayload & { expireAt: string },
})

const rules: FormRules = {
  code: [
    { required: true, message: '请输入租户编码', trigger: 'blur' },
    { min: 2, max: 64, message: '长度 2-64', trigger: 'blur' },
  ],
  name: [{ required: true, message: '请输入租户名称', trigger: 'blur' }],
}

function openCreate() {
  dialog.isEdit = false
  dialog.editId = 0
  dialog.form = { code: '', name: '', contact: '', phone: '', expireAt: '', status: 1, remark: '' }
  dialog.visible = true
}

function openEdit(row: Tenant) {
  dialog.isEdit = true
  dialog.editId = row.id
  dialog.form = {
    code: row.code,
    name: row.name,
    contact: row.contact,
    phone: row.phone,
    expireAt: row.expireAt ? row.expireAt.slice(0, 10) : '',
    status: row.status,
    remark: row.remark,
  }
  dialog.visible = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  dialog.saving = true
  try {
    const payload = { ...dialog.form }
    const body: TenantSavePayload = { ...payload, expireAt: payload.expireAt || undefined }
    if (dialog.isEdit) {
      await updateTenant(dialog.editId, body)
    } else {
      await createTenant(body)
    }
    ElMessage.success('保存成功')
    dialog.visible = false
    load()
  } catch {
    // error toast handled by interceptor
  } finally {
    dialog.saving = false
  }
}

async function onDelete(row: Tenant) {
  try {
    await deleteTenant(row.id)
    ElMessage.success('删除成功')
    load()
  } catch {
    // error toast handled by interceptor
  }
}

onMounted(load)
</script>

