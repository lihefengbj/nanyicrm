<template>
  <div class="page">
    <div class="toolbar">
      <div>
        <h2>AI模型治理</h2>
        <p>候选配置必须通过固定样本质量门禁后才能灰度或激活。</p>
      </div>
      <div class="actions">
        <el-button @click="load">刷新</el-button>
        <el-button @click="credentialVisible = true">凭证管理</el-button>
        <el-button type="primary" @click="openCreate">新增候选</el-button>
        <el-button type="warning" plain @click="rollback">回滚上一个版本</el-button>
      </div>
    </div>

    <el-alert title="运行时使用数据库中的 active 模型配置和 Prompt；canary 配置按客户输入哈希稳定分流，单次调用版本以分析历史为准。" type="info" show-icon />

    <el-card class="effective-card" shadow="never">
      <template #header>
        <div class="effective-header">
          <strong>当前实际生效 Prompt</strong>
          <el-tag v-if="activePromptRow" type="success">主版本 {{ activePromptRow.version }}</el-tag>
          <el-tag v-else type="info">代码默认 v1</el-tag>
        </div>
      </template>
      <div class="effective-grid">
        <div>
          <span class="effective-label">主版本</span>
          <span>{{ activePromptRow?.version || 'v1（默认回退）' }}</span>
        </div>
        <div>
          <span class="effective-label">灰度版本</span>
          <span v-if="promptCanaryRow">{{ promptCanaryRow.version }} · {{ promptCanaryRow.canaryPercent }}%</span>
          <span v-else>未配置</span>
        </div>
        <div>
          <span class="effective-label">主模型配置</span>
          <span>{{ activeModelRow?.configVersion || '未配置' }}</span>
        </div>
        <div>
          <span class="effective-label">说明</span>
          <span>灰度命中时使用灰度 Prompt，否则使用主版本</span>
        </div>
      </div>
    </el-card>

    <el-table :data="rows" border stripe class="table">
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column prop="provider" label="Provider" width="140" />
      <el-table-column prop="model" label="模型" min-width="180" />
      <el-table-column label="API凭证" min-width="180">
        <template #default="{ row }">{{ credentialLabel(row.credentialId) }}</template>
      </el-table-column>
      <el-table-column prop="configVersion" label="配置版本" width="150" />
      <el-table-column prop="promptVersion" label="回退Prompt版本" width="140" />
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
          <el-button
            v-if="row.status !== 'active' && row.status !== 'canary'"
            link
            type="warning"
            @click="openCredentialBinding(row)"
          >绑定凭证</el-button>
          <el-button link type="primary" @click="gate(row)">质量门禁</el-button>
          <el-button link @click="showChanges(row)">审计记录</el-button>
          <el-button v-if="row.qualityPassed && row.status !== 'active'" link type="warning" @click="canary(row)">灰度</el-button>
          <el-button v-if="row.qualityPassed && row.status !== 'active'" link type="success" @click="activate(row)">激活</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-card class="prompt-card" shadow="never">
      <template #header>
        <div class="prompt-header">
          <div>
            <strong>Prompt治理</strong>
            <span class="prompt-hint">动态配置仅承载研判规则，JSON 输出契约仍由代码固定校验。</span>
          </div>
          <div class="actions">
            <el-button @click="loadPrompts">刷新</el-button>
            <el-button type="warning" plain @click="rollbackPrompt">回滚上一个版本</el-button>
            <el-button type="primary" @click="openPromptCreate">新增Prompt版本</el-button>
          </div>
        </div>
      </template>
      <el-table :data="promptRows" border stripe>
        <el-table-column prop="name" label="名称" min-width="170" />
        <el-table-column prop="version" label="版本" width="150" />
        <el-table-column label="研判规则" min-width="320" show-overflow-tooltip>
          <template #default="{ row }">{{ row.content }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="promptStatusType(row.status)">{{ promptStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="门禁" width="90">
          <template #default="{ row }">
            <el-tag :type="row.qualityPassed ? 'success' : 'danger'">{{ row.qualityPassed ? '通过' : '未通过' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="灰度" width="80">
          <template #default="{ row }">{{ row.canaryPercent ? `${row.canaryPercent}%` : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" min-width="320">
          <template #default="{ row }">
            <el-button v-if="row.status === 'draft'" link type="primary" @click="openPromptEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="gatePrompt(row)">质量门禁</el-button>
            <el-button link @click="showPromptChanges(row)">审计记录</el-button>
            <el-button v-if="row.qualityPassed && row.status !== 'active'" link type="warning" @click="canaryPrompt(row)">灰度</el-button>
            <el-button v-if="row.qualityPassed && row.status !== 'active'" link type="success" @click="activatePrompt(row)">激活</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="新增AI模型候选配置" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="Provider"><el-input v-model="form.provider" /></el-form-item>
        <el-form-item label="BaseURL"><el-input v-model="form.baseUrl" /></el-form-item>
        <el-form-item label="模型"><el-input v-model="form.model" /></el-form-item>
        <el-form-item label="API凭证">
          <el-select v-model="form.credentialId" clearable placeholder="使用环境变量 LLM_API_KEY" style="width: 100%">
            <el-option :value="0" label="环境变量 LLM_API_KEY" />
            <el-option
              v-for="item in credentials.filter((credential) => credential.status === 1)"
              :key="item.id"
              :label="`${item.name}（${item.provider} · ****${item.keyLast4}）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="配置版本"><el-input v-model="form.configVersion" /></el-form-item>
        <el-form-item label="回退Prompt版本"><el-input v-model="form.promptVersion" /></el-form-item>
        <el-form-item label="最大Token"><el-input-number v-model="form.maxTokens" :min="1" /></el-form-item>
        <el-form-item label="温度"><el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="create">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="changesVisible" :title="`模型治理审计记录${selectedConfigName ? `：${selectedConfigName}` : ''}`" width="760px">
      <el-table :data="changes" border stripe>
        <el-table-column prop="action" label="动作" width="110" />
        <el-table-column prop="reason" label="原因" min-width="220" show-overflow-tooltip />
        <el-table-column prop="qualitySummary" label="质量摘要" min-width="260" show-overflow-tooltip />
        <el-table-column prop="actorId" label="操作人ID" width="100" />
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ formatBeijingTime(row.createdAt) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!changes.length" description="暂无审计记录" />
    </el-dialog>

    <el-dialog v-model="promptDialogVisible" :title="promptEditing ? '编辑Prompt草稿' : '新增Prompt版本'" width="720px" destroy-on-close>
      <el-alert
        title="版本号由研判规则内容哈希生成；仅修改名称不会生成新版本，请先调整研判规则内容。已通过门禁或已发布版本不可编辑。"
        type="info"
        :closable="false"
        style="margin-bottom: 16px"
      />
      <el-form :model="promptForm" label-width="100px">
        <el-form-item label="名称"><el-input v-model="promptForm.name" maxlength="64" /></el-form-item>
        <el-form-item label="研判规则">
          <el-input v-model="promptForm.content" type="textarea" :rows="12" maxlength="20000" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="promptDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="savePrompt">保存草稿</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="promptChangesVisible" :title="`Prompt审计记录${selectedPromptName ? `：${selectedPromptName}` : ''}`" width="820px">
      <el-table :data="promptChanges" border stripe>
        <el-table-column prop="action" label="动作" width="110" />
        <el-table-column prop="fromVersion" label="原版本" width="140" />
        <el-table-column prop="toVersion" label="目标版本" width="140" />
        <el-table-column prop="reason" label="原因" min-width="200" show-overflow-tooltip />
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ formatBeijingTime(row.createdAt) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!promptChanges.length" description="暂无审计记录" />
    </el-dialog>

    <el-dialog v-model="credentialVisible" title="AI凭证管理" width="720px">
      <el-alert
        title="API Key 仅在保存或轮换时提交，服务端加密存储，页面不会回显明文。"
        type="info"
        :closable="false"
        style="margin-bottom: 12px"
      />
      <el-button type="primary" @click="credentialCreateVisible = true">新增凭证</el-button>
      <el-table :data="credentials" border stripe style="margin-top: 12px">
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column prop="provider" label="Provider" width="140" />
        <el-table-column label="Key">
          <template #default="{ row }">****{{ row.keyLast4 }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button link type="primary" @click="rotateCredential(row)">轮换</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="credentialCreateVisible" title="新增AI凭证" width="520px" destroy-on-close>
      <el-form :model="credentialForm" label-width="100px">
        <el-form-item label="名称"><el-input v-model="credentialForm.name" maxlength="64" /></el-form-item>
        <el-form-item label="Provider"><el-input v-model="credentialForm.provider" maxlength="64" /></el-form-item>
        <el-form-item label="API Key"><el-input v-model="credentialForm.apiKey" type="password" show-password /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="credentialCreateVisible = false">取消</el-button>
        <el-button type="primary" @click="createCredential">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="credentialBindVisible" title="绑定AI凭证" width="520px" destroy-on-close>
      <el-alert
        title="绑定或解绑凭证后，当前配置会退回草稿状态，必须重新执行质量门禁。激活/灰度配置不能直接更换凭证。"
        type="warning"
        :closable="false"
        style="margin-bottom: 16px"
      />
      <el-form label-width="90px">
        <el-form-item label="模型">
          <el-text>{{ bindingRow?.name || '-' }}</el-text>
        </el-form-item>
        <el-form-item label="API凭证">
          <el-select v-model="bindingCredentialId" style="width: 100%">
            <el-option :value="0" label="环境变量 LLM_API_KEY" />
            <el-option
              v-for="item in credentials.filter((credential) => credential.status === 1)"
              :key="item.id"
              :label="`${item.name}（${item.provider} · ****${item.keyLast4}）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="credentialBindVisible = false">取消</el-button>
        <el-button type="primary" @click="bindCredential">保存绑定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'SystemAIModel' })

