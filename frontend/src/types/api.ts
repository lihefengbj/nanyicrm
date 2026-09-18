export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data?: T
}

export interface PageResult<T> {
  records: T[]
  total: number
  pageNum: number
  pageSize: number
}

export interface PageQuery {
  pageNum: number
  pageSize: number
}

export interface TokenPair {
  accessToken: string
  refreshToken: string
  expiresIn: number
}

export interface TenantBrief {
  id: number
  code: string
  name: string
}

export interface UserInfo {
  id: number
  username: string
  nickname: string
  email: string
  phone: string
  tenantId: number
  tenant?: TenantBrief
  isSuper: boolean
  isPrivileged: boolean
  dept?: Dept
  roles: string[]
  perms: string[]
  menus: Menu[]
  isAdmin: boolean
}

export interface Tenant {
  id: number
  code: string
  name: string
  contact: string
  phone: string
  expireAt?: string
  status: number
  remark: string
  createdAt: string
}

export interface TenantSavePayload {
  code: string
  name: string
  contact: string
  phone: string
  expireAt?: string
  status: number
  remark: string
}

export interface User {
  id: number
  username: string
  nickname: string
  email: string
  phone: string
  deptId?: number
  tenantId: number
  tenant?: Tenant
  status: number
  remark: string
  dept?: Dept
  roles?: Role[]
  createdAt: string
}

export interface UserSavePayload {
  username: string
  pwd?: string
  nickname: string
  email: string
  phone: string
  deptId?: number
  tenantId?: number
  status: number
  remark: string
  roleIds: number[]
}

export interface Role {
  id: number
  name: string
  code: string
  sort: number
  status: number
  remark: string
  menus?: Menu[]
  createdAt: string
}

export interface RoleSavePayload {
  name: string
  code: string
  sort: number
  status: number
  remark: string
  menuIds: number[]
}

export interface Menu {
  id: number
  parentId: number
  title: string
  type: number // 1 dir, 2 menu, 3 button
  path: string
  component: string
  perms: string
  icon: string
  sort: number
  visible: number
  status: number
  children?: Menu[]
}

export interface MenuSavePayload {
  parentId: number
  title: string
  type: number
  path: string
  component: string
  perms: string
  icon: string
  sort: number
  visible: number
  status: number
}

export interface ApiInfo {
  id: number
  method: string
  path: string
  handler: string
  title: string
  module: string
  perms: string
  status: number
  updatedAt: string
}

export interface Dept {
  id: number
  tenantId: number
  parentId: number
  name: string
  leader: string
  sort: number
  status: number
  children?: Dept[]
}

export interface DeptSavePayload {
  tenantId?: number
  parentId: number
  name: string
  leader: string
  sort: number
  status: number
}

export interface OperLog {
  id: number
  tenantId: number
  userId: number
  username: string
  module: string
  action: string
  method: string
  path: string
  ip: string
  status: number
  errorMsg: string
  costMillis: number
  createdAt: string
}

export interface LoginLog {
  id: number
  tenantId: number
  username: string
  ip: string
  userAgent: string
  success: boolean
  message: string
  createdAt: string
}

// ---- CRM ----

export interface Customer {
  id: number
  tenantId: number
  name: string
  phone: string
  source: string
  industry: string
  level: string
  status: number // 1 跟进中, 2 已成交, 3 已流失
  ownerId?: number
  owner?: User
  address: string
  remark: string
  createdAt: string
  intent?: CustomerIntent | null
}

export interface CustomerSavePayload {
  tenantId?: number
  name: string
  phone: string
  source: string
  industry: string
  level: string
  status: number
  ownerId?: number
  address: string
  remark: string
}

export interface Contact {
  id: number
  tenantId: number
  customerId: number
  name: string
  phone: string
  email: string
  position: string
  isPrimary: number
  remark: string
  customer?: Customer
  createdAt: string
}

export interface ContactSavePayload {
  customerId: number
  name: string
  phone: string
  email: string
  position: string
  isPrimary: number
  remark: string
}

export interface FollowUp {
  id: number
  tenantId: number
  customerId: number
  contactId?: number
  type: number // 1 电话, 2 拜访, 3 会议, 4 其他
  content: string
  nextAt?: string
  creatorId: number
  creator: string
  customer?: Customer
  contact?: Contact
  createdAt: string
}

export interface FollowUpSavePayload {
  customerId: number
  contactId?: number
  type: number
  content: string
  nextAt?: string
}

export interface CustomerIntent {
  id: number
  tenantId: number
  customerId: number
  intentLevel: 'high' | 'medium' | 'low' | 'unknown'
  intentScore?: number
  confidence?: number
  summary: string
  needs: string[]
  painPoints: string[]
  budget: string
  purchaseTimeline: string
  decisionRole: string
  risks: string[]
  nextAction: string
  suggestedNextAt?: string
  analyzedAt: string
  provider: string
  model: string
  promptVersion: string
  status: string
  manualOverride?: boolean
  manualIntentLevel?: 'high' | 'medium' | 'low' | 'unknown' | ''
  followUpStatus?: 'today' | 'overdue' | 'future' | 'none'
  createdAt: string
  updatedAt: string
}

