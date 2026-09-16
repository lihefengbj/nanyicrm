<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="客户">
          <el-select v-model="query.customerId" clearable filterable placeholder="全部客户" style="width: 200px">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="方式">
          <el-select v-model="query.type" clearable placeholder="全部" style="width: 110px">
            <el-option label="电话" value="1" />
            <el-option label="拜访" value="2" />
            <el-option label="会议" value="3" />
            <el-option label="其他" value="4" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="onlyMine">只看我写的</el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load(1)">查询</el-button>
          <el-button v-if="store.hasPerm('crm:follow:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            写跟进
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column label="客户" min-width="140">
          <template #default="{ row }">{{ row.customer?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="方式" width="80">
          <template #default="{ row }">
            <el-tag>{{ typeText(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="content" label="跟进内容" min-width="220" show-overflow-tooltip />
        <el-table-column label="联系人" width="100">
          <template #default="{ row }">{{ row.contact?.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="nextAt" label="下次跟进" width="160">
          <template #default="{ row }">{{ row.nextAt || '-' }}</template>
        </el-table-column>
        <el-table-column prop="creator" label="记录人" width="100" />
        <el-table-column prop="createdAt" label="创建时间" width="170" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('crm:follow:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('crm:follow:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        layout="total, prev, pager, next" style="margin-top: 12px; justify-content: flex-end"
        @current-change="load()" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑跟进' : '写跟进'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="客户" prop="customerId">
          <el-select v-model="form.customerId" filterable placeholder="请选择客户" style="width: 100%" @change="form.contactId = undefined">
            <el-option v-for="cu in customerOptions" :key="cu.id" :label="cu.name" :value="cu.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="联系人">
          <el-select v-model="form.contactId" clearable placeholder="可选" style="width: 100%">
            <el-option v-for="ct in contactOptions" :key="ct.id" :label="ct.name" :value="ct.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="方式">
          <el-radio-group v-model="form.type">
            <el-radio :value="1">电话</el-radio>
            <el-radio :value="2">拜访</el-radio>
            <el-radio :value="3">会议</el-radio>
            <el-radio :value="4">其他</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="内容" prop="content">
          <el-input v-model="form.content" type="textarea" :rows="3" maxlength="1024" />
        </el-form-item>
        <el-form-item label="下次跟进">
          <el-date-picker v-model="form.nextAt" type="datetime" placeholder="可选" style="width: 100%"
            value-format="YYYY-MM-DDTHH:mm:ssZ" />
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
defineOptions({ name: 'CrmFollow' })
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import { listFollowUps, createFollowUp, updateFollowUp, deleteFollowUp, listAllCustomers, listContacts } from '@/api/crm'
import type { Contact, Customer, FollowUp } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const route = useRoute()

const loading = ref(false)
const rows = ref<FollowUp[]>([])
const total = ref(0)
const onlyMine = ref(false)
const customerOptions = ref<Customer[]>([])
const allContacts = ref<Contact[]>([])
const query = reactive({
  pageNum: 1,
  pageSize: 10,
  customerId: route.query.customerId ? Number(route.query.customerId) : undefined as number | undefined,
  type: '',
})

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ customerId: undefined as number | undefined, contactId: undefined as number | undefined, type: 1, content: '', nextAt: undefined as string | undefined })

const formRules: FormRules = {
  customerId: [{ required: true, message: '请选择客户', trigger: 'change' }],
  content: [{ required: true, message: '请输入跟进内容', trigger: 'blur' }],
}

const contactOptions = computed(() => allContacts.value.filter((c) => c.customerId === form.customerId))

function typeText(t: number) {
  return t === 2 ? '拜访' : t === 3 ? '会议' : t === 4 ? '其他' : '电话'
}

async function load(page?: number) {
  if (page) query.pageNum = page
  loading.value = true
  try {
    const data = await listFollowUps({ ...query, mine: onlyMine.value ? '1' : '' })
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

watch(onlyMine, () => load(1))
watch(
  () => route.query.customerId,
  (value) => {
    query.customerId = value ? Number(value) : undefined
    load(1)
  },
)

function openDialog(row?: FollowUp) {
  editingId.value = row?.id ?? null
  form.customerId = row?.customerId ?? query.customerId
  form.contactId = row?.contactId ?? undefined
  form.type = row?.type ?? 1
  form.content = row?.content ?? ''
  form.nextAt = row?.nextAt ?? undefined
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const payload = { ...form, customerId: form.customerId! }
    if (editingId.value) {
      await updateFollowUp(editingId.value, payload)
      ElMessage.success('已更新')
    } else {
      await createFollowUp(payload)
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

async function onDelete(row: FollowUp) {
  await ElMessageBox.confirm('确认删除该跟进记录？', '提示', { type: 'warning' })
  await deleteFollowUp(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(async () => {
  customerOptions.value = await listAllCustomers()
  const contacts = await listContacts({ pageNum: 1, pageSize: 100 })
  allContacts.value = contacts.records
  load()
})
</script>
