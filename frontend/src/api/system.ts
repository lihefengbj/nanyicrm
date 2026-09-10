import { del, get, post, put } from './request'
import type {
  Dept,
  DeptSavePayload,
  LoginLog,
  Menu,
  OperLog,
  PageQuery,
  PageResult,
  Role,
  RoleSavePayload,
  Tenant,
  TenantSavePayload,
  User,
  UserSavePayload,
} from '@/types/api'

// ---- user ----
export function listUsers(query: PageQuery & { username?: string; status?: string; tenantId?: number }) {
  return get<PageResult<User>>('/system/user', { ...query })
}
export function createUser(data: UserSavePayload) {
  return post<{ id: number }>('/system/user', data)
}
export function updateUser(id: number, data: UserSavePayload) {
  return put<void>(`/system/user/${id}`, data)
}
export function deleteUser(id: number) {
  return del<void>(`/system/user/${id}`)
}

// ---- role ----
export function listRoles(query: PageQuery & { name?: string }) {
  return get<PageResult<Role>>('/system/role', { ...query })
}
export function listAllRoles(tenantId?: number) {
  return get<Role[]>('/system/role/all', tenantId ? { tenantId } : {})
}
export function createRole(data: RoleSavePayload) {
  return post<{ id: number }>('/system/role', data)
}
export function updateRole(id: number, data: RoleSavePayload) {
  return put<void>(`/system/role/${id}`, data)
}
export function deleteRole(id: number) {
  return del<void>(`/system/role/${id}`)
}

// ---- menu ----
export function menuTree() {
  return get<Menu[]>('/system/menu/tree')
}

// ---- dept ----
export function deptTree(tenantId?: number) {
  return get<Dept[]>('/system/dept/tree', tenantId ? { tenantId } : {})
}
export function createDept(data: DeptSavePayload) {
  return post<{ id: number }>('/system/dept', data)
}
export function updateDept(id: number, data: DeptSavePayload) {
  return put<void>(`/system/dept/${id}`, data)
}
export function deleteDept(id: number) {
  return del<void>(`/system/dept/${id}`)
}

// ---- logs ----
export function listOperLogs(query: PageQuery & { username?: string; tenantId?: number }) {
  return get<PageResult<OperLog>>('/system/log/oper', { ...query })
}
export function listLoginLogs(query: PageQuery & { username?: string; tenantId?: number }) {
  return get<PageResult<LoginLog>>('/system/log/login', { ...query })
}

// ---- tenant (super admin only) ----
export function listTenants(query: PageQuery & { keyword?: string; status?: string }) {
  return get<PageResult<Tenant>>('/system/tenant', { ...query })
}
export function listAllTenants() {
  return get<Tenant[]>('/system/tenant/all')
}
export function createTenant(data: TenantSavePayload) {
  return post<{ id: number }>('/system/tenant', data)
}
export function updateTenant(id: number, data: TenantSavePayload) {
  return put<void>(`/system/tenant/${id}`, data)
}
export function deleteTenant(id: number) {
  return del<void>(`/system/tenant/${id}`)
}


