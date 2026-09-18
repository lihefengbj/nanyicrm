<template>
  <div>
    <el-card>
      <template #header>
        <span>工作台</span>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="账号">{{ store.profile?.username }}</el-descriptions-item>
        <el-descriptions-item label="昵称">{{ store.profile?.nickname || '-' }}</el-descriptions-item>
        <el-descriptions-item label="部门">{{ store.profile?.dept?.name || '-' }}</el-descriptions-item>
        <el-descriptions-item label="角色">{{ roleText }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-row v-if="summary" :gutter="16" class="stats">
      <el-col :span="6">
        <el-card shadow="hover" @click="go('/crm/customer')">
          <el-statistic title="客户总数" :value="summary.customerTotal" />
          <div class="stat-sub">我负责的 {{ summary.myCustomerTotal }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" @click="go('/sales/opportunity')">
          <el-statistic title="进行中商机" :value="summary.openOppCount" />
          <div class="stat-sub">预计金额 {{ fmtAmount(summary.openOppAmount) }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" @click="go('/sales/contract')">
          <el-statistic title="有效合同" :value="summary.contractTotal" />
          <div class="stat-sub">合同金额 {{ fmtAmount(summary.contractAmount) }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" @click="go('/crm/follow')">
          <el-statistic title="近 7 天跟进" :value="summary.followWeekCount" />
          <div class="stat-sub">待跟进 {{ summary.pendingFollowCount }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card v-if="workbench && store.hasPerm('crm:intent:workbench')" class="intent-workbench">
      <template #header>
        <div class="card-header">
          <span>AI意向待跟进</span>
          <el-button link type="primary" @click="go('/crm/customer')">查看客户列表</el-button>
        </div>
      </template>
      <el-row :gutter="16">
        <el-col :span="6"><el-statistic title="高意向客户" :value="workbench.highIntentCount" /></el-col>
        <el-col :span="6"><el-statistic title="今日待跟进" :value="workbench.todayFollowUpCount" /></el-col>
        <el-col :span="6"><el-statistic title="已逾期" :value="workbench.overdueFollowUpCount" /></el-col>
        <el-col :span="6"><el-statistic title="近7日分析失败" :value="workbench.failedAnalysisCount" /></el-col>
      </el-row>
      <el-divider />
      <el-row :gutter="24">
        <el-col :span="12">
          <div class="intent-list-title">最近意向升高</div>
          <el-empty v-if="!workbench.risingCustomers.length" description="暂无数据" :image-size="50" />
          <div v-for="item in workbench.risingCustomers" :key="item.customerId" class="intent-list-item" @click="goCustomer(item.customerId)">
            <span>{{ item.customerName || `客户#${item.customerId}` }}</span>
            <el-tag type="success">+{{ item.scoreDiff }}</el-tag>
          </div>
        </el-col>
        <el-col :span="12">
          <div class="intent-list-title">最近意向降低</div>
          <el-empty v-if="!workbench.fallingCustomers.length" description="暂无数据" :image-size="50" />
          <div v-for="item in workbench.fallingCustomers" :key="item.customerId" class="intent-list-item" @click="goCustomer(item.customerId)">
            <span>{{ item.customerName || `客户#${item.customerId}` }}</span>
            <el-tag type="danger">{{ item.scoreDiff }}</el-tag>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <el-row :gutter="16" class="quick">
      <el-col v-if="store.hasPerm('system:user:list')" :span="8">
        <el-card shadow="hover" class="quick-card" @click="router.push('/system/user')">
          <div class="quick-title">用户管理</div>
          <div class="quick-desc">维护系统用户及角色分配</div>
        </el-card>
      </el-col>
      <el-col v-if="store.hasPerm('system:role:list')" :span="8">
        <el-card shadow="hover" class="quick-card" @click="router.push('/system/role')">
          <div class="quick-title">角色管理</div>
          <div class="quick-desc">配置角色与菜单权限</div>
        </el-card>
      </el-col>
      <el-col v-if="store.hasPerm('system:dept:list')" :span="8">
        <el-card shadow="hover" class="quick-card" @click="router.push('/system/dept')">
          <div class="quick-title">部门管理</div>
          <div class="quick-desc">维护组织架构</div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'Dashboard' })
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { dashboardSummary, getIntentWorkbench } from '@/api/crm'
import type { DashboardSummary, IntentWorkbench } from '@/types/api'

const store = useUserStore()
const router = useRouter()

const roleText = computed(() => (store.profile?.roles.length ? store.profile.roles.join('、') : '-'))
const canLoadIntentWorkbench = computed(() => {
  if (!store.hasPerm('crm:intent:workbench')) return false
  if (!store.profile?.isPrivileged) return true
  return (store.profile.tenantId ?? 0) > 0
})

const summary = ref<DashboardSummary | null>(null)
const workbench = ref<IntentWorkbench | null>(null)

function fmtAmount(v: number) {
  return v >= 10000 ? `${(v / 10000).toFixed(1)} 万` : v.toFixed(2)
}

function go(path: string) {
  router.push(path)
}

function goCustomer(id: number) {
  router.push({ path: '/crm/customer', query: { customerId: id } })
}

onMounted(async () => {
  try {
    summary.value = await dashboardSummary()
  } catch {
    // summary stays hidden when the CRM module is unavailable
  }
  if (canLoadIntentWorkbench.value) {
    const tenantId = store.profile?.isPrivileged ? store.profile.tenantId : undefined
    workbench.value = await getIntentWorkbench(tenantId).catch(() => null)
  }
})
</script>

<style scoped>
.quick {
  margin-top: 16px;
}
.intent-workbench {
  margin-top: 16px;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.intent-list-title {
  margin-bottom: 8px;
  font-weight: 600;
}
.intent-list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
  cursor: pointer;
}
.stats {
  margin-top: 16px;
  cursor: pointer;
}
.stat-sub {
  margin-top: 6px;
  color: #909399;
  font-size: 12px;
}
.quick-card {
  cursor: pointer;
}
.quick-title {
  font-size: 16px;
  font-weight: 600;
}
.quick-desc {
  margin-top: 8px;
  color: #909399;
  font-size: 13px;
}
</style>

