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

export interface Dept {
  id: number
  parentId: number
  name: string
  leader: string
  sort: number
  status: number
  children?: Dept[]
}

export interface DeptSavePayload {
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
}

export interface CustomerSavePayload {
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


