<template>
  <div>
    <el-card>
      <el-form inline @submit.prevent>
        <el-form-item label="菜单名称">
          <el-input v-model="query.title" placeholder="模糊搜索" clearable style="width: 180px" @change="load" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" clearable placeholder="全部" style="width: 120px" @change="load">
            <el-option label="启用" value="1" />
            <el-option label="停用" value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button @click="toggleExpand">{{ expandAll ? '折叠全部' : '展开全部' }}</el-button>
          <el-button v-if="store.hasPerm('system:menu:create')" type="primary" plain :icon="Plus" @click="openDialog()">
            新增菜单
          </el-button>
        </el-form-item>
      </el-form>

      <el-table v-if="renderTable" v-loading="loading" :data="rows" row-key="id" border :default-expand-all="expandAll">
        <el-table-column prop="title" label="菜单名称" min-width="200" />
        <el-table-column label="图标" width="70" align="center">
          <template #default="{ row }">
            <el-icon v-if="row.icon"><component :is="row.icon" /></el-icon>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag :type="typeTag(row.type)">{{ typeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="路由路径" min-width="140" show-overflow-tooltip />
        <el-table-column prop="component" label="组件" min-width="160" show-overflow-tooltip />
        <el-table-column prop="perms" label="权限标识" min-width="160" show-overflow-tooltip />
        <el-table-column prop="sort" label="排序" width="70" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="可见" width="80">
          <template #default="{ row }">
            <el-tag v-if="row.type !== 3" :type="row.visible === 1 ? 'success' : 'warning'">
              {{ row.visible === 1 ? '显示' : '隐藏' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button v-if="store.hasPerm('system:menu:create') && row.type !== 3" link type="primary"
              @click="openDialog(undefined, row.id)">
              新增下级
            </el-button>
            <el-button v-if="store.hasPerm('system:menu:update')" link type="primary" @click="openDialog(row)">编辑</el-button>
            <el-button v-if="store.hasPerm('system:menu:delete')" link type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑菜单' : '新增菜单'" width="560px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="上级菜单">
          <el-tree-select v-model="form.parentId" :data="menuOptions" check-strictly :render-after-expand="false"
            :props="{ label: 'title', value: 'id', children: 'children', disabled: 'disabled' }" clearable
            placeholder="留空为根节点" style="width: 100%" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="form.type">
            <el-radio :value="1">目录</el-radio>
            <el-radio :value="2">菜单</el-radio>
            <el-radio :value="3">按钮</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="名称" prop="title">
          <el-input v-model="form.title" />
        </el-form-item>
        <el-form-item v-if="form.type !== 3" label="路由路径" prop="path">
          <el-input v-model="form.path" placeholder="目录以 / 开头，菜单填相对路径" />
        </el-form-item>
        <el-form-item v-if="form.type === 2" label="组件路径" prop="component">
          <el-input v-model="form.component" placeholder="如 system/menu/index" />
          <el-text size="small" type="info">对应 src/views/ 下的 .vue 文件；填错时页面显示「页面未实现」占位</el-text>
        </el-form-item>
        <el-form-item v-if="form.type !== 1" label="权限标识" prop="perms">
          <el-input v-model="form.perms" placeholder="如 system:menu:list" />
        </el-form-item>
        <el-form-item v-if="form.type !== 3" label="图标">
          <el-input v-model="form.icon" placeholder="Element Plus 图标名，如 setting" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" />
        </el-form-item>
        <el-form-item v-if="form.type !== 3" label="可见">
          <el-radio-group v-model="form.visible">
            <el-radio :value="1">显示</el-radio>
            <el-radio :value="0">隐藏</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">停用</el-radio>
          </el-radio-group>
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
defineOptions({ name: 'SystemMenu' })
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { menuTree, createMenu, updateMenu, deleteMenu } from '@/api/system'
import type { Menu } from '@/types/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const loading = ref(false)
const rows = ref<Menu[]>([])
const menuOptions = ref<Menu[]>([])
const query = reactive({ title: '', status: '' })
const expandAll = ref(true)
const renderTable = ref(true)

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({
  parentId: undefined as number | undefined,
  title: '',
  type: 2,
  path: '',
  component: '',
  perms: '',
  icon: '',
  sort: 0,
  visible: 1,
  status: 1,
})

const formRules: FormRules = {
  title: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  path: [{ required: true, message: '请输入路由路径', trigger: 'blur' }],
  component: [{ required: true, message: '请输入组件路径', trigger: 'blur' }],
  perms: [{ required: true, message: '请输入权限标识', trigger: 'blur' }],
}

function typeLabel(t: number) {
  return t === 1 ? '目录' : t === 2 ? '菜单' : '按钮'
}
function typeTag(t: number) {
  return t === 1 ? 'warning' : t === 2 ? 'primary' : 'info'
}

// Buttons (type 3) cannot be parents; disable them in the parent picker.
function markButtonDisabled(list: Menu[]) {
  for (const m of list) {
    ;(m as Menu & { disabled?: boolean }).disabled = m.type === 3
    if (m.children) markButtonDisabled(m.children)
  }
}

async function load() {
  loading.value = true
  try {
    const data = await menuTree({ title: query.title || undefined, status: query.status || undefined })
    markButtonDisabled(data)
    rows.value = data
    menuOptions.value = data
  } finally {
    loading.value = false
  }
}

function toggleExpand() {
  expandAll.value = !expandAll.value
  // el-table only reads default-expand-all on mount; force a re-render.
  renderTable.value = false
  requestAnimationFrame(() => {
    renderTable.value = true
  })
}

function openDialog(row?: Menu, parentId?: number) {
  editingId.value = row?.id ?? null
  form.parentId = row?.parentId ?? parentId ?? undefined
  if (form.parentId === 0) form.parentId = undefined
  form.title = row?.title ?? ''
  form.type = row?.type ?? 2
  form.path = row?.path ?? ''
  form.component = row?.component ?? ''
  form.perms = row?.perms ?? ''
  form.icon = row?.icon ?? ''
  form.sort = row?.sort ?? 0
  form.visible = row?.visible ?? 1
  form.status = row?.status ?? 1
  dialogVisible.value = true
}

async function onSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  const payload = {
    parentId: form.parentId ?? 0,
    title: form.title,
    type: form.type,
    path: form.type === 3 ? '' : form.path,
    component: form.type === 2 ? form.component : '',
    perms: form.perms,
    icon: form.type === 3 ? '' : form.icon,
    sort: form.sort,
    visible: form.visible,
    status: form.status,
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateMenu(editingId.value, payload)
      ElMessage.success('已更新')
    } else {
      await createMenu(payload)
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    load()
  } catch {
    // interceptor shows the message
  } finally {
    saving.value = false
  }
}

async function onDelete(row: Menu) {
  await ElMessageBox.confirm(`确认删除「${row.title}」？`, '提示', { type: 'warning' })
  await deleteMenu(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
