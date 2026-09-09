import { get, post } from './request'
import type { TokenPair, UserInfo } from '@/types/api'

export function login(username: string, pwd: string) {
  return post<TokenPair>('/auth/login', { username, pwd })
}

export function logout(refreshToken: string) {
  return post<void>('/auth/logout', { refreshToken })
}

export function fetchProfile() {
  return get<UserInfo>('/auth/profile')
}
