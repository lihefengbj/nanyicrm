<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="logo">
        <span class="logo-mark">N</span>
        <span class="logo-name">Nanyi CRM</span>
      </div>
      <el-menu :default-active="activePath" router background-color="transparent" text-color="#a8c5c2"
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
          <el-menu-item v-if="isPrivileged" index="/system/tenant">租户管理</el-menu-item>
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
            <el-tag v-if="store.profile?.tenant" size="small" effect="plain" class="tenant-tag">{{ store.profile.tenant.name }}</el-tag>
            <el-tag v-else-if="isSuper" size="small" type="warning" effect="plain" class="tenant-tag">平台</el-tag>
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
      <TabsBar />
      <el-main class="main">
        <router-view v-slot="{ Component }">
          <keep-alive :include="tabs.cachedNames">
            <component :is="Component" :key="route.path" />
          </keep-alive>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Monitor, Setting, ArrowDown } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { useTabsStore } from '@/store/tabs'
import TabsBar from './TabsBar.vue'
import { logout } from '@/api/auth'

const route = useRoute()
const router = useRouter()
const store = useUserStore()
const tabs = useTabsStore()

const activePath = computed(() => route.path)
const isSuper = computed(() => store.profile?.isSuper ?? false)
const isPrivileged = computed(() => store.profile?.isPrivileged ?? false)
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
    tabs.reset()
    router.push('/login')
  }
}
</script>

<style scoped>
.layout {
  height: 100vh;
}
.aside {
  background: linear-gradient(180deg, #0f766e 0%, #115e59 60%, #134e4a 100%);
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #fff;
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
}
.logo-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.16);
  border: 1px solid rgba(255, 255, 255, 0.3);
  font-size: 15px;
  font-weight: 700;
}
.logo-name {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.5px;
}
.aside :deep(.el-menu) {
  border-right: none;
  background: transparent;
  padding: 8px;
}
.aside :deep(.el-menu-item),
.aside :deep(.el-sub-menu__title) {
  border-radius: 6px;
  margin: 2px 0;
  height: 44px;
  line-height: 44px;
}
.aside :deep(.el-menu-item:hover),
.aside :deep(.el-sub-menu__title:hover) {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}
.aside :deep(.el-menu-item.is-active) {
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
}
.aside :deep(.el-menu) .el-menu {
  background: transparent;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e4e7ed;
  background: #fff;
}
.tenant-tag {
  margin-right: 4px;
}
.user-entry {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}
.main {
  background: #eef1f4;
}
</style>






