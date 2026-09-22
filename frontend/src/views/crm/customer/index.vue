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
        <el-form-item label="AI意向">
          <el-select v-model="query.intentLevel" clearable placeholder="全部" style="width: 120px">
            <el-option label="高意向" value="high" />
            <el-option label="中意向" value="medium" />
            <el-option label="低意向" value="low" />
            <el-option label="未知" value="unknown" />
            <el-option label="未分析" value="none" />
          </el-select>
        </el-form-item>
        <el-form-item label="分析状态">
          <el-select v-model="query.intentStatus" clearable placeholder="全部" style="width: 120px">
            <el-option label="未分析" value="unanalysed" />
            <el-option label="分析中" value="running" />
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
          </el-select>
        </el-form-item>
        <el-form-item label="分数">
          <el-input-number v-model="query.minScore" :min="0" :max="100" controls-position="right" placeholder="最低" style="width: 105px" />
          <span class="range-separator">-</span>
          <el-input-number v-model="query.maxScore" :min="0" :max="100" controls-position="right" placeholder="最高" style="width: 105px" />
        </el-form-item>
        <el-form-item label="最低置信度">
          <el-input-number v-model="query.minConfidence" :min="0" :max="1" :step="0.1" :precision="2" controls-position="right" placeholder="0~1" style="width: 115px" />
        </el-form-item>
        <el-form-item label="待跟进">
          <el-select v-model="query.followUpStatus" clearable placeholder="全部" style="width: 130px">
            <el-option label="今日到期" value="today" />
            <el-option label="已逾期" value="overdue" />
            <el-option label="未来待跟进" value="future" />
            <el-option label="无建议时间" value="none" />
          </el-select>
        </el-form-item>
        <el-form-item label="最近分析">
          <el-select v-model="query.analyzedWithin" clearable placeholder="不限" style="width: 120px">
            <el-option label="最近7天" value="7d" />
            <el-option label="最近30天" value="30d" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序">
          <el-select v-model="query.intentSort" clearable placeholder="默认" style="width: 150px">
            <el-option label="意向分数降序" value="score_desc" />
            <el-option label="置信度降序" value="confidence_desc" />
            <el-option label="最近分析倒序" value="analyzed_desc" />
            <el-option label="建议跟进正序" value="follow_up_asc" />
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
          <el-button v-if="store.hasPerm('crm:intent:batch')" type="warning" plain @click="onBatchAnalyze">
            批量AI分析{{ selectedIds.length ? `（${selectedIds.length}）` : '' }}
          </el-button>
          <el-button v-if="store.hasPerm('crm:customer:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增客户
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border @selection-change="onSelectionChange">
        <el-table-column type="selection" width="45" />
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
        <el-table-column label="AI意向" width="120">
          <template #default="{ row }">
            <el-button v-if="row.intent" link type="primary" @click="openIntent(row)">
              <el-tag :type="intentTag(effectiveIntentLevel(row.intent))">
                {{ intentText(effectiveIntentLevel(row.intent)) }}{{ !row.intent.manualOverride && row.intent.intentScore != null ? ` ${row.intent.intentScore}` : '' }}
              </el-tag>
            </el-button>
            <span v-else class="muted-text">未分析</span>
            <el-tag v-if="row.intent?.followUpStatus === 'overdue'" type="danger" size="small">逾期</el-tag>
            <el-tag v-else-if="row.intent?.followUpStatus === 'today'" type="warning" size="small">今日</el-tag>
            <el-tag v-if="row.intent?.manualOverride" type="success" size="small">人工修正</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="归属人" width="110">
          <template #default="{ row }">{{ row.owner?.nickname || row.owner?.username || '-' }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ formatBeijingTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="310" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('crm:customer:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button link type="primary" @click="goFollow(row)">跟进</el-button>
            <el-button v-if="store.hasPerm('crm:intent:list') && row.intent" link type="primary" @click="openIntent(row)">意向详情</el-button>
            <el-button v-if="store.hasPerm('crm:intent:analyze')" link type="primary" :loading="analyzingId === row.id" @click="onAnalyze(row)">
              AI分析
            </el-button>
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

    <el-dialog v-model="intentVisible" title="AI客户意向" width="680px" destroy-on-close>
      <el-skeleton v-if="intentLoading" :rows="6" animated />
      <template v-else>
        <el-empty v-if="!intentResult" description="暂无AI分析结果" />
        <template v-else>
          <div class="intent-header">
            <el-tag :type="intentTag(intentResult.intentLevel)" size="large">
              AI判断：{{ intentText(intentResult.intentLevel) }}
            </el-tag>
            <span v-if="intentResult.intentScore != null" class="intent-score">{{ intentResult.intentScore }} 分</span>
            <span v-if="intentResult.confidence != null" class="muted-text">
              置信度 {{ Math.round(intentResult.confidence * 100) }}%
            </span>
            <el-tag v-if="intentResult.manualOverride" :type="intentTag(effectiveIntentLevel(intentResult))" size="large">
              当前有效：{{ intentText(effectiveIntentLevel(intentResult)) }}（人工）
            </el-tag>
          </div>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="分析摘要" :span="2">{{ intentResult.summary || '-' }}</el-descriptions-item>
            <el-descriptions-item label="预算">{{ intentResult.budget || '未明确' }}</el-descriptions-item>
            <el-descriptions-item label="采购时间">{{ intentResult.purchaseTimeline || '未明确' }}</el-descriptions-item>
            <el-descriptions-item label="决策角色">{{ intentResult.decisionRole || '未明确' }}</el-descriptions-item>
            <el-descriptions-item label="建议跟进时间">{{ formatBeijingTime(intentResult.suggestedNextAt) }}</el-descriptions-item>
            <el-descriptions-item label="需求" :span="2">{{ intentResult.needs.join('、') || '-' }}</el-descriptions-item>
            <el-descriptions-item label="客户痛点" :span="2">{{ intentResult.painPoints.join('、') || '-' }}</el-descriptions-item>
            <el-descriptions-item label="风险" :span="2">{{ intentResult.risks.join('、') || '-' }}</el-descriptions-item>
            <el-descriptions-item label="下一步建议" :span="2">{{ intentResult.nextAction || '-' }}</el-descriptions-item>
          </el-descriptions>
          <el-alert v-if="intentResult.followUpStatus === 'overdue'" title="建议跟进时间已逾期" type="error" :closable="false" style="margin-top: 12px" />
          <div v-if="latestInputSummary" class="intent-input">最近一次输入：{{ latestInputSummary }}</div>
          <el-divider />
          <el-alert v-if="latestFeedback" type="success" :closable="false" class="latest-feedback">
            <template #title>
              最近反馈：{{ feedbackTypeText(latestFeedback.feedbackType) }}
              · {{ latestFeedback.userName || `用户${latestFeedback.userId}` }}
              · {{ formatBeijingTime(latestFeedback.createdAt) }}
            </template>
            <div>
              建议采纳：{{ acceptedText(latestFeedback.accepted) }}
              <span v-if="latestFeedback.manualIntentLevel">
                · 人工意向：{{ intentText(latestFeedback.manualIntentLevel) }}
              </span>
              <span v-if="latestFeedback.note"> · 备注：{{ latestFeedback.note }}</span>
            </div>
          </el-alert>
          <div class="feedback-panel">
            <div class="feedback-title">人工反馈</div>
            <el-radio-group v-model="feedback.feedbackType">
              <el-radio value="accurate">准确</el-radio>
              <el-radio value="partial">部分准确</el-radio>
              <el-radio value="inaccurate">不准确</el-radio>
            </el-radio-group>
            <el-checkbox v-model="feedback.accepted">采纳下一步建议</el-checkbox>
            <el-select v-model="feedback.manualIntentLevel" clearable placeholder="人工修正等级" style="width: 140px">
              <el-option label="高意向" value="high" />
              <el-option label="中意向" value="medium" />
              <el-option label="低意向" value="low" />
              <el-option label="未知" value="unknown" />
            </el-select>
            <el-input v-model="feedback.note" maxlength="1024" show-word-limit placeholder="备注（可选）" style="width: 240px" />
            <el-button v-if="intentCustomer && store.hasPerm('crm:intent:feedback')" type="primary" @click="submitFeedback">提交反馈</el-button>
          </div>
          <el-collapse v-if="canViewFeedback" v-model="feedbackCollapseNames" class="feedback-history">
            <el-collapse-item :title="`反馈历史（${feedbackTotal}）`" name="feedback">
              <el-empty v-if="!feedbackHistory.length" description="暂无人工反馈" :image-size="60" />
              <el-timeline v-else>
                <el-timeline-item v-for="item in feedbackHistory" :key="item.id" :timestamp="formatBeijingTime(item.createdAt)">
                  <div>
                    {{ feedbackTypeText(item.feedbackType) }} · 建议采纳：{{ acceptedText(item.accepted) }}
                    <span v-if="item.manualIntentLevel"> · 人工意向：{{ intentText(item.manualIntentLevel) }}</span>
                  </div>
                  <div class="muted-text">
                    {{ item.userName || `用户${item.userId}` }}
                    <span v-if="item.analysisAt"> · 对应分析：{{ formatBeijingTime(item.analysisAt) }}</span>
                  </div>
                  <div v-if="item.note" class="feedback-note">备注：{{ item.note }}</div>
                </el-timeline-item>
              </el-timeline>
              <el-pagination v-if="feedbackTotal > feedbackPageSize" v-model:current-page="feedbackPage"
                :page-size="feedbackPageSize" :total="feedbackTotal" small layout="prev, pager, next"
                @current-change="loadFeedback" />
            </el-collapse-item>
          </el-collapse>
          <el-alert v-if="compareResult?.previous" type="info" :closable="false" style="margin-top: 12px">
            与上次结果相比：分数{{ compareResult.scoreDiff == null ? '无变化' : `${compareResult.scoreDiff > 0 ? '+' : ''}${compareResult.scoreDiff}` }}，
            {{ compareResult.levelChanged ? '意向等级已变化' : '意向等级未变化' }}
          </el-alert>
          <div class="intent-meta">
            分析时间：{{ formatBeijingTime(intentResult.analyzedAt) }} · 模型：{{ intentResult.provider || '-' }}/{{ intentResult.model || '-' }}
            · Prompt版本：{{ intentResult.promptVersion || '未记录' }}
            · 模型配置版本：{{ currentHistory?.modelConfigVersion || '未记录' }}
          </div>
        </template>

        <el-divider v-if="intentHistory.length">分析历史</el-divider>
        <el-timeline v-if="intentHistory.length">
          <el-timeline-item v-for="item in intentHistory" :key="item.id" :timestamp="formatBeijingTime(item.createdAt)">
           <span>{{ item.status === 'success' ? '分析成功' : item.status === 'running' ? '分析中' : '分析失败' }}</span>
             <span v-if="item.result">：{{ intentText(item.result.intentLevel) }}{{ item.result.intentScore != null ? ` ${item.result.intentScore}分` : '' }}</span>
             <span v-else-if="item.errorMessage" class="muted-text">：{{ item.errorMessage }}</span>
             <div class="intent-history-meta">
               <el-tag size="small" type="info">Prompt版本：{{ item.promptVersion || '未记录' }}</el-tag>
               <span>模型配置版本：{{ item.modelConfigVersion || '未记录' }}</span>
               <span>实际模型：{{ item.actualModel || item.model || '未记录' }}</span>
             </div>
          </el-timeline-item>
        </el-timeline>
      </template>
      <template #footer>
        <el-button @click="intentVisible = false">关闭</el-button>
        <el-button v-if="intentCustomer && store.hasPerm('crm:intent:analyze')" type="primary" :loading="analyzingId === intentCustomer.id" @click="onAnalyze(intentCustomer)">
          重新分析
        </el-button>
        <el-button v-if="intentCustomer && store.hasPerm('crm:intent:analyze')" @click="onRetry">重试最近失败</el-button>
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
import {
  listCustomers,
  createCustomer,
  updateCustomer,
  deleteCustomer,
  getCustomerIntent,
  analyzeCustomerIntent,
  retryCustomerIntent,
  submitCustomerIntentFeedback,
  listCustomerIntentFeedback,
  compareCustomerIntent,
  listCustomerIntentHistory,
  batchAnalyzeCustomerIntent,
} from '@/api/crm'
import { listAllTenants } from '@/api/system'
import type { Customer, CustomerIntent, CustomerIntentFeedback, CustomerIntentHistory, Tenant } from '@/types/api'
import { useUserStore } from '@/store/user'
import { formatBeijingTime } from '@/utils/datetime'