export interface CustomerIntentHistory {
  id: number
  customerId: number
  triggerUserId: number
  result?: CustomerIntent | null
  status: string
  errorMessage: string
  provider: string
  model: string
  promptVersion: string
  costMillis: number
  inputSummary: string
  analyzedAt?: string
  createdAt: string
}

export interface CustomerIntentFeedbackPayload {
  analysisId?: number
  feedbackType: 'accurate' | 'partial' | 'inaccurate'
  accepted?: boolean
  manualIntentLevel?: 'high' | 'medium' | 'low' | 'unknown'
  note?: string
}

export interface CustomerIntentCompare {
  current?: CustomerIntentHistory | null
  previous?: CustomerIntentHistory | null
  scoreDiff?: number | null
  levelChanged: boolean
}

export interface CustomerIntentTaskItem {
  id: number
  taskId: number
  customerId: number
  status: 'pending' | 'running' | 'success' | 'failed' | 'canceled'
  attempts: number
  errorMessage: string
  startedAt?: string
  finishedAt?: string
}

export interface CustomerIntentTask {
  id: number
  tenantId: number
  createdBy: number
  status: 'pending' | 'running' | 'success' | 'failed' | 'canceled'
  totalCount: number
  pendingCount: number
  runningCount: number
  successCount: number
  failedCount: number
  canceledCount: number
  maxAttempts: number
  errorMessage: string
  startedAt?: string
  finishedAt?: string
  createdAt: string
  items?: CustomerIntentTaskItem[]
}

export interface IntentWorkbenchCustomer {
  customerId: number
  customerName: string
  intentLevel: CustomerIntent['intentLevel']
  intentScore?: number
  suggestedNextAt?: string
  followUpStatus: 'today' | 'overdue' | 'future' | 'none'
  summary: string
  scoreDiff?: number
}

export interface IntentWorkbench {
  highIntentCount: number
  todayFollowUpCount: number
  overdueFollowUpCount: number
  failedAnalysisCount: number
  risingCustomers: IntentWorkbenchCustomer[]
  fallingCustomers: IntentWorkbenchCustomer[]
  failedCustomers: IntentWorkbenchCustomer[]
}

export interface IntentMetricGroup {
  provider: string
  model: string
  promptVersion: string
  count: number
}

export interface IntentFailureGroup {
  reason: string
  count: number
}

export interface IntentMetrics {
  from: string
  to: string
  requestCount: number
  successCount: number
  failedCount: number
  successRate: number
  averageCostMillis: number
  p95CostMillis: number
  byProvider: IntentMetricGroup[]
  failureReasons: IntentFailureGroup[]
  levelDistribution: Record<string, number>
  feedbackCount: number
  acceptedFeedback: number
  acceptanceRate: number
}

// ---- sales ----

export interface Opportunity {
  id: number
  tenantId: number
  customerId: number
  name: string
  stage: number // 1 初步接触 2 需求确认 3 方案报价 4 商务谈判 5 赢单 6 输单
  amount: number
  expectDate?: string
  ownerId?: number
  remark: string
  customer?: Customer
  owner?: User
  createdAt: string
}

export interface OpportunitySavePayload {
  customerId: number
  name: string
  stage: number
  amount: number
  expectDate?: string
  ownerId?: number
  remark: string
}

export interface Contract {
  id: number
  tenantId: number
  code: string
  name: string
  customerId: number
  opportunityId?: number
  amount: number
  signDate?: string
  startDate?: string
  endDate?: string
  status: number // 1 草稿 2 履行中 3 已完成 4 已作废
  ownerId?: number
  remark: string
  customer?: Customer
  opportunity?: Opportunity
  owner?: User
  createdAt: string
}

export interface ContractSavePayload {
  code: string
  name: string
  customerId: number
  opportunityId?: number
  amount: number
  signDate?: string
  startDate?: string
  endDate?: string
  status: number
  ownerId?: number
  remark: string
}

export interface DashboardSummary {
  customerTotal: number
  myCustomerTotal: number
  openOppCount: number
  openOppAmount: number
  contractTotal: number
  contractAmount: number
  followWeekCount: number
  pendingFollowCount: number
  opportunityStages: { stage: number; count: number; total: number }[]
}

// ---- dict ----

export interface DictItem {
  id: number
  dictId: number
  label: string
  value: string
  sort: number
  status: number
}

export interface Dict {
  id: number
  tenantId: number
  name: string
  type: string
  status: number
  remark: string
  items?: DictItem[]
  createdAt: string
}

export interface DictSavePayload {
  name: string
  type: string
  status: number
  remark: string
}

export interface DictItemSavePayload {
  dictId: number
  label: string
  value: string
  sort: number
  status: number
}


