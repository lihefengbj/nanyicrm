<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="姓名">
          <el-input v-model="query.name" clearable placeholder="模糊搜索" style="width: 160px" @keyup.enter="load(1)" />
        </el-form-item>
        <el-form-item label="所属客户">
          <el-select v-model="query.customerId" clearable filterable placeholder="全部客户" style="width: 200px">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load(1)">查询</el-button>
          <el-button v-if="store.hasPerm('crm:contact:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增联系人
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="name" label="姓名" width="120" />
        <el-table-column label="所属客户" min-width="150">
          <template #default="{ row }">{{ row.customer?.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="position" label="职务" width="110" />
        <el-table-column prop="phone" label="电话" width="130" />
        <el-table-column prop="email" label="邮箱" min-width="150" />
        <el-table-column label="首要" width="70">
          <template #default="{ row }">
            <el-tag v-if="row.isPrimary === 1" type="success">是</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="170" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('crm:contact:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('crm:contact:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        layout="total, prev, pager, next" style="margin-top: 12px; justify-content: flex-end"
        @current-change="load()" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑联系人' : '新增联系人'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="所属客户" prop="customerId">
          <el-select v-model="form.customerId" filterable placeholder="请选择客户" style="width: 100%">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="姓名" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="职务">
          <el-input v-model="form.position" />
        </el-form-item>
        <el-form-item label="电话">
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item label="首要联系人">
          <el-switch v-model="form.isPrimary" :active-value="1" :inactive-value="0" />
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
defineOptions({ name: 'CrmContact' })
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listContacts, createContact, updateContact, deleteContact, listAllCustomers } from '@/api/crm'
import type { Contact, Customer } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const loading = ref(false)
const rows = ref<Contact[]>([])
const total = ref(0)
const customerOptions = ref<Customer[]>([])
const query = reactive({ pageNum: 1, pageSize: 10, name: '', customerId: undefined as number | undefined })

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ customerId: undefined as number | undefined, name: '', position: '', phone: '', email: '', isPrimary: 0, remark: '' })

const formRules: FormRules = {
  customerId: [{ required: true, message: '请选择客户', trigger: 'change' }],
  name: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  email: [{ type: 'email', message: '邮箱格式不正确', trigger: 'blur' }],
}

async function load(page?: number) {
  if (page) query.pageNum = page
  loading.value = true
  try {
    const data = await listContacts({ ...query })
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function openDialog(row?: Contact) {
  editingId.value = row?.id ?? null
  form.customerId = row?.customerId ?? query.customerId
  form.name = row?.name ?? ''
  form.position = row?.position ?? ''
  form.phone = row?.phone ?? ''
  form.email = row?.email ?? ''
  form.isPrimary = row?.isPrimary ?? 0
  form.remark = row?.remark ?? ''
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const payload = { ...form, customerId: form.customerId! }
    if (editingId.value) {
      await updateContact(editingId.value, payload)
      ElMessage.success('已更新')
    } else {
      await createContact(payload)
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

async function onDelete(row: Contact) {
  await ElMessageBox.confirm(`确认删除联系人「${row.name}」？`, '提示', { type: 'warning' })
  await deleteContact(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(async () => {
  customerOptions.value = await listAllCustomers()
  load()
})
</script>