const store = useUserStore()
const router = useRouter()
const isPrivileged = computed(() => store.profile?.isPrivileged ?? false)
const tenantOptions = ref<Tenant[]>([])

const loading = ref(false)
const rows = ref<Customer[]>([])
const total = ref(0)
const selectedIds = ref<number[]>([])
const onlyMine = ref(false)
const query = reactive({
  pageNum: 1,
  pageSize: 10,
  name: '',
  status: '',
  level: '',
  tenantId: undefined as number | undefined,
  intentLevel: '',
  intentStatus: '',
  minScore: undefined as number | undefined,
  maxScore: undefined as number | undefined,
  minConfidence: undefined as number | undefined,
  followUpStatus: '',
  analyzedWithin: '',
  intentSort: '',
})

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ tenantId: undefined as number | undefined, name: '', phone: '', source: '', industry: '', level: 'B', status: 1, address: '', remark: '' })
const analyzingId = ref<number | null>(null)
const intentVisible = ref(false)
const intentLoading = ref(false)
const intentCustomer = ref<Customer | null>(null)
const intentResult = ref<CustomerIntent | null>(null)
const intentHistory = ref<CustomerIntentHistory[]>([])
const latestInputSummary = ref('')
const compareResult = ref<Awaited<ReturnType<typeof compareCustomerIntent>> | null>(null)
const feedbackHistory = ref<CustomerIntentFeedback[]>([])
const feedbackTotal = ref(0)
const feedbackPage = ref(1)
const feedbackPageSize = 10
const feedbackCollapseNames = ref(['feedback'])
const feedback = reactive({
  feedbackType: 'accurate' as 'accurate' | 'partial' | 'inaccurate',
  accepted: false,
  manualIntentLevel: undefined as 'high' | 'medium' | 'low' | 'unknown' | undefined,
  note: '',
})

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
function intentText(level: CustomerIntent['intentLevel']) {
  return level === 'high' ? '高意向' : level === 'medium' ? '中意向' : level === 'low' ? '低意向' : '未知'
}
function intentTag(level: CustomerIntent['intentLevel']) {
  return level === 'high' ? 'danger' : level === 'medium' ? 'warning' : level === 'low' ? 'info' : ''
}
function effectiveIntentLevel(intent: CustomerIntent): CustomerIntent['intentLevel'] {
  return intent.effectiveIntentLevel || (intent.manualOverride && intent.manualIntentLevel
    ? intent.manualIntentLevel
    : intent.intentLevel)
}
function feedbackTypeText(type: CustomerIntentFeedback['feedbackType']) {
  return type === 'accurate' ? '准确' : type === 'partial' ? '部分准确' : '不准确'
}
function acceptedText(accepted?: boolean | null) {
  return accepted == null ? '未填写' : accepted ? '是' : '否'
}

