<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="客户名称">
          <el-input v-model="query.name" clearable placeholder="模糊搜索" style="width: 180px" @keyup.enter="load(1)" />
        </el-form-item>
        <el-form-item label="等级">
          <el-select v-model="query.level" clearable placeholder="全部" style="width: 110px">
            <el-option label="A" value="A" />
            <el-option label="B" value="B" />
            <el-option label="C" value="C" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" clearable placeholder="全部" style="width: 120px">
            <el-option label="跟进中" value="1" />
            <el-option label="已成交" value="2" />
            <el-option label="已流失" value="3" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="isPrivileged" label="租户">
          <el-select v-model="query.tenantId" clearable placeholder="全部" style="width: 160px">
            <el-option v-for="tenant in tenantOptions" :key="tenant.id" :label="tenant.name" :value="tenant.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="onlyMine">只看我负责的</el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load(1)">查询</el-button>
          <el-button v-if="store.hasPerm('crm:customer:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增客户
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="name" label="客户名称" min-width="160" />
        <el-table-column v-if="isPrivileged" prop="tenantId" label="租户ID" width="90" />
        <el-table-column prop="phone" label="电话" width="130" />
        <el-table-column prop="source" label="来源" width="100" />
        <el-table-column prop="industry" label="行业" width="120" />
        <el-table-column label="等级" width="80">
          <template #default="{ row }">
            <el-tag v-if="row.level" :type="row.level === 'A' ? 'danger' : row.level === 'B' ? 'warning' : 'info'">
              {{ row.level }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="归属人" width="110">
          <template #default="{ row }">{{ row.owner?.nickname || row.owner?.username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="170" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('crm:customer:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button link type="primary" @click="goFollow(row)">跟进</el-button>
            <el-button v-if="store.hasPerm('crm:customer:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        layout="total, prev, pager, next" style="margin-top: 12px; justify-content: flex-end"
        @current-change="load()" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑客户' : '新增客户'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item v-if="isPrivileged" label="所属租户" prop="tenantId">
          <el-select v-model="form.tenantId" placeholder="请选择租户" style="width: 100%">
            <el-option v-for="tenant in tenantOptions" :key="tenant.id" :label="tenant.name" :value="tenant.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="客户名称" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="电话">
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-form-item label="来源">
          <el-input v-model="form.source" placeholder="如：广告 / 转介绍 / 自拓" />
        </el-form-item>
        <el-form-item label="行业">
          <el-input v-model="form.industry" />
        </el-form-item>
        <el-form-item label="等级">
          <el-radio-group v-model="form.level">
            <el-radio value="A">A</el-radio>
            <el-radio value="B">B</el-radio>
            <el-radio value="C">C</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">跟进中</el-radio>
            <el-radio :value="2">已成交</el-radio>
            <el-radio :value="3">已流失</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="地址">
          <el-input v-model="form.address" />
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
defineOptions({ name: 'CrmCustomer' })
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { listCustomers, createCustomer, updateCustomer, deleteCustomer } from '@/api/crm'
import { listAllTenants } from '@/api/system'
import type { Customer, Tenant } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()
const isPrivileged = computed(() => store.profile?.isPrivileged ?? false)
const tenantOptions = ref<Tenant[]>([])

const loading = ref(false)
const rows = ref<Customer[]>([])
const total = ref(0)
const onlyMine = ref(false)
const query = reactive({ pageNum: 1, pageSize: 10, name: '', status: '', level: '', tenantId: undefined as number | undefined })

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ tenantId: undefined as number | undefined, name: '', phone: '', source: '', industry: '', level: 'B', status: 1, address: '', remark: '' })

const formRules: FormRules = {
  name: [{ required: true, message: '请输入客户名称', trigger: 'blur' }],
  tenantId: [{
    validator: (_rule: unknown, value: number | undefined, callback: (error?: Error) => void) => {
      if (isPrivileged.value && !value) callback(new Error('请选择所属租户'))
      else callback()
    },
    trigger: 'change',
  }],
}

function statusText(s: number) {
  return s === 2 ? '已成交' : s === 3 ? '已流失' : '跟进中'
}
function statusTag(s: number) {
  return s === 2 ? 'success' : s === 3 ? 'info' : 'primary'
}

async function load(page?: number) {
  if (page) query.pageNum = page
  loading.value = true
  try {
    const data = await listCustomers({ ...query, mine: onlyMine.value ? '1' : '' })
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

watch(onlyMine, () => load(1))

function openDialog(row?: Customer) {
  editingId.value = row?.id ?? null
  form.tenantId = row?.tenantId || undefined
  form.name = row?.name ?? ''
  form.phone = row?.phone ?? ''
  form.source = row?.source ?? ''
  form.industry = row?.industry ?? ''
  form.level = row?.level || 'B'
  form.status = row?.status ?? 1
  form.address = row?.address ?? ''
  form.remark = row?.remark ?? ''
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (editingId.value) {
      await updateCustomer(editingId.value, form)
      ElMessage.success('已更新')
    } else {
      await createCustomer(form)
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

async function onDelete(row: Customer) {
  await ElMessageBox.confirm(`确认删除客户「${row.name}」？其联系人和跟进记录将一并删除。`, '提示', { type: 'warning' })
  await deleteCustomer(row.id)
  ElMessage.success('已删除')
  load()
}

function goFollow(row: Customer) {
  router.push({ path: '/crm/follow', query: { customerId: row.id } })
}

onMounted(async () => {
  if (isPrivileged.value) {
    tenantOptions.value = await listAllTenants().catch(() => [])
  }
  load()
})
</script>
