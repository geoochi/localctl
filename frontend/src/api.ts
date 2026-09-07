import type { Service } from './types'

export class ApiError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new ApiError((data as { error?: string }).error ?? res.statusText)
  }
  return data as T
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
