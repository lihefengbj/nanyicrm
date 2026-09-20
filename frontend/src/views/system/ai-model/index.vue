<template>
  <div class="page">
    <div class="toolbar">
      <div>
        <h2>AI模型治理</h2>
        <p>候选配置必须通过固定样本质量门禁后才能灰度或激活。</p>
      </div>
      <div class="actions">
        <el-button @click="load">刷新</el-button>
        <el-button type="primary" @click="openCreate">新增候选</el-button>
        <el-button type="warning" plain @click="rollback">回滚上一个版本</el-button>
      </div>
    </div>

    <el-alert title="运行时使用数据库中的 active 配置；canary 配置按客户输入哈希稳定分流。" type="info" show-icon />

    <el-table :data="rows" border stripe class="table">
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column prop="provider" label="Provider" width="140" />
      <el-table-column prop="model" label="模型" min-width="180" />
      <el-table-column prop="configVersion" label="配置版本" width="150" />
      <el-table-column prop="promptVersion" label="提示词版本" width="120" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="门禁" width="90">
        <template #default="{ row }">
          <el-tag :type="row.qualityPassed ? 'success' : 'danger'">{{ row.qualityPassed ? '通过' : '未通过' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="canaryPercent" label="灰度" width="80">
        <template #default="{ row }">{{ row.canaryPercent ? `${row.canaryPercent}%` : '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" fixed="right" min-width="300">
        <template #default="{ row }">
          <el-button link type="primary" @click="gate(row)">质量门禁</el-button>
          <el-button v-if="row.qualityPassed && row.status !== 'active'" link type="warning" @click="canary(row)">灰度</el-button>
          <el-button v-if="row.qualityPassed && row.status !== 'active'" link type="success" @click="activate(row)">激活</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="新增AI模型候选配置" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="Provider"><el-input v-model="form.provider" /></el-form-item>
        <el-form-item label="BaseURL"><el-input v-model="form.baseUrl" /></el-form-item>
        <el-form-item label="模型"><el-input v-model="form.model" /></el-form-item>
        <el-form-item label="配置版本"><el-input v-model="form.configVersion" /></el-form-item>
        <el-form-item label="提示词版本"><el-input v-model="form.promptVersion" /></el-form-item>
        <el-form-item label="最大Token"><el-input-number v-model="form.maxTokens" :min="1" /></el-form-item>
        <el-form-item label="温度"><el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="create">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'SystemAIModel' })

import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  activateAIModel,
  createAIModelConfig,
  listAIModelConfigs,
  rollbackAIModel,
  runAIModelQualityGate,
  setAIModelCanary,
} from '@/api/crm'
import type { AIModelConfig } from '@/types/api'

const rows = ref<AIModelConfig[]>([])
const dialogVisible = ref(false)
const form = reactive({
  name: '',
  provider: 'openai-compatible',
  baseUrl: '',
  model: '',
  configVersion: '',
  promptVersion: 'v1',
  responseFormat: 'json_object',
  thinkingMode: 'disabled',
  maxTokens: 1200,
  temperature: 0.2,
})

async function load() {
  rows.value = (await listAIModelConfigs({ pageNum: 1, pageSize: 100 })).records
}
function openCreate() {
  dialogVisible.value = true
}
async function create() {
  await createAIModelConfig(form)
  dialogVisible.value = false
  ElMessage.success('候选配置已创建，请执行质量门禁')
  await load()
}
async function gate(row: AIModelConfig) {
  const result = await runAIModelQualityGate(row.id)
  ElMessage[result.passed ? 'success' : 'error'](result.config.qualitySummary || '质量门禁完成')
  await load()
}
async function canary(row: AIModelConfig) {
  const value = await ElMessageBox.prompt('请输入灰度比例（1-99）', '发布灰度', {
    inputValue: String(row.canaryPercent || 10),
    inputPattern: /^(?:[1-9]|[1-9][0-9])$/,
    inputErrorMessage: '请输入 1-99',
  })
  await setAIModelCanary(row.id, Number(value.value), '管理端灰度发布')
  ElMessage.success('灰度配置已生效')
  await load()
}
async function activate(row: AIModelConfig) {
  await ElMessageBox.confirm(`确认激活 ${row.configVersion}？`, '模型切换', { type: 'warning' })
  await activateAIModel(row.id, '管理端激活')
  ElMessage.success('模型版本已激活')
  await load()
}
async function rollback() {
  await ElMessageBox.confirm('确认回滚到上一激活版本？', '模型回滚', { type: 'warning' })
  await rollbackAIModel(undefined, '管理端回滚')
  ElMessage.success('模型已回滚')
  await load()
}
function statusText(status: AIModelConfig['status']) {
  return status === 'active' ? '激活' : status === 'canary' ? '灰度' : status === 'approved' ? '已通过' : status === 'draft' ? '草稿' : '历史'
}
function statusType(status: AIModelConfig['status']) {
  return status === 'active' ? 'success' : status === 'canary' ? 'warning' : status === 'approved' ? 'primary' : status === 'draft' ? 'info' : ''
}
onMounted(load)
</script>

<style scoped>
.page { padding: 20px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { margin: 0 0 6px; }
p { margin: 0; color: #909399; font-size: 13px; }
.actions { display: flex; gap: 8px; }
.table { margin-top: 16px; }
</style>
