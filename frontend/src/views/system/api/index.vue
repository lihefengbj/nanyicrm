<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="方法">
          <el-select v-model="query.method" clearable placeholder="全部" style="width: 110px" @change="load">
            <el-option v-for="m in ['GET', 'POST', 'PUT', 'DELETE']" :key="m" :label="m" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="路径">
          <el-input v-model="query.path" placeholder="模糊搜索" clearable style="width: 220px" @change="load" />
        </el-form-item>
        <el-form-item label="模块">
          <el-select v-model="query.module" clearable placeholder="全部" style="width: 130px" @change="load">
            <el-option v-for="m in modules" :key="m" :label="m" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button v-if="store.hasPerm('system:api:update')" :loading="syncing" @click="onSync">手动同步</el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column label="方法" width="90">
          <template #default="{ row }">
            <el-tag :type="methodTag(row.method)">{{ row.method }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="路径" min-width="240" show-overflow-tooltip />
        <el-table-column prop="title" label="接口名称" min-width="140">
          <template #default="{ row }">{{ row.title || '—' }}</template>
        </el-table-column>
        <el-table-column prop="module" label="模块" width="110" />
        <el-table-column label="所需权限" min-width="170" show-overflow-tooltip>
          <template #default="{ row }">{{ row.perms || '公开（登录即可）' }}</template>
        </el-table-column>
        <el-table-column prop="updatedAt" label="更新时间" width="170" />
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('system:api:update')" link type="primary" @click="openDialog(row)">
              编辑名称
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total"
        layout="total, prev, pager, next, sizes" :page-sizes="[10, 20, 50, 100]" style="margin-top: 12px"
        @change="load" />
    </el-card>

    <el-dialog v-model="dialogVisible" title="编辑接口名称" width="440px" destroy-on-close>
      <el-form label-width="90px">
        <el-form-item label="接口">
          <el-text>{{ editing?.method }} {{ editing?.path }}</el-text>
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="titleForm" maxlength="64" placeholder="如：用户-编辑" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'SystemApi' })
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listApis, updateApiTitle, syncApis } from '@/api/system'
import type { ApiInfo } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const loading = ref(false)
const syncing = ref(false)
const rows = ref<ApiInfo[]>([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 20, method: '', path: '', module: '' })

const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref<ApiInfo | null>(null)
const titleForm = ref('')

const modules = computed(() => [...new Set(rows.value.map((r) => r.module))].sort())

function methodTag(m: string) {
  return m === 'GET' ? 'success' : m === 'POST' ? 'primary' : m === 'PUT' ? 'warning' : 'danger'
}

async function load() {
  loading.value = true
  try {
    const data = await listApis({
      pageNum: query.pageNum,
      pageSize: query.pageSize,
      method: query.method || undefined,
      path: query.path || undefined,
      module: query.module || undefined,
    })
    rows.value = data.records
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function openDialog(row: ApiInfo) {
  editing.value = row
  titleForm.value = row.title
  dialogVisible.value = true
}

async function onSave() {
  if (!editing.value) return
  saving.value = true
  try {
    await updateApiTitle(editing.value.id, titleForm.value)
    ElMessage.success('已保存')
    dialogVisible.value = false
    load()
  } catch {
    // interceptor shows the message
  } finally {
    saving.value = false
  }
}

async function onSync() {
  syncing.value = true
  try {
    await syncApis()
    ElMessage.success('同步完成')
    load()
  } finally {
    syncing.value = false
  }
}

onMounted(load)
</script>
