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
  CustomerIntent,
  CustomerIntentCompare,
  CustomerIntentFeedbackPayload,
  CustomerIntentHistory,
  CustomerIntentTask,
  IntentMetrics,
  IntentWorkbench,
  Opportunity,
  OpportunitySavePayload,
  PageQuery,
  PageResult,
} from '@/types/api'

// ---- customer ----
export function listCustomers(query: PageQuery & {
  name?: string
  status?: string
  level?: string
  mine?: string
  ownerId?: number
  tenantId?: number
  intentLevel?: string
  intentStatus?: string
  minScore?: number
  maxScore?: number
  minConfidence?: number
  followUpStatus?: string
  intentSort?: string
  analyzedWithin?: string
}) {
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
export function getCustomerIntent(id: number) {
  return get<CustomerIntent | null>(`/crm/customer/${id}/intent`)
}
export function analyzeCustomerIntent(id: number) {
  return post<CustomerIntent>(`/crm/customer/${id}/intent/analyze`)
}
export function retryCustomerIntent(id: number) {
  return post<CustomerIntent>(`/crm/customer/${id}/intent/retry`)
}
export function submitCustomerIntentFeedback(id: number, data: CustomerIntentFeedbackPayload) {
  return post<{ id: number }>(`/crm/customer/${id}/intent/feedback`, data)
}
export function compareCustomerIntent(id: number) {
  return get<CustomerIntentCompare>(`/crm/customer/${id}/intent/compare`)
}
export function listCustomerIntentHistory(id: number, query: PageQuery & { status?: string }) {
  return get<PageResult<CustomerIntentHistory>>(`/crm/customer/${id}/intent/history`, { ...query })
}
export function batchAnalyzeCustomerIntent(data: {
  customerIds?: number[]
  intentLevel?: string
  minScore?: number
  maxScore?: number
  followUpStatus?: string
  limit?: number
}) {
  return post<CustomerIntentTask>('/crm/customer/intent/batch', data)
}
export function getCustomerIntentTask(id: number) {
  return get<CustomerIntentTask>(`/crm/customer/intent/tasks/${id}`)
}
export function cancelCustomerIntentTask(id: number) {
  return post<CustomerIntentTask>(`/crm/customer/intent/tasks/${id}/cancel`)
}
export function getIntentWorkbench(tenantId?: number) {
  return get<IntentWorkbench>('/crm/intent/workbench', tenantId ? { tenantId } : {})
}
export function getIntentMetrics(params?: { from?: string; to?: string; tenantId?: number }) {
  return get<IntentMetrics>('/crm/intent/metrics', params)
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
