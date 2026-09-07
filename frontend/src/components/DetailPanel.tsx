import { useState } from 'react'
import { EditModal } from './EditModal'
import { SourceModal } from './SourceModal'
import type { Service } from '../types'

function Row({ k, children }: { k: string; children: React.ReactNode }) {
  return (
    <tr>
      <th>{k}</th>
      <td>{children}</td>
    </tr>
  )
}

export function DetailPanel({ service }: { service: Service }) {
  const agent = service.agent
  const [showSource, setShowSource] = useState(false)
  const [editing, setEditing] = useState(false)
  return (
    <div className="detail-body">
      <div className="detail-toolbar">
        {agent && (
          <>
            <button className="btn detail" onClick={() => setEditing(true)}>编辑</button>
            <button className="btn detail" onClick={() => setShowSource((v) => !v)}>
              {showSource ? '隐藏源文件' : '源文件'}
            </button>
          </>
        )}
      </div>
      {service.parse_error && <div className="detail-err">错误：{service.parse_error}</div>}
      {agent ? (
        <table className="kv">
          <Row k="运行方式"><strong>{agent.run_description || '仅手动启动'}</strong></Row>
          <Row k="plist 文件"><code>{agent.path}</code></Row>
          {agent.program && <Row k="Program"><code>{agent.program}</code></Row>}
          {agent.program_arguments && agent.program_arguments.length > 0 && (
            <Row k="ProgramArguments">
              <code>{agent.program_arguments.join(' ')}</code>
            </Row>
          )}
          {agent.working_dir && <Row k="WorkingDirectory"><code>{agent.working_dir}</code></Row>}
          {agent.run_at_load && <Row k="RunAtLoad">true — 登录时自动运行</Row>}
          {agent.keep_alive_text && <Row k="KeepAlive">{agent.keep_alive_text}</Row>}
          {!!agent.start_interval && agent.start_interval > 0 && (
            <Row k="StartInterval">每 {agent.start_interval} 秒</Row>
          )}
          {agent.start_calendar && agent.start_calendar.length > 0 && (
            <Row k="StartCalendarInterval">
              {agent.start_calendar.map((c, i) => (
                <div key={i}>{c}</div>
              ))}
            </Row>
          )}
          {agent.watch_paths && agent.watch_paths.length > 0 && (
            <Row k="WatchPaths">
              {agent.watch_paths.map((p) => (
                <div key={p}><code>{p}</code></div>
              ))}
            </Row>
          )}
          {agent.queue_directories && agent.queue_directories.length > 0 && (
            <Row k="QueueDirectories">
              {agent.queue_directories.map((p) => (
                <div key={p}><code>{p}</code></div>
              ))}
            </Row>
          )}
          {agent.std_out_path && <Row k="StandardOutPath"><code>{agent.std_out_path}</code></Row>}
          {agent.std_err_path && <Row k="StandardErrorPath"><code>{agent.std_err_path}</code></Row>}
          {agent.user_name && <Row k="UserName">{agent.user_name}</Row>}
          {agent.group_name && <Row k="GroupName">{agent.group_name}</Row>}
          {agent.process_type && <Row k="ProcessType">{agent.process_type}</Row>}
          {agent.low_priority_io && <Row k="LowPriorityIO">true</Row>}
          {agent.environment && agent.environment.length > 0 && (
            <Row k="EnvironmentVariables">
              {agent.environment.map(([k, v]) => (
                <div key={k}><code>{k}={v}</code></div>
              ))}
            </Row>
          )}
        </table>
      ) : (
        <div className="detail-err">
          未找到对应的 plist 文件{service.plist_path ? `（${service.plist_path}）` : ''}，
          该服务可能由其它目录加载或已在 launchd 中注销。
        </div>
      )}
      {showSource && <SourceModal service={service} onClose={() => setShowSource(false)} />}
      {editing && <EditModal service={service} onClose={() => setEditing(false)} onSaved={() => { setEditing(false); window.location.reload() }} />}
      {!!service.runs && service.runs > 0 && (
        <div className="runs">
          已运行 {service.runs} 次 · 上次 PID {service.pid ?? '-'} · 上次退出码 {service.exit_code ?? '-'}
        </div>
      )}
    </div>
  )
}
