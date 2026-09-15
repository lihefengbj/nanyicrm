<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="关键词">
          <el-input v-model="query.keyword" clearable placeholder="名称 / 编号" style="width: 180px" @keyup.enter="load(1)" />
        </el-form-item>
        <el-form-item label="客户">
          <el-select v-model="query.customerId" clearable filterable placeholder="全部客户" style="width: 180px">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" clearable placeholder="全部" style="width: 120px">
            <el-option v-for="(label, i) in statusLabels" :key="i" :label="label" :value="String(i + 1)" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load(1)">查询</el-button>
          <el-button v-if="store.hasPerm('crm:contract:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增合同
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="code" label="编号" width="140">
          <template #default="{ row }">{{ row.code || '-' }}</template>
        </el-table-column>
        <el-table-column prop="name" label="合同名称" min-width="160" />
        <el-table-column label="客户" min-width="130">
          <template #default="{ row }">{{ row.customer?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="关联商机" min-width="130">
          <template #default="{ row }">{{ row.opportunity?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="金额" width="110" align="right">
          <template #default="{ row }">{{ row.amount?.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="签约日期" width="110">
          <template #default="{ row }">{{ row.signDate?.slice(0, 10) || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 3 ? 'success' : row.status === 4 ? 'info' : 'primary'">
              {{ statusLabels[row.status - 1] || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="负责人" width="100">
          <template #default="{ row }">{{ row.owner?.nickname || row.owner?.username || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('crm:contract:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('crm:contract:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        layout="total, prev, pager, next" style="margin-top: 12px; justify-content: flex-end"
        @current-change="load()" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑合同' : '新增合同'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="合同名称" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="编号">
          <el-input v-model="form.code" placeholder="租户内唯一" />
        </el-form-item>
        <el-form-item label="客户" prop="customerId">
          <el-select v-model="form.customerId" filterable placeholder="请选择客户" style="width: 100%"
            @change="form.opportunityId = undefined">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="关联商机">
          <el-select v-model="form.opportunityId" clearable filterable placeholder="可选，关联后该商机记为赢单" style="width: 100%">
            <el-option v-for="op in opportunityOptions" :key="op.id" :label="op.name" :value="op.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="金额">
          <el-input-number v-model="form.amount" :min="0" :precision="2" style="width: 100%" />
        </el-form-item>
        <el-form-item label="签约日期">
          <el-date-picker v-model="form.signDate" type="date" style="width: 100%" value-format="YYYY-MM-DDT00:00:00Z" />
        </el-form-item>
        <el-form-item label="履行周期">
          <el-date-picker v-model="period" type="daterange" start-placeholder="开始" end-placeholder="结束"
            style="width: 100%" value-format="YYYY-MM-DDT00:00:00Z" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio v-for="(label, i) in statusLabels" :key="i" :value="i + 1">{{ label }}</el-radio>
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
defineOptions({ name: 'SalesContract' })
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listContracts, createContract, updateContract, deleteContract, listAllCustomers, listAllOpportunities } from '@/api/crm'
import type { Contract, Customer, Opportunity } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const statusLabels = ['草稿', '履行中', '已完成', '已作废']

const loading = ref(false)
const rows = ref<Contract[]>([])
const total = ref(0)
const customerOptions = ref<Customer[]>([])
const allOpportunities = ref<Opportunity[]>([])
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', customerId: undefined as number | undefined, status: '' })

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({
  name: '', code: '', customerId: undefined as number | undefined, opportunityId: undefined as number | undefined,
  amount: 0, signDate: undefined as string | undefined, status: 1, remark: '',
})
const period = ref<[string, string] | null>(null)

const formRules: FormRules = {
  name: [{ required: true, message: '请输入合同名称', trigger: 'blur' }],
  customerId: [{ required: true, message: '请选择客户', trigger: 'change' }],
}

const opportunityOptions = computed(() =>
  allOpportunities.value.filter((o) => !form.customerId || o.customerId === form.customerId),
)

async function load(page?: number) {
  if (page) query.pageNum = page
  loading.value = true
  try {
    const data = await listContracts({ ...query })
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

watch(() => query.customerId, async () => {
  allOpportunities.value = await listAllOpportunities(query.customerId)
})

function openDialog(row?: Contract) {
  editingId.value = row?.id ?? null
  form.name = row?.name ?? ''
  form.code = row?.code ?? ''
  form.customerId = row?.customerId ?? query.customerId
  form.opportunityId = row?.opportunityId ?? undefined
  form.amount = row?.amount ?? 0
  form.signDate = row?.signDate ?? undefined
  form.status = row?.status ?? 1
  form.remark = row?.remark ?? ''
  period.value = row?.startDate && row?.endDate ? [row.startDate, row.endDate] : null
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const payload = {
      ...form,
      customerId: form.customerId!,
      startDate: period.value?.[0],
      endDate: period.value?.[1],
    }
    if (editingId.value) {
      await updateContract(editingId.value, payload)
      ElMessage.success('已更新')
    } else {
      await createContract(payload)
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

async function onDelete(row: Contract) {
  await ElMessageBox.confirm(`确认删除合同「${row.name}」？`, '提示', { type: 'warning' })
  await deleteContract(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(async () => {
  customerOptions.value = await listAllCustomers()
  allOpportunities.value = await listAllOpportunities()
  load()
})
</script>
