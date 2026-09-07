import { useCallback, useEffect, useState } from 'react'
import { fetchServices } from '../api'
import type { Service } from '../types'

// 每 5 秒轮询一次服务列表；操作后可调用 refresh 立即刷新。
export function useServices(intervalMs = 5000) {
  const [services, setServices] = useState<Service[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchServices()
      setServices(data.services)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void refresh()
    const id = setInterval(() => void refresh(), intervalMs)
    return () => clearInterval(id)
  }, [refresh, intervalMs])

  return { services, error, refresh }
}
