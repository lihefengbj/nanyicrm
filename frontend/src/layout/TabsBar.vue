<template>
  <div class="tabs-bar">
    <el-scrollbar>
      <div class="tabs-wrap">
        <div
          v-for="tab in tabs.tabs"
          :key="tab.path"
          class="tab"
          :class="{ active: tab.path === route.path }"
          @click="onClick(tab)"
          @contextmenu.prevent="openMenu(tab, $event)"
        >
          <span class="tab-dot" />
          <span class="tab-title">{{ tab.title }}</span>
          <el-icon v-if="tab.closable" class="tab-close" @click.stop="onClose(tab)">
            <Close />
          </el-icon>
        </div>
      </div>
    </el-scrollbar>

    <teleport to="body">
      <ul v-show="menu.visible" class="tab-menu" :style="{ left: menu.x + 'px', top: menu.y + 'px' }">
        <li @click="closeCurrent">关闭当前</li>
        <li @click="closeOthers">关闭其他</li>
        <li @click="closeAll">关闭全部</li>
      </ul>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, watch } from "vue"
import { useRoute, useRouter } from "vue-router"
import { Close } from "@element-plus/icons-vue"
import { useTabsStore, type TabItem } from "@/store/tabs"

const route = useRoute()
const router = useRouter()
const tabs = useTabsStore()

watch(
  () => route.path,
  () => {
    if (!route.name || route.meta.public) return
    tabs.add({
      path: route.path,
      title: (route.meta.title as string) || String(route.name),
      name: String(route.name),
      closable: route.path !== "/dashboard",
    })
  },
  { immediate: true },
)

function onClick(tab: TabItem) {
  if (tab.path !== route.path) router.push(tab.path)
}

function onClose(tab: TabItem) {
  const next = tabs.close(tab.path, route.path)
  if (next) router.push(next)
}

const menu = reactive({ visible: false, x: 0, y: 0, tab: null as TabItem | null })

function openMenu(tab: TabItem, e: MouseEvent) {
  menu.visible = true
  menu.x = e.clientX
  menu.y = e.clientY
  menu.tab = tab
}

function closeCurrent() {
  if (menu.tab) onClose(menu.tab)
}
function closeOthers() {
  if (!menu.tab) return
  tabs.closeOthers(menu.tab.path)
  if (route.path !== menu.tab.path) router.push(menu.tab.path)
}
function closeAll() {
  tabs.closeAll()
  router.push("/dashboard")
}

function hideMenu() {
  menu.visible = false
}
onMounted(() => document.addEventListener("click", hideMenu))
onUnmounted(() => document.removeEventListener("click", hideMenu))
</script>

<style scoped>
.tabs-bar {
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  padding: 6px 12px 0;
}
.tabs-wrap {
  display: flex;
  gap: 6px;
  width: max-content;
}
.tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  font-size: 12px;
  color: #606266;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.tab:hover {
  color: var(--el-color-primary);
}
.tab.active {
  color: #fff;
  background: var(--el-color-primary);
  border-color: var(--el-color-primary);
}
.tab-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0;
}
.tab.active .tab-dot {
  opacity: 1;
}
.tab-close {
  font-size: 12px;
  border-radius: 50%;
}
.tab-close:hover {
  background: rgba(255, 255, 255, 0.3);
}
.tab:not(.active) .tab-close:hover {
  background: rgba(0, 0, 0, 0.12);
}

.tab-menu {
  position: fixed;
  z-index: 3000;
  margin: 0;
  padding: 4px 0;
  list-style: none;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.12);
  font-size: 12px;
  color: #606266;
}
.tab-menu li {
  padding: 7px 18px;
  cursor: pointer;
}
.tab-menu li:hover {
  background: #f5f7fa;
  color: var(--el-color-primary);
}
</style>
