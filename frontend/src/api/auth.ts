import { get, post } from './request'
import type { SessionResponse, UserInfo } from '@/types/api'

export function login(username: string, pwd: string) {
  return post<SessionResponse>('/auth/login', { username, pwd })
}

export function logout() {
  return post<void>('/auth/logout')
}

export function fetchProfile() {
  return get<UserInfo>('/auth/profile')
}
