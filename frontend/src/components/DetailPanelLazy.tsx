import { useEffect, useState } from 'react'
import { fetchServiceDetail } from '../api'
import type { Service } from '../types'
import { DetailPanel } from './DetailPanel'

export function DetailPanelLazy({ service }: { service: Service }) {
  const [detail, setDetail] = useState<Service | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchServiceDetail(service.label, service.plist_path)
      .then((d) => {
        if (!cancelled) setDetail(d.service)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      })
    return () => {
      cancelled = true
    }
  }, [service.label, service.plist_path])

  if (error) return <div className="detail-body"><div className="detail-err">加载失败：{error}</div></div>
  if (!detail) return <div className="detail-body"><div className="detail-err">加载中…</div></div>
  return <DetailPanel service={detail} />
}
