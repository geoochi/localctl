import { useEffect, useState } from 'react'
import { fetchSource, saveSource } from '../api'
import type { Service } from '../types'

export function EditModal({
  service,
  onClose,
  onSaved,
}: {
  service: Service
  onClose: () => void
  onSaved: () => void
}) {
  const [content, setContent] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

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

  async function handleSave() {
    if (content === null) return
    setBusy(true)
    setError(null)
    try {
      await saveSource(service.label, service.plist_path ?? '', content)
      onSaved()
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="source-box" onClick={(e) => e.stopPropagation()}>
        <div className="source-box-head">
          <span>编辑 {service.file_name ?? service.label}</span>
          <button onClick={onClose}>关闭</button>
        </div>
        {error && <div className="detail-err edit-error">{error}</div>}
        {content === null && !error ? (
          <div className="hint" style={{ padding: 14 }}>加载中…</div>
        ) : (
          <textarea
            className="source-textarea"
            value={content ?? ''}
            onChange={(e) => setContent(e.target.value)}
            spellCheck={false}
          />
        )}
        <div className="edit-actions">
          <button className="btn primary" disabled={busy || content === null} onClick={handleSave}>
            {busy ? '保存中…' : '保存'}
          </button>
          <button className="btn" disabled={busy} onClick={onClose}>取消</button>
        </div>
      </div>
    </div>
  )
}