const latestFeedback = computed(() => feedbackHistory.value[0] ?? null)
const canViewFeedback = computed(() =>
  store.hasPerm('crm:intent:feedback') || store.hasPerm('crm:intent:feedback:list'),
)
const currentAnalysisId = computed(() =>
  intentResult.value?.analysisId || intentHistory.value.find((item) => item.status === 'success' && item.result)?.id,
)
const currentHistory = computed(() =>
  intentHistory.value.find((item) => item.id === intentResult.value?.analysisId) ??
  intentHistory.value.find((item) => item.status === 'success' && item.result) ??
  null,
)

async function load(page?: number) {
  if (page) query.pageNum = page
  loading.value = true
  try {
    const data = await listCustomers({ ...query, mine: onlyMine.value ? '1' : '' })
    rows.value = data.records
    total.value = data.total
    selectedIds.value = []
  } finally {
    loading.value = false
  }
}

async function openIntent(row: Customer) {
  intentCustomer.value = row
  intentVisible.value = true
  intentLoading.value = true
  try {
    // The customer list returns an intent summary for performance. The
    // detail dialog needs the full structured result (needs, pain points,
    // budget, risks, etc.), so fetch it only when the dialog is opened.
    intentResult.value = await getCustomerIntent(row.id)
    const history = await listCustomerIntentHistory(row.id, { pageNum: 1, pageSize: 10 }).catch(() => null)
    intentHistory.value = history?.records ?? []
    latestInputSummary.value = intentHistory.value.find((item) => item.inputSummary)?.inputSummary ?? ''
    compareResult.value = await compareCustomerIntent(row.id).catch(() => null)
    await loadFeedback(1)
    resetFeedback()
  } finally {
    intentLoading.value = false
  }
}

