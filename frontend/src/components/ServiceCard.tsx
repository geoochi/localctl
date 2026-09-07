import { useState } from 'react'
import { runAction, type ActionOp } from '../api'
import type { Service } from '../types'
import { StatusBadge } from './StatusBadge'
import { DetailPanelLazy } from './DetailPanelLazy'

const DANGEROUS_OPS = new Set<ActionOp>(['stop', 'disable', 'unload'])

const CONFIRM_MSG: Record<string, string> = {
  stop: '确定停止',
  disable: '禁用后即使重启也不会运行，确定禁用',
  unload: '卸载后服务将从 launchd 注销，确定卸载',
}

export function ServiceCard({ service, onChanged }: { service: Service; onChanged: () => void }) {
  const [showDetail, setShowDetail] = useState(false)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  async function doAction(op: ActionOp) {
    if (op === 'delete' && !window.confirm(`删除将卸载服务并永久移除 plist 文件：\n${service.plist_path}\n\n确定删除 ${service.label}？`)) {
      return
    }
    if (DANGEROUS_OPS.has(op) && !window.confirm(`${CONFIRM_MSG[op]} ${service.label}？`)) {
      return
    }
    setBusy(true)
    setActionError(null)
    try {
      await runAction(service.label, op, service.plist_path)
      onChanged()
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="svc">
      <div className="svc-head">
        <div className="svc-main">
          <span className="label">{service.label}</span>
          <StatusBadge service={service} />
          {service.enabled ? (
            <span className="badge enabled">enabled</span>
          ) : (
            <span className="badge disabled">disabled</span>
          )}
          {service.program && <span className="program">{service.program}</span>}
          {service.agent?.run_description && (
            <span className="run-desc">⏱ {service.agent.run_description}</span>
          )}
          {service.parse_error && <span className="badge failed">plist 解析失败</span>}
        </div>
        <div className="svc-actions">
          <button className="btn detail" onClick={() => setShowDetail((v) => !v)}>
            {showDetail ? '收起' : '详情'}
          </button>
          <button className="btn" disabled={busy} onClick={() => doAction('start')}>Start</button>
          <button className="btn" disabled={busy} onClick={() => doAction('restart')}>Restart</button>
          <button className="btn warn" disabled={busy} onClick={() => doAction('stop')}>Stop</button>
          {service.enabled ? (
            <button className="btn warn" disabled={busy} onClick={() => doAction('disable')}>Disable</button>
          ) : (
            <button className="btn" disabled={busy} onClick={() => doAction('enable')}>Enable</button>
          )}
          {service.loaded ? (
            <button className="btn warn" disabled={busy} onClick={() => doAction('unload')}>Unload</button>
          ) : (
            service.plist_path && (
              <button className="btn" disabled={busy} onClick={() => doAction('load')}>Load</button>
            )
          )}
          {service.plist_path && (
            <button className="btn danger" disabled={busy} onClick={() => doAction('delete')}>Delete</button>
          )}
        </div>
      </div>
      {actionError && <div className="action-error">{actionError}</div>}
      {showDetail && <DetailPanelLazy service={service} />}
    </div>
  )
}
