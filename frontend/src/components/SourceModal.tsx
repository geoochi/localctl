import { useEffect, useState } from 'react'
import { fetchSource } from '../api'
import type { Service } from '../types'

// 极简 XML 语法高亮：先转义，再用占位符避免高亮 span 被二次处理。
function highlightXML(code: string): string {
  const esc = code.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  const spans: string[] = []
  const put = (cls: string, text: string) => {
    spans.push(`<span class="tok-${cls}">${text}</span>`)
    return `\x00${spans.length - 1}\x00`
  }
  let out = esc
    // 标签之间的文本内容
    .replace(/&gt;([^&\x00]+?)&lt;/g, (_m, c) => '&gt;' + put('str', c) + '&lt;')
    // 注释
    .replace(/&lt;!--[\s\S]*?--&gt;/g, (m) => put('comment', m))
    // 标签名与尖括号
    .replace(/&lt;\/?[\w.:-]+|&gt;/g, (m) => put('tag', m))
  out = out.replace(/\x00(\d+)\x00/g, (_m, i) => spans[Number(i)])
  return out
}

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

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const highlighted = content !== null ? highlightXML(content) : ''
  const lineCount = content !== null ? content.split('\n').length : 0
  const downloadHref = service.plist_path
    ? `/api/services/${encodeURIComponent(service.label)}/source?path=${encodeURIComponent(service.plist_path)}&download=1`
    : undefined

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="editor" onClick={(e) => e.stopPropagation()}>
        <div className="editor-titlebar">
          <span className="editor-tab">{service.file_name ?? service.label}</span>
          <div className="editor-actions">
            {downloadHref && (
              <a className="editor-btn" href={downloadHref}>下载</a>
            )}
            <button className="editor-btn" onClick={onClose}>关闭 ✕</button>
          </div>
        </div>
        <div className="editor-body">
          {error && <div className="editor-status">读取源文件失败：{error}</div>}
          {!error && content === null && <div className="editor-status">加载中…</div>}
          {content !== null && (
            <>
              <pre className="editor-gutter">
                {Array.from({ length: lineCount }, (_, i) => i + 1).join('\n')}
              </pre>
              <pre className="editor-code" dangerouslySetInnerHTML={{ __html: highlighted }} />
            </>
          )}
        </div>
        <div className="editor-statusbar">
          <span>XML · {lineCount} 行</span>
          <span>UTF-8</span>
        </div>
      </div>
    </div>
  )
}