import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  activateAIModel,
  bindAIModelCredential,
  createAIModelConfig,
  createAICredential,
  listAIModelChanges,
  listAICredentials,
  listAIModelConfigs,
  rollbackAIModel,
  runAIModelQualityGate,
  setAIModelCanary,
  rotateAICredential,
  activateAIPrompt,
  createAIPrompt,
  listAIPromptChanges,
  listAIPrompts,
  rollbackAIPrompt,
  runAIPromptQualityGate,
  setAIPromptCanary,
  updateAIPrompt,
} from '@/api/crm'
import type { AICredential, AIModelChange, AIModelConfig, AIPrompt, AIPromptChange } from '@/types/api'
import { formatBeijingTime } from '@/utils/datetime'

const rows = ref<AIModelConfig[]>([])
const changes = ref<AIModelChange[]>([])
const dialogVisible = ref(false)
const changesVisible = ref(false)
const credentialVisible = ref(false)
const credentialCreateVisible = ref(false)
const credentialBindVisible = ref(false)
const selectedConfigName = ref('')
const credentials = ref<AICredential[]>([])
const promptRows = ref<AIPrompt[]>([])
const promptChanges = ref<AIPromptChange[]>([])
const bindingRow = ref<AIModelConfig | null>(null)
const bindingCredentialId = ref(0)
const promptDialogVisible = ref(false)
const promptChangesVisible = ref(false)
const promptEditing = ref(false)
const selectedPromptName = ref('')
const editingPromptId = ref(0)
const form = reactive({
  name: '',
  provider: 'openai-compatible',
  baseUrl: '',
  model: '',
  credentialId: 0,
  configVersion: '',
  promptVersion: 'v1',
  responseFormat: 'json_object',
  thinkingMode: 'disabled',
  maxTokens: 1200,
  temperature: 0.2,
})
const credentialForm = reactive({
  name: '',
  provider: 'deepseek',
  apiKey: '',
})
const promptForm = reactive({
  name: '',
  content: '',
})
const activePromptRow = computed(() => promptRows.value.find((row) => row.status === 'active'))
const promptCanaryRow = computed(() => promptRows.value.find((row) => row.status === 'canary'))
const activeModelRow = computed(() => rows.value.find((row) => row.status === 'active'))

