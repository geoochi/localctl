import type { CreatePlistRequest, CronEntry, PlistSource, Service } from './types'

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
  | 'delete'

export function runAction(label: string, op: ActionOp, path?: string) {
  return request<{ service: Service }>(`/api/services/${encodeURIComponent(label)}/actions`, {
    method: 'POST',
    body: JSON.stringify({ op, path }),
  })
}

export function fetchCron() {
  return request<{ entries: CronEntry[] }>('/api/cron')
}

export function importCronEntry(index: number) {
  return request<{ service: Service }>(`/api/cron/${index}/import`, { method: 'POST' })
}

export function fetchSource(label: string, path: string) {
  return request<PlistSource>(
    `/api/services/${encodeURIComponent(label)}/source?path=${encodeURIComponent(path)}`,
  )
}

export function createPlist(req: CreatePlistRequest) {
  return request<{ service: Service }>('/api/plist', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}
