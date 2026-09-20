<template>
  <div class="intent-metrics-page">
    <el-card class="filter-card">
      <el-form inline @submit.prevent>
        <el-form-item label="统计范围">
          <el-date-picker
            v-model="query.range"
            type="daterange"
            value-format="YYYY-MM-DD"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            :clearable="false"
            style="width: 280px"
          />
        </el-form-item>
        <el-form-item v-if="store.profile?.isPrivileged" label="租户">
          <el-select v-model="query.tenantId" placeholder="请选择租户" style="width: 220px">
            <el-option
              v-for="tenant in tenantOptions"
              :key="tenant.id"
              :label="`${tenant.name}（${tenant.code}）`"
              :value="tenant.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="load">查询</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-alert
      v-if="metrics && !metrics.costConfigured"
      title="尚未配置 Token 价格，费用仅显示为未配置；请在后端 llm.pricing 中填写供应商价格。"
      type="info"
      :closable="false"
      style="margin-top: 16px"
    />

    <el-row v-if="metrics" :gutter="16" class="stats">
      <el-col :xs="12" :sm="8" :lg="6"><el-card><el-statistic title="请求数" :value="metrics.requestCount" /></el-card></el-col>
      <el-col :xs="12" :sm="8" :lg="6">
        <el-card>
          <div class="stat-title">成功率</div>
          <div class="stat-value">{{ formatPercent(metrics.successRate) }}</div>
          <div class="stat-subtitle">成功 {{ formatNumber(metrics.successCount) }} 次</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="6">
        <el-card>
          <div class="stat-title">失败数</div>
          <div class="stat-value">{{ formatNumber(metrics.failedCount) }}</div>
          <div class="stat-subtitle">未完成 {{ formatNumber(metrics.incompleteCount) }} 条</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="6"><el-card><el-statistic title="总 Token" :value="metrics.totalTokens" /></el-card></el-col>
      <el-col :xs="12" :sm="8" :lg="6">
        <el-card>
          <div class="stat-title">分析耗时</div>
          <div class="stat-value">{{ formatMillis(metrics.averageCostMillis) }}</div>
          <div class="stat-subtitle">P95 {{ formatMillis(metrics.p95CostMillis) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="6">
        <el-card>
          <div class="stat-title">反馈采纳率</div>
          <div class="stat-value">{{ formatPercent(metrics.acceptanceRate) }}</div>
          <div class="stat-subtitle">反馈 {{ formatNumber(metrics.feedbackCount) }} 条</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="6">
        <el-card>
          <div class="cost-title">预估输入费用</div>
          <div class="cost-value">{{ formatCost(metrics.inputCost) }}</div>
          <div v-if="metrics.costConfigured" class="cost-breakdown">
            命中 {{ formatCost(metrics.inputCacheHitCost) }} · 未命中 {{ formatCost(metrics.inputCacheMissCost) }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="6">
        <el-card>
          <div class="cost-title">预估输出费用</div>
          <div class="cost-value">{{ formatCost(metrics.outputCost) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="6">
        <el-card>
          <div class="cost-title">预估总费用</div>
          <div class="cost-value">{{ formatCost(metrics.estimatedCost) }}</div>
          <div v-if="metrics.costConfigured" class="cost-breakdown">输入 + 输出</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card v-if="metrics" class="panel">
      <template #header>
        <div class="card-header">
          <span>模型调用分组</span>
          <div class="header-meta">
            <span class="scope-note">按分析创建时间统计</span>
            <el-tag v-if="metrics.costConfigured" type="success">
              {{ metrics.costCurrency }} · {{ formatPricingMode(metrics.costMode, metrics.costFixedPeriod) }}
            </el-tag>
          </div>
        </div>
      </template>
      <el-table :data="metrics.byProvider" border stripe>
        <el-table-column prop="provider" label="Provider" width="120" show-overflow-tooltip />
        <el-table-column label="配置模型" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ displayValue(row.model) }}</template>
        </el-table-column>
        <el-table-column label="实际模型" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ displayValue(row.actualModel) }}</template>
        </el-table-column>
        <el-table-column label="配置版本" width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ displayValue(row.modelConfigVersion) }}</template>
        </el-table-column>
        <el-table-column label="Prompt" width="100" show-overflow-tooltip>
          <template #default="{ row }">{{ displayValue(row.promptVersion) }}</template>
        </el-table-column>
        <el-table-column label="结果状态" width="150">
          <template #default="{ row }">
            <span class="status-summary">
              成功 {{ row.successCount }} · 失败 {{ row.failedCount }} · 未完成 {{ row.incompleteCount }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="请求数" width="90" />
        <el-table-column label="计费时段" width="100">
          <template #default="{ row }">{{ formatPeriod(row.billingPeriod) }}</template>
        </el-table-column>
        <el-table-column prop="totalTokens" label="Token" width="110" />
        <el-table-column label="预估费用" width="130">
          <template #default="{ row }">{{ formatCost(row.estimatedCost) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!metrics.byProvider.length" description="暂无调用数据" />
    </el-card>

    <el-card v-if="metrics" class="panel">
      <template #header>
        <div class="card-header">
          <span>计费时段汇总</span>
          <span class="scope-note">历史记录优先使用实际计费时段</span>
        </div>
      </template>
      <el-table :data="periodRows" border stripe>
        <el-table-column label="计费时段" width="140">
          <template #default="{ row }">{{ formatPeriod(row.period) }}</template>
        </el-table-column>
        <el-table-column prop="count" label="请求数" width="100" />
        <el-table-column prop="successCount" label="成功数" width="100" />
        <el-table-column prop="failedCount" label="失败数" width="100" />
        <el-table-column prop="incompleteCount" label="未完成" width="100" />
        <el-table-column prop="totalTokens" label="Token" min-width="120" />
        <el-table-column label="预估费用" min-width="140">
          <template #default="{ row }">{{ formatCost(row.estimatedCost) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!periodRows.length" description="暂无计费记录" />
    </el-card>

    <el-row v-if="metrics" :gutter="16" class="panel">
      <el-col :span="12">
        <el-card>
          <template #header>失败原因</template>
          <el-table :data="metrics.failureReasons" border stripe>
            <el-table-column label="原因" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ formatFailureReason(row.reason) }}</template>
            </el-table-column>
            <el-table-column prop="count" label="次数" width="100" />
          </el-table>
          <el-empty v-if="!metrics.failureReasons.length" description="暂无失败记录" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>当前客户意向等级分布</span>
              <span class="scope-note">共 {{ formatNumber(currentIntentTotal) }} 个客户</span>
            </div>
          </template>
          <el-table :data="levelRows" border stripe>
            <el-table-column label="等级">
              <template #default="{ row }">{{ row.label }}</template>
            </el-table-column>
            <el-table-column prop="count" label="数量" />
            <el-table-column label="占比" width="100">
              <template #default="{ row }">{{ formatPercent(row.ratio) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'CrmIntentMetrics' })

