import { useState } from 'react'
import { runAction, type ActionOp } from '../api'
import type { Service } from '../types'
import { StatusBadge } from './StatusBadge'
import { DetailPanelLazy } from './DetailPanelLazy'
import { EditModal } from './EditModal'

const DANGEROUS_OPS = new Set<ActionOp>(['disable', 'unload'])

const CONFIRM_MSG: Record<string, string> = {
  disable: '禁用后即使重启也不会运行，确定禁用',
  unload: '卸载后服务将从 launchd 注销（恢复需装载），确定卸载',
}

export function ServiceCard({ service, onChanged }: { service: Service; onChanged: () => void }) {
  const [showDetail, setShowDetail] = useState(false)
  const [editing, setEditing] = useState(false)
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
      <div className="svc-grid">
        <div className="svc-col">
          <span className="label">{service.label}</span>
          <div className="badge-row">
            <StatusBadge service={service} />
            {service.enabled ? (
              <span className="badge enabled">enabled</span>
            ) : (
              <span className="badge disabled">disabled</span>
            )}
          </div>
          {service.agent?.run_description && (
            <span className="run-desc">⏱ {service.agent.run_description}</span>
          )}
          {service.parse_error && <span className="badge failed">plist 解析失败</span>}
        </div>

        <div className="svc-col">
          {service.program ? (
            <span className="program">{service.program}</span>
          ) : (
            <span className="program">（无程序路径）</span>
          )}
        </div>

        <div className="svc-col svc-actions">
          <button className="btn detail" onClick={() => setShowDetail((v) => !v)}>
            {showDetail ? '收起' : '详情'}
          </button>
          {service.plist_path && (
            <button className="btn detail" onClick={() => setEditing(true)}>编辑</button>
          )}
          {/* kickstart -k：没跑就启动，跑着就重启 */}
          <button className="btn" disabled={busy} onClick={() => doAction('restart')}>
            {service.state === 'running' ? '重启' : '运行'}
          </button>
          {service.loaded && (
            <button className="btn warn" disabled={busy} onClick={() => doAction('unload')}>卸载</button>
          )}
          {!service.loaded && service.plist_path && (
            <button className="btn" disabled={busy} onClick={() => doAction('load')}>装载</button>
          )}
          {service.enabled ? (
            <button className="btn warn" disabled={busy} onClick={() => doAction('disable')}>禁用</button>
          ) : (
            <button className="btn" disabled={busy} onClick={() => doAction('enable')}>启用</button>
          )}
          {service.plist_path && (
            <button className="btn danger" disabled={busy} onClick={() => doAction('delete')}>删除</button>
          )}
        </div>
      </div>
      {actionError && <div className="action-error">{actionError}</div>}
      {showDetail && <DetailPanelLazy service={service} />}
      {editing && (
        <EditModal service={service} onClose={() => setEditing(false)} onSaved={() => void onChanged()} />
      )}
    </div>
  )
}