async function load() {
  rows.value = (await listAIModelConfigs({ pageNum: 1, pageSize: 100 })).records
}
async function loadCredentials() {
  credentials.value = await listAICredentials()
}
async function loadPrompts() {
  promptRows.value = (await listAIPrompts({ pageNum: 1, pageSize: 100 })).records
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
async function createCredential() {
  await createAICredential(credentialForm)
  credentialForm.name = ''
  credentialForm.apiKey = ''
  credentialCreateVisible.value = false
  ElMessage.success('AI凭证已保存')
  await loadCredentials()
}
function openPromptCreate() {
  promptEditing.value = false
  editingPromptId.value = 0
  promptForm.name = ''
  promptForm.content = ''
  promptDialogVisible.value = true
}
function openPromptEdit(row: AIPrompt) {
  promptEditing.value = true
  editingPromptId.value = row.id
  promptForm.name = row.name
  promptForm.content = row.content
  promptDialogVisible.value = true
}
async function savePrompt() {
  if (promptEditing.value) {
    await updateAIPrompt(editingPromptId.value, promptForm)
  } else {
    await createAIPrompt(promptForm)
  }
  promptDialogVisible.value = false
  ElMessage.success('Prompt草稿已保存')
  await loadPrompts()
}
async function gatePrompt(row: AIPrompt) {
  const result = await runAIPromptQualityGate(row.id)
  ElMessage[result.passed ? 'success' : 'error'](result.prompt.qualitySummary || 'Prompt质量门禁完成')
  await loadPrompts()
}
async function showPromptChanges(row: AIPrompt) {
  selectedPromptName.value = row.name
  promptChanges.value = await listAIPromptChanges(row.id)
  promptChangesVisible.value = true
}
async function canaryPrompt(row: AIPrompt) {
  const value = await ElMessageBox.prompt('请输入灰度比例（1-99）', 'Prompt灰度发布', {
    inputValue: String(row.canaryPercent || 10),
    inputPattern: /^(?:[1-9]|[1-9][0-9])$/,
    inputErrorMessage: '请输入 1-99',
  })
  await setAIPromptCanary(row.id, Number(value.value), '管理端Prompt灰度发布')
  ElMessage.success('Prompt灰度配置已生效')
  await loadPrompts()
}
async function activatePrompt(row: AIPrompt) {
  await ElMessageBox.confirm(`确认激活 Prompt ${row.version}？`, 'Prompt切换', { type: 'warning' })
  await activateAIPrompt(row.id, '管理端Prompt激活')
  ElMessage.success('Prompt版本已激活')
  await loadPrompts()
}
async function rollbackPrompt() {
  await ElMessageBox.confirm('确认回滚到上一个激活的Prompt版本？', 'Prompt回滚', { type: 'warning' })
  await rollbackAIPrompt(undefined, '管理端Prompt回滚')
  ElMessage.success('Prompt已回滚')
  await loadPrompts()
}
function openCredentialBinding(row: AIModelConfig) {
  bindingRow.value = row
  bindingCredentialId.value = row.credentialId || 0
  credentialBindVisible.value = true
}
async function bindCredential() {
  if (!bindingRow.value) return
  await bindAIModelCredential(bindingRow.value.id, bindingCredentialId.value)
  credentialBindVisible.value = false
  ElMessage.success('凭证绑定已保存，请重新执行质量门禁')
  await load()
}
async function rotateCredential(row: AICredential) {
  const value = await ElMessageBox.prompt(`请输入「${row.name}」的新 API Key`, '轮换凭证', {
    inputType: 'password',
    showInput: true,
  })
  await rotateAICredential(row.id, value.value)
  ElMessage.success('AI凭证已轮换')
  await loadCredentials()
}
async function gate(row: AIModelConfig) {
  const result = await runAIModelQualityGate(row.id)
  ElMessage[result.passed ? 'success' : 'error'](result.config.qualitySummary || '质量门禁完成')
  await load()
}
async function showChanges(row: AIModelConfig) {
  selectedConfigName.value = row.name
  changes.value = await listAIModelChanges(row.id)
  changesVisible.value = true
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
function credentialLabel(id: number) {
  if (!id) return '环境变量 LLM_API_KEY'
  const item = credentials.value.find((credential) => credential.id === id)
  return item ? `${item.name}（****${item.keyLast4}）` : `凭证 #${id}`
}
function promptStatusText(status: AIPrompt['status']) {
  return status === 'active' ? '激活' : status === 'canary' ? '灰度' : status === 'approved' ? '已通过' : status === 'draft' ? '草稿' : '历史'
}
function promptStatusType(status: AIPrompt['status']) {
  return status === 'active' ? 'success' : status === 'canary' ? 'warning' : status === 'approved' ? 'primary' : status === 'draft' ? 'info' : ''
}
onMounted(async () => {
  await Promise.all([load(), loadCredentials(), loadPrompts()])
})
</script>

<style scoped>
.page { padding: 20px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { margin: 0 0 6px; }
p { margin: 0; color: #909399; font-size: 13px; }
.actions { display: flex; gap: 8px; }
.table { margin-top: 16px; }
.effective-card { margin-top: 16px; }
.effective-header { display: flex; align-items: center; gap: 10px; }
.effective-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 24px; color: #606266; font-size: 13px; }
.effective-label { display: inline-block; width: 92px; color: #909399; }
.prompt-card { margin-top: 20px; }
.prompt-header { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.prompt-hint { margin-left: 10px; color: #909399; font-size: 13px; font-weight: normal; }
</style>
