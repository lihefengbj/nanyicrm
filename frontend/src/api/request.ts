import axios, { AxiosError, type AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'
import type { ApiResponse, TokenPair } from '@/types/api'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

let refreshing: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  const store = useUserStore()
  if (!store.refreshToken) return false
  try {
    const resp = await axios.post<ApiResponse<TokenPair>>('/api/v1/auth/refresh', {
      refreshToken: store.refreshToken,
    })
    if (resp.data.code === 0 && resp.data.data) {
      store.setTokens(resp.data.data)
      return true
    }
  } catch {
    // fall through
  }
  return false
}

request.interceptors.request.use((config) => {
  const store = useUserStore()
  if (store.accessToken) {
    config.headers.Authorization = `Bearer ${store.accessToken}`
  }
  return config
})

request.interceptors.response.use(
  (response) => {
    const body = response.data as ApiResponse
    if (body.code === 0) {
      return response
    }
    ElMessage.error(body.message || '请求失败')
    return Promise.reject(new Error(body.message))
  },
  async (error: AxiosError<ApiResponse>) => {
    const status = error.response?.status
    const config = error.config as AxiosRequestConfig & { _retried?: boolean }

    if (status === 401 && config && !config._retried && !config.url?.includes('/auth/')) {
      config._retried = true
      refreshing = refreshing ?? tryRefresh()
      const ok = await refreshing
      refreshing = null
      if (ok) {
        return request(config)
      }
      const store = useUserStore()
      store.logout()
      window.location.href = '/login'
      return Promise.reject(error)
    }

    const msg = error.response?.data?.message || error.message || '网络异常'
    ElMessage.error(msg)
    return Promise.reject(error)
  },
)

export async function get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const resp = await request.get<ApiResponse<T>>(url, { params })
  return resp.data.data as T
}

export async function post<T>(url: string, data?: unknown): Promise<T> {
  const resp = await request.post<ApiResponse<T>>(url, data)
  return resp.data.data as T
}

export async function put<T>(url: string, data?: unknown): Promise<T> {
  const resp = await request.put<ApiResponse<T>>(url, data)
  return resp.data.data as T
}

export async function del<T>(url: string): Promise<T> {
  const resp = await request.delete<ApiResponse<T>>(url)
  return resp.data.data as T
}
