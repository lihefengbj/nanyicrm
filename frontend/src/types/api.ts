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


