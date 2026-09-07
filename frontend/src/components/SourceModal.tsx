import { useEffect, useState } from 'react'
import { fetchSource } from '../api'
import type { Service } from '../types'

export function SourceModal({ service, onClose }: { service: Service; onClose: () => void }) {
  const [content, setContent] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!service.plist_path) return
    let cancelled = false
    fetchSource(service.label, service.plist_path)
      .then((d) => {
        if (!cancelled) setContent(d.content)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      })
    return () => {
      cancelled = true
    }
  }, [service.label, service.plist_path])

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="source-box" onClick={(e) => e.stopPropagation()}>
        <div className="source-box-head">
          <span>{service.file_name ?? service.label}</span>
          <button onClick={onClose}>关闭</button>
        </div>
        {error ? (
          <div className="detail-err" style={{ padding: 14 }}>读取源文件失败：{error}</div>
        ) : content === null ? (
          <div className="hint" style={{ padding: 14 }}>加载中…</div>
        ) : (
          <pre className="source-text">{content}</pre>
        )}
      </div>
    </div>
  )
}
