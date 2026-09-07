import type { LoginResponse, Service } from './types'

const TOKEN_KEY = 'localctl_token'

export class UnauthorizedError extends Error {
  constructor() {
    super('未登录或登录已过期')
    this.name = 'UnauthorizedError'
  }
}

export class ApiError extends Error {}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const res = await fetch(path, { ...init, headers })
  const data = await res.json().catch(() => ({}))
  if (res.status === 401) {
    setToken(null)
    window.dispatchEvent(new Event('localctl:unauthorized'))
    throw new UnauthorizedError()
  }
  if (!res.ok) {
    throw new ApiError((data as { error?: string }).error ?? res.statusText)
  }
  return data as T
}

export function login(password: string) {
  return request<LoginResponse>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ password }),
  })
}

export function fetchServices() {
  return request<{ services: Service[] }>('/api/services')
}

export function fetchServiceDetail(label: string, path?: string) {
  const qs = path ? `?path=${encodeURIComponent(path)}` : ''
  return request<{ service: Service }>(`/api/services/${encodeURIComponent(label)}${qs}`)
}

export type ActionOp =
  | 'start'
  | 'restart'
  | 'stop'
  | 'enable'
  | 'disable'
  | 'load'
  | 'unload'

export function runAction(label: string, op: ActionOp, path?: string) {
  return request<{ service: Service }>(`/api/services/${encodeURIComponent(label)}/actions`, {
    method: 'POST',
    body: JSON.stringify({ op, path }),
  })
}
