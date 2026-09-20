import axios, { AxiosError, type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'
import type { ApiResponse } from '@/types/api'

const request = axios.create({
  baseURL: '/api/v1',
  // AI analysis is synchronous for the current phase. Keep this longer than
  // the backend LLM timeout so a slow but valid provider response is not
  // reported as a frontend timeout first.
  timeout: 60000,
  withCredentials: true,
})

let refreshing: Promise<boolean> | null = null

// Business codes that mean "not authenticated": backend returns them with
// HTTP 200, so the axios error branch (which expects 401) never fires.
const AUTH_CODES = new Set([2001, 2002])

async function tryRefresh(): Promise<boolean> {
  try {
    const resp = await axios.post<ApiResponse<{ authenticated: boolean }>>(
      '/api/v1/auth/refresh',
      undefined,
      { withCredentials: true },
    )
    if (resp.data.code === 0 && resp.data.data?.authenticated) {
      useUserStore().setAuthenticated()
      return true
    }
  } catch {
    // fall through
  }
  return false
}

function isSessionEndpoint(url?: string) {
  return /\/auth\/(?:login|refresh|logout)(?:$|[?#])/.test(url ?? '')
}

async function handleAuthFailure(
  config: AxiosRequestConfig & { _retried?: boolean },
): Promise<AxiosResponse> {
  if (!config._retried && !isSessionEndpoint(config.url)) {
    config._retried = true
    refreshing = refreshing ?? tryRefresh()
    const ok = await refreshing
    refreshing = null
    if (ok) {
      return request(config)
    }
  }
  const store = useUserStore()
  store.logout()
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
  return Promise.reject(new AxiosError('登录已过期'))
}

request.interceptors.request.use((config) => {
  return config
})

request.interceptors.response.use(
  (response) => {
    const body = response.data as ApiResponse
    if (body.code === 0) {
      return response
    }
    if (AUTH_CODES.has(body.code)) {
      return handleAuthFailure(response.config as AxiosRequestConfig & { _retried?: boolean })
    }
    ElMessage.error(body.message || '请求失败')
    return Promise.reject(new Error(body.message))
  },
  async (error: AxiosError<ApiResponse>) => {
    const config = error.config as AxiosRequestConfig & { _retried?: boolean }

    if (config && error.response?.status === 401 && !config.url?.includes('/auth/login')) {
      return handleAuthFailure(config)
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
