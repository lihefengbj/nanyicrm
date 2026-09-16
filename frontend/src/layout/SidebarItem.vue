<template>
  <template v-for="item in visibleMenus" :key="item.id">
    <el-sub-menu v-if="item.type === 1" :index="fullPath(item)">
      <template #title>
        <el-icon v-if="iconOf(item.icon)"><component :is="iconOf(item.icon)" /></el-icon>
        <span>{{ item.title }}</span>
      </template>
      <SidebarItem :menus="item.children ?? []" :base="fullPath(item)" />
    </el-sub-menu>
    <el-menu-item v-else :index="fullPath(item)">
      <el-icon v-if="iconOf(item.icon)"><component :is="iconOf(item.icon)" /></el-icon>
      <span>{{ item.title }}</span>
    </el-menu-item>
  </template>
</template>

<script setup lang="ts">
defineOptions({ name: 'SidebarItem' })
import { computed } from 'vue'
import * as icons from '@element-plus/icons-vue'
import { joinPath } from '@/router'
import type { Menu } from '@/types/api'

const props = defineProps<{ menus: Menu[]; base?: string }>()

// Buttons (type 3) never render; hidden menus (visible=0) keep their route
// but stay out of the sidebar. Empty dirs collapse away.
const visibleMenus = computed(() =>
  (props.menus ?? []).filter((m) => {
    if (m.type === 1) {
      return (m.children ?? []).some((child) =>
        child.type === 1
          ? (child.children ?? []).some((nested) => nested.type === 2 && nested.visible === 1)
          : child.type === 2 && child.visible === 1,
      )
    }
    return m.type === 2 && m.visible === 1
  }),
)

function fullPath(item: Menu): string {
  return joinPath(props.base ?? '', item.path)
}

// DB stores icon names like "setting" / "user"; map to the Element Plus
// PascalCase export, fall back to no icon for unknown names.
function iconOf(name: string) {
  if (!name) return null
  const key = name.charAt(0).toUpperCase() + name.slice(1)
  return (icons as Record<string, unknown>)[key] ?? null
}
</script>
