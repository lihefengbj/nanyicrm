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
import { dashboardSummary } from '@/api/crm'
import type { DashboardSummary } from '@/types/api'

const store = useUserStore()
const router = useRouter()

const roleText = computed(() => (store.profile?.roles.length ? store.profile.roles.join('、') : '-'))

const summary = ref<DashboardSummary | null>(null)

function fmtAmount(v: number) {
  return v >= 10000 ? `${(v / 10000).toFixed(1)} 万` : v.toFixed(2)
}

function go(path: string) {
  router.push(path)
}

onMounted(async () => {
  try {
    summary.value = await dashboardSummary()
  } catch {
    // summary stays hidden when the CRM module is unavailable
  }
})
</script>

<style scoped>
.quick {
  margin-top: 16px;
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