async function onAnalyze(row: Customer) {
  analyzingId.value = row.id
  try {
    const result = await analyzeCustomerIntent(row.id)
    row.intent = result
    if (intentCustomer.value?.id === row.id) {
      intentResult.value = result
      const history = await listCustomerIntentHistory(row.id, { pageNum: 1, pageSize: 10 }).catch(() => null)
      intentHistory.value = history?.records ?? []
      latestInputSummary.value = intentHistory.value.find((item) => item.inputSummary)?.inputSummary ?? ''
      compareResult.value = await compareCustomerIntent(row.id).catch(() => null)
    } else {
      await openIntent(row)
    }
    ElMessage.success('AI意向分析完成')
  } catch {
    // interceptor shows the message
  } finally {
    analyzingId.value = null
  }
}

async function onRetry() {
  if (!intentCustomer.value) return
  analyzingId.value = intentCustomer.value.id
  try {
    const result = await retryCustomerIntent(intentCustomer.value.id)
    intentCustomer.value.intent = result
    intentResult.value = result
    const history = await listCustomerIntentHistory(result.customerId, { pageNum: 1, pageSize: 10 }).catch(() => null)
    intentHistory.value = history?.records ?? []
    compareResult.value = await compareCustomerIntent(result.customerId).catch(() => null)
    ElMessage.success('已重新分析')
  } finally {
    analyzingId.value = null
  }
}

