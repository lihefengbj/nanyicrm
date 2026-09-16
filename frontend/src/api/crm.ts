import { del, get, post, put } from './request'
import type {
  Contact,
  ContactSavePayload,
  Customer,
  CustomerSavePayload,
  FollowUp,
  FollowUpSavePayload,
  Contract,
  ContractSavePayload,
  DashboardSummary,
  Opportunity,
  OpportunitySavePayload,
  PageQuery,
  PageResult,
} from '@/types/api'

// ---- customer ----
export function listCustomers(query: PageQuery & { name?: string; status?: string; level?: string; mine?: string; ownerId?: number; tenantId?: number }) {
  return get<PageResult<Customer>>('/crm/customer', { ...query })
}
export function listAllCustomers(tenantId?: number) {
  return get<Customer[]>('/crm/customer/all', tenantId ? { tenantId } : {})
}
export function createCustomer(data: CustomerSavePayload) {
  return post<{ id: number }>('/crm/customer', data)
}
export function updateCustomer(id: number, data: CustomerSavePayload) {
  return put<void>(`/crm/customer/${id}`, data)
}
export function deleteCustomer(id: number) {
  return del<void>(`/crm/customer/${id}`)
}

// ---- contact ----
export function listContacts(query: PageQuery & { name?: string; customerId?: number }) {
  return get<PageResult<Contact>>('/crm/contact', { ...query })
}
export function createContact(data: ContactSavePayload) {
  return post<{ id: number }>('/crm/contact', data)
}
export function updateContact(id: number, data: ContactSavePayload) {
  return put<void>(`/crm/contact/${id}`, data)
}
export function deleteContact(id: number) {
  return del<void>(`/crm/contact/${id}`)
}

// ---- follow-up ----
export function listFollowUps(query: PageQuery & { customerId?: number; type?: string; mine?: string }) {
  return get<PageResult<FollowUp>>('/crm/follow', { ...query })
}
export function createFollowUp(data: FollowUpSavePayload) {
  return post<{ id: number }>('/crm/follow', data)
}
export function updateFollowUp(id: number, data: FollowUpSavePayload) {
  return put<void>(`/crm/follow/${id}`, data)
}
export function deleteFollowUp(id: number) {
  return del<void>(`/crm/follow/${id}`)
}

// ---- opportunity ----
export function listOpportunities(query: PageQuery & { name?: string; customerId?: number; stage?: string; mine?: string }) {
  return get<PageResult<Opportunity>>('/crm/opportunity', { ...query })
}
export function listAllOpportunities(customerId?: number) {
  return get<Opportunity[]>('/crm/opportunity/all', customerId ? { customerId } : {})
}
export function createOpportunity(data: OpportunitySavePayload) {
  return post<{ id: number }>('/crm/opportunity', data)
}
export function updateOpportunity(id: number, data: OpportunitySavePayload) {
  return put<void>(`/crm/opportunity/${id}`, data)
}
export function deleteOpportunity(id: number) {
  return del<void>(`/crm/opportunity/${id}`)
}

// ---- contract ----
export function listContracts(query: PageQuery & { keyword?: string; customerId?: number; status?: string; mine?: string }) {
  return get<PageResult<Contract>>('/crm/contract', { ...query })
}
export function createContract(data: ContractSavePayload) {
  return post<{ id: number }>('/crm/contract', data)
}
export function updateContract(id: number, data: ContractSavePayload) {
  return put<void>(`/crm/contract/${id}`, data)
}
export function deleteContract(id: number) {
  return del<void>(`/crm/contract/${id}`)
}

// ---- dashboard ----
export function dashboardSummary() {
  return get<DashboardSummary>('/dashboard/summary')
}
