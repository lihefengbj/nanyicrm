import { del, get, post, put } from './request'
import type {
  Contact,
  ContactSavePayload,
  Customer,
  CustomerSavePayload,
  FollowUp,
  FollowUpSavePayload,
  PageQuery,
  PageResult,
} from '@/types/api'

// ---- customer ----
export function listCustomers(query: PageQuery & { name?: string; status?: string; level?: string; mine?: string; ownerId?: number }) {
  return get<PageResult<Customer>>('/crm/customer', { ...query })
}
export function listAllCustomers() {
  return get<Customer[]>('/crm/customer/all')
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
