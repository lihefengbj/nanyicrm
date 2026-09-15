<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="商机名称">
          <el-input v-model="query.name" clearable placeholder="模糊搜索" style="width: 180px" @keyup.enter="load(1)" />
        </el-form-item>
        <el-form-item label="客户">
          <el-select v-model="query.customerId" clearable filterable placeholder="全部客户" style="width: 180px">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="阶段">
          <el-select v-model="query.stage" clearable placeholder="全部" style="width: 130px">
            <el-option v-for="(label, i) in stageLabels" :key="i" :label="label" :value="String(i + 1)" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="onlyMine">只看我负责的</el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load(1)">查询</el-button>
          <el-button v-if="store.hasPerm('crm:opportunity:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增商机
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="name" label="商机名称" min-width="160" />
        <el-table-column label="客户" min-width="140">
          <template #default="{ row }">{{ row.customer?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="阶段" width="110">
          <template #default="{ row }">
            <el-tag :type="row.stage === 5 ? 'success' : row.stage === 6 ? 'info' : 'warning'">
              {{ stageLabels[row.stage - 1] || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="预计金额" width="120" align="right">
          <template #default="{ row }">{{ row.amount?.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="预计成交" width="120">
          <template #default="{ row }">{{ row.expectDate?.slice(0, 10) || '-' }}</template>
        </el-table-column>
        <el-table-column label="归属人" width="110">
          <template #default="{ row }">{{ row.owner?.nickname || row.owner?.username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="170" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('crm:opportunity:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('crm:opportunity:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        layout="total, prev, pager, next" style="margin-top: 12px; justify-content: flex-end"
        @current-change="load()" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑商机' : '新增商机'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="客户" prop="customerId">
          <el-select v-model="form.customerId" filterable placeholder="请选择客户" style="width: 100%">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="商机名称" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="阶段">
          <el-select v-model="form.stage" style="width: 100%">
            <el-option v-for="(label, i) in stageLabels" :key="i" :label="label" :value="i + 1" />
          </el-select>
        </el-form-item>
        <el-form-item label="预计金额">
          <el-input-number v-model="form.amount" :min="0" :precision="2" style="width: 100%" />
        </el-form-item>
        <el-form-item label="预计成交">
          <el-date-picker v-model="form.expectDate" type="date" placeholder="可选" style="width: 100%"
            value-format="YYYY-MM-DDT00:00:00Z" />
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
defineOptions({ name: 'SalesOpportunity' })
import { onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listOpportunities, createOpportunity, updateOpportunity, deleteOpportunity, listAllCustomers } from '@/api/crm'
import type { Customer, Opportunity } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const stageLabels = ['初步接触', '需求确认', '方案报价', '商务谈判', '赢单', '输单']

const loading = ref(false)
const rows = ref<Opportunity[]>([])
const total = ref(0)
const onlyMine = ref(false)
const customerOptions = ref<Customer[]>([])
const query = reactive({ pageNum: 1, pageSize: 10, name: '', customerId: undefined as number | undefined, stage: '' })

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ customerId: undefined as number | undefined, name: '', stage: 1, amount: 0, expectDate: undefined as string | undefined, remark: '' })

const formRules: FormRules = {
  customerId: [{ required: true, message: '请选择客户', trigger: 'change' }],
  name: [{ required: true, message: '请输入商机名称', trigger: 'blur' }],
}

async function load(page?: number) {
  if (page) query.pageNum = page
  loading.value = true
  try {
    const data = await listOpportunities({ ...query, mine: onlyMine.value ? '1' : '' })
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

watch(onlyMine, () => load(1))

function openDialog(row?: Opportunity) {
  editingId.value = row?.id ?? null
  form.customerId = row?.customerId ?? query.customerId
  form.name = row?.name ?? ''
  form.stage = row?.stage ?? 1
  form.amount = row?.amount ?? 0
  form.expectDate = row?.expectDate ?? undefined
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
      await updateOpportunity(editingId.value, payload)
      ElMessage.success('已更新')
    } else {
      await createOpportunity(payload)
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

async function onDelete(row: Opportunity) {
  await ElMessageBox.confirm(`确认删除商机「${row.name}」？关联合同将解除绑定。`, '提示', { type: 'warning' })
  await deleteOpportunity(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(async () => {
  customerOptions.value = await listAllCustomers()
  load()
})
</script>
