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
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()

const roleText = computed(() => (store.profile?.roles.length ? store.profile.roles.join('、') : '-'))
</script>

<style scoped>
.quick {
  margin-top: 16px;
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

