import { CreatePlistForm } from '../components/CreatePlistForm'
import { CronPanel } from '../components/CronPanel'
import { ThemeToggle } from '../components/ThemeToggle'
import { useState } from 'react'
import { runBackup } from '../api'
import { ServiceCard } from '../components/ServiceCard'
import { useServices } from '../hooks/useServices'

export function Dashboard() {
  const { services, error, refresh } = useServices(5000)
  const [backupMsg, setBackupMsg] = useState<string | null>(null)
  const [backingUp, setBackingUp] = useState(false)

  async function doBackup() {
    setBackingUp(true)
    setBackupMsg(null)
    try {
      const { result } = await runBackup()
      let msg = `已备份 ${result.copied} 个 plist`
      if (result.commit) msg += `，提交 ${result.commit}`
      msg += result.pushed ? '，已推送远端' : `（${result.message ?? '未推送'}）`
      setBackupMsg(msg)
    } catch (e) {
      setBackupMsg('备份失败：' + (e instanceof Error ? e.message : String(e)))
    } finally {
      setBackingUp(false)
    }
  }

  return (
    <>
      <header>
        <h1>localctl — LaunchAgents (gui domain)</h1>
        <div className="header-right">
          <span className="meta">每 5 秒自动刷新</span>
          <ThemeToggle />
        </div>
      </header>
      <main>
        <div className="hint">点击「详情」查看 plist 配置；操作按钮会直接对 launchctl 生效。</div>
        <div className="toolbar">
          <CreatePlistForm onCreated={() => void refresh()} />
          <button className="btn" disabled={backingUp} onClick={() => void doBackup()}>
            {backingUp ? '备份中…' : '备份到 Git'}
          </button>
        </div>
        {backupMsg && <div className="hint">{backupMsg}</div>}
        {error && <div className="detail-err">加载失败：{error}</div>}
        {!services && !error && <div className="hint">加载中…</div>}
        {services?.map((s) => (
          <ServiceCard key={s.plist_path ?? s.label} service={s} onChanged={() => void refresh()} />
        ))}
        <CronPanel onChanged={() => void refresh()} />
      </main>
    </>
  )
}