async function submitFeedback() {
  if (!intentCustomer.value) return
  const requestedManualIntentLevel = feedback.manualIntentLevel
  const result = await submitCustomerIntentFeedback(intentCustomer.value.id, {
    analysisId: currentAnalysisId.value,
    feedbackType: feedback.feedbackType,
    accepted: feedback.accepted,
    manualIntentLevel: feedback.manualIntentLevel,
    note: feedback.note,
  })
  if (requestedManualIntentLevel && result.appliedToCurrent && intentResult.value) {
    intentResult.value.manualOverride = true
    intentResult.value.manualIntentLevel = requestedManualIntentLevel
    intentResult.value.effectiveIntentLevel = requestedManualIntentLevel
    intentCustomer.value.intent = intentResult.value
  }
  await loadFeedback(1)
  resetFeedback()
  if (requestedManualIntentLevel && !result.appliedToCurrent) {
    ElMessage.warning('反馈已保存；当前已有更新的分析结果，人工修正未覆盖新结果')
  } else {
    ElMessage.success('反馈已提交')
  }
}

async function loadFeedback(page = feedbackPage.value) {
  feedbackPage.value = page
  if (!intentCustomer.value || !canViewFeedback.value) {
    feedbackHistory.value = []
    feedbackTotal.value = 0
    return
  }
  try {
    const result = await listCustomerIntentFeedback(intentCustomer.value.id, {
      pageNum: page,
      pageSize: feedbackPageSize,
    })
    feedbackHistory.value = result.records ?? []
    feedbackTotal.value = result.total ?? 0
  } catch {
    // The request interceptor already displays the backend error. Keep the
    // dialog usable while avoiding a misleading empty-history success state.
    feedbackHistory.value = []
    feedbackTotal.value = 0
  }
}

function resetFeedback() {
  feedback.feedbackType = 'accurate'
  feedback.accepted = false
  feedback.manualIntentLevel = undefined
  feedback.note = ''
}

function onSelectionChange(selection: Customer[]) {
  selectedIds.value = selection.map((item) => item.id)
}

async function onBatchAnalyze() {
  const selected = selectedIds.value.length ? selectedIds.value : undefined
  let tenantId = query.tenantId
  if (isPrivileged.value && !tenantId && selected) {
    const selectedTenantIds = new Set(
      rows.value
        .filter((row) => selectedIds.value.includes(row.id))
        .map((row) => row.tenantId)
        .filter((id): id is number => !!id),
    )
    if (selectedTenantIds.size === 1) {
      tenantId = [...selectedTenantIds][0]
    }
  }
  if (isPrivileged.value && !tenantId) {
    ElMessage.warning('平台用户批量分析请先选择租户，或仅选择同一租户的客户')
    return
  }
  await ElMessageBox.confirm(
    selected ? `确认提交 ${selected.length} 个客户的AI分析任务？` : '确认按当前筛选条件提交最多50个客户的AI分析任务？',
    '批量AI分析',
    { type: 'warning' },
  )
  const task = await batchAnalyzeCustomerIntent({
    tenantId,
    customerIds: selected,
    intentLevel: selected ? undefined : query.intentLevel || undefined,
    minScore: selected ? undefined : query.minScore,
    maxScore: selected ? undefined : query.maxScore,
    followUpStatus: selected ? undefined : query.followUpStatus || undefined,
    limit: 50,
  })
  ElMessage.success(`已提交任务，共${task.totalCount}个客户`)
  load()
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

<style scoped>
.muted-text {
  color: var(--el-text-color-secondary);
}

.intent-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.intent-score {
  font-size: 20px;
  font-weight: 600;
}

.intent-meta {
  margin-top: 12px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.intent-history-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.range-separator {
  margin: 0 4px;
  color: var(--el-text-color-secondary);
}

.intent-input {
  margin-top: 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.feedback-panel {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.feedback-title {
  font-weight: 600;
}

.latest-feedback,
.feedback-history {
  margin-bottom: 12px;
}

.feedback-note {
  margin-top: 4px;
  white-space: pre-wrap;
}
</style>
