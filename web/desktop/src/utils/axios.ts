import axios, { type AxiosRequestConfig } from 'axios'
import lang from '@/i18n/i18n'
import { router } from '@/router'
import type { ApiResponse } from '@/types/api'

const instance = axios.create({
  baseURL: import.meta.env.VITE_APP_URL,
  timeout: 60000,
  headers: {
    Lang: lang.lang
  }
})

instance.interceptors.request.use(
  (config) => {
    if (config.data instanceof FormData) {
      delete config.headers['Content-Type']
    } else if (config.method === 'POST') {
      config.data = JSON.stringify(config.data)
      config.headers['Content-Type'] = 'application/json;charset=UTF-8;'
    }
    return config
  },
  (error) => Promise.reject(error)
)

instance.interceptors.response.use(
  async (response) => {
    const data = response.data as ApiResponse
    if (data.errorNo === 403 && router.currentRoute.value.path !== '/login') {
      await router.replace({
        path: '/login',
        query: { redirect: router.currentRoute.value.fullPath }
      })
    }
    if (data.errorNo === 402 && router.currentRoute.value.path !== '/setup') {
      await router.replace({
        path: '/setup',
        query: { redirect: router.currentRoute.value.fullPath }
      })
    }
    return data as never
  },
  async (error) => {
    if (error.response?.status === 403 && router.currentRoute.value.path !== '/login') {
      await router.replace({
        path: '/login',
        query: { redirect: router.currentRoute.value.fullPath }
      })
    }
    return Promise.reject(error)
  }
)

export interface HttpClient {
  get<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  postForm<T = unknown>(url: string, data: FormData, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
}

export const http: HttpClient = {
  get<T = unknown>(url: string, config?: AxiosRequestConfig) {
    return instance.get(url, config) as unknown as Promise<ApiResponse<T>>
  },
  post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return instance.post(url, data, config) as unknown as Promise<ApiResponse<T>>
  },
  postForm<T = unknown>(url: string, data: FormData, config?: AxiosRequestConfig) {
    return instance.post(url, data, config) as unknown as Promise<ApiResponse<T>>
  }
}
