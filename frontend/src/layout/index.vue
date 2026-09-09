<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="logo">Nanyi CRM</div>
      <el-menu :default-active="activePath" router background-color="#001529" text-color="#a6adb4"
        active-text-color="#ffffff">
        <el-menu-item index="/dashboard">
          <el-icon><Monitor /></el-icon>
          <span>工作台</span>
        </el-menu-item>
        <el-sub-menu v-if="showSystem" index="/system">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>系统管理</span>
          </template>
          <el-menu-item v-if="store.hasPerm('system:user:list')" index="/system/user">用户管理</el-menu-item>
          <el-menu-item v-if="store.hasPerm('system:role:list')" index="/system/role">角色管理</el-menu-item>
          <el-menu-item v-if="store.hasPerm('system:dept:list')" index="/system/dept">部门管理</el-menu-item>
          <el-menu-item v-if="store.hasPerm('system:log:oper')" index="/system/operlog">操作日志</el-menu-item>
          <el-menu-item v-if="store.hasPerm('system:log:login')" index="/system/loginlog">登录日志</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item>{{ route.meta.title || '工作台' }}</el-breadcrumb-item>
        </el-breadcrumb>
        <el-dropdown @command="onCommand">
          <span class="user-entry">
            {{ store.profile?.nickname || store.profile?.username }}
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Monitor, Setting, ArrowDown } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { logout } from '@/api/auth'

const route = useRoute()
const router = useRouter()
const store = useUserStore()

const activePath = computed(() => route.path)
const showSystem = computed(
  () =>
    store.hasPerm('system:user:list') ||
    store.hasPerm('system:role:list') ||
    store.hasPerm('system:dept:list') ||
    store.hasPerm('system:log:oper') ||
    store.hasPerm('system:log:login'),
)

async function onCommand(cmd: string) {
  if (cmd === 'logout') {
    try {
      await logout(store.refreshToken)
    } catch {
      // best effort
    }
    store.logout()
    router.push('/login')
  }
}
</script>

<style scoped>
.layout {
  height: 100vh;
}
.aside {
  background-color: #001529;
}
.logo {
  height: 60px;
  line-height: 60px;
  text-align: center;
  color: #fff;
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 1px;
}
.aside :deep(.el-menu) {
  border-right: none;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e4e7ed;
  background: #fff;
}
.user-entry {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}
.main {
  background: #f0f2f5;
}
</style>