import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getIntentMetrics } from '@/api/crm'
import { listAllTenants } from '@/api/system'
import type { IntentMetricGroup, IntentMetrics, Tenant } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const loading = ref(false)
const metrics = ref<IntentMetrics | null>(null)
const tenantOptions = ref<Tenant[]>([])

function dateText(date: Date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const today = new Date()
const weekAgo = new Date(today)
weekAgo.setDate(today.getDate() - 6)
const query = reactive({
  range: [dateText(weekAgo), dateText(today)] as [string, string],
  tenantId: store.profile?.tenantId ?? undefined,
})

const levelLabels: Record<string, string> = {
  high: '高意向',
  medium: '中意向',
  low: '低意向',
  unknown: '未知',
}

const currentIntentTotal = computed(() =>
  Object.values(metrics.value?.levelDistribution ?? {}).reduce((total, count) => total + count, 0),
)

const levelRows = computed(() =>
  ['high', 'medium', 'low', 'unknown'].map((level) => {
    const count = metrics.value?.levelDistribution[level] ?? 0
    return {
      level,
      label: levelLabels[level],
      count,
      ratio: currentIntentTotal.value > 0 ? count / currentIntentTotal.value : 0,
    }
  }),
)

const periodRows = computed(() => {
  const groups = new Map<string, {
    period: IntentMetricGroup['billingPeriod']
    count: number
    successCount: number
    failedCount: number
    incompleteCount: number
    totalTokens: number
    estimatedCost: number
  }>()
  for (const group of metrics.value?.byProvider ?? []) {
    const current = groups.get(group.billingPeriod) ?? {
      period: group.billingPeriod,
      count: 0,
      successCount: 0,
      failedCount: 0,
      incompleteCount: 0,
      totalTokens: 0,
      estimatedCost: 0,
    }
    current.count += group.count
    current.successCount += group.successCount
    current.failedCount += group.failedCount
    current.incompleteCount += group.incompleteCount
    current.totalTokens += group.totalTokens
    current.estimatedCost += group.estimatedCost
    groups.set(group.billingPeriod, current)
  }
  return ['peak', 'idle', 'unified']
    .map((period) => groups.get(period as IntentMetricGroup['billingPeriod']))
    .filter((row): row is NonNullable<typeof row> => !!row)
})

function formatNumber(value: number) {
  return new Intl.NumberFormat('zh-CN').format(value)
}

function formatPercent(value: number) {
  return `${(value * 100).toFixed(1)}%`
}

function formatMillis(value: number) {
  return `${formatNumber(Math.round(value))} ms`
}

function formatCost(value: number) {
  if (!metrics.value?.costConfigured) return '未配置'
  return `${metrics.value.costCurrency} ${value.toFixed(6)}`
}

function displayValue(value: string) {
  return value?.trim() || '—'
}

function formatPeriod(period: IntentMetricGroup['billingPeriod']) {
  if (period === 'peak') return '高峰'
  if (period === 'unified') return '统一价格'
  if (period === 'idle') return '空闲'
  return '未记录'
}

function formatPricingMode(mode: IntentMetrics['costMode'], fixedPeriod: IntentMetrics['costFixedPeriod']) {
  if (mode === 'unified') return '统一价格'
  if (fixedPeriod) return `自动模式·强制${fixedPeriod === 'peak' ? '高峰' : '空闲'}`
  return '自动时段'
}

function formatFailureReason(reason: string) {
  const labels: Record<string, string> = {
    config: '配置错误',
    timeout: '请求超时',
    network: '网络错误',
    rate_limit: '供应商限流',
    authentication: '鉴权失败',
    model_not_found: '模型不存在',
    invalid_request: '请求参数错误',
    provider_error: '供应商错误',
    response_decode: '供应商响应解析失败',
    result_decode: '意向结果解析失败',
    empty_response: '供应商返回空响应',
    result_validation: '意向结果校验失败',
    quota_exceeded: '超过租户额度',
  }
  const normalized = reason.trim().toLowerCase()
  if (labels[normalized]) return labels[normalized]
  if (normalized.includes('no message content')) return '供应商返回空响应'
  if (normalized.includes('unexpected end') || normalized.includes('decode intent result')) return '意向结果解析失败'
  return reason || '未知错误'
}

async function load() {
  if (store.profile?.isPrivileged && !query.tenantId) {
    ElMessage.warning('平台用户查询指标时必须选择租户')
    return
  }
  loading.value = true
  try {
    metrics.value = await getIntentMetrics({
      from: query.range[0],
      to: query.range[1],
      tenantId: store.profile?.isPrivileged ? query.tenantId : undefined,
    })
  } finally {
    loading.value = false
  }
}

async function loadTenantOptions() {
  if (!store.profile?.isPrivileged) return
  tenantOptions.value = await listAllTenants().catch(() => [])
  if (!query.tenantId && tenantOptions.value.length > 0) {
    query.tenantId = tenantOptions.value[0].id
  }
}

onMounted(async () => {
  await loadTenantOptions()
  await load()
})
</script>

<style scoped>
.intent-metrics-page {
  min-width: 0;
}
.stats,
.panel {
  margin-top: 16px;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.header-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}
.scope-note,
.stat-subtitle {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.stat-title {
  color: var(--el-text-color-regular);
  font-size: 14px;
}
.stat-value {
  margin-top: 10px;
  font-size: 24px;
  line-height: 1;
}
.cost-title {
  color: var(--el-text-color-regular);
  font-size: 14px;
}
.cost-value {
  margin-top: 10px;
  font-size: 24px;
  line-height: 1;
}
.cost-breakdown {
  margin-top: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
}
.status-summary {
  color: var(--el-text-color-regular);
  font-size: 12px;
}
</style>
