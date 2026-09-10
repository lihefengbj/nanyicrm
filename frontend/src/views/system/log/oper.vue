<template>
  <el-card>
    <el-form inline @submit.prevent>
      <el-form-item label="操作人">
        <el-input v-model="query.username" placeholder="模糊搜索" clearable style="width: 180px" @keyup.enter="load" />
      </el-form-item>
      <el-form-item v-if="isSuper" label="租户">
        <el-select v-model="query.tenantId" placeholder="全部" clearable style="width: 160px" @change="load">
          <el-option v-for="t in tenantOptions" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :icon="Search" @click="load">查询</el-button>
      </el-form-item>
    </el-form>

    <el-table v-loading="loading" :data="rows" border stripe>
      <el-table-column prop="username" label="操作人" width="120" />
      <el-table-column prop="module" label="模块" width="120" />
      <el-table-column prop="action" label="操作" width="140" />
      <el-table-column prop="method" label="方法" width="80" />
      <el-table-column prop="path" label="路径" min-width="200" show-overflow-tooltip />
      <el-table-column prop="ip" label="IP" width="130" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status < 400 ? 'success' : 'danger'">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="costMillis" label="耗时(ms)" width="90" />
      <el-table-column prop="createdAt" label="时间" width="180">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
    </el-table>

    <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
      :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" class="pager"
      @current-change="load" @size-change="load" />
  </el-card>
</template>

<script setup lang="ts">
defineOptions({ name: 'SystemOperLog' })
import { computed, onMounted, reactive, ref } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { listOperLogs, listAllTenants } from '@/api/system'
import type { OperLog, Tenant } from '@/types/api'
import { useUserStore } from '@/store/user'

const loading = ref(false)
const rows = ref<OperLog[]>([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, username: '', tenantId: undefined as number | undefined })
const store = useUserStore()
const isSuper = computed(() => store.profile?.isPrivileged ?? false)
const tenantOptions = ref<Tenant[]>([])

async function loadTenants() {
  if (!isSuper.value) return
  try {
    tenantOptions.value = await listAllTenants()
  } catch {
    tenantOptions.value = []
  }
}

function formatTime(t: string) {
  return t ? new Date(t).toLocaleString('zh-CN') : '-'
}

async function load() {
  loading.value = true
  try {
    const data = await listOperLogs(query)
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  loadTenants()
})
</script>

<style scoped>
.pager {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>





