<template>
  <el-result icon="warning" title="页面未实现">
    <template #sub-title>
      <p>该菜单配置的组件路径在前端不存在，请联系管理员检查菜单的「组件路径」配置。</p>
    </template>
  </el-result>
</template>

<script setup lang="ts">
defineOptions({ name: 'NotImplemented' })
</script>
