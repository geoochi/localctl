import { CreatePlistForm } from '../components/CreatePlistForm'
import { CronPanel } from '../components/CronPanel'
import { ServiceCard } from '../components/ServiceCard'
import { useServices } from '../hooks/useServices'

export function Dashboard() {
  const { services, error, refresh } = useServices(5000)

  return (
    <>
      <header>
        <h1>localctl — LaunchAgents (gui domain)</h1>
        <div>
          <span className="meta">每 5 秒自动刷新</span>
        </div>
      </header>
      <main>
        <div className="hint">点击「详情」查看 plist 配置；操作按钮会直接对 launchctl 生效。</div>
        <CreatePlistForm onCreated={() => void refresh()} />
        {error && <div className="detail-err">加载失败：{error}</div>}
        {!services && !error && <div className="hint">加载中…</div>}
        {services?.map((s) => <ServiceCard key={s.label} service={s} onChanged={() => void refresh()} />)}
        <CronPanel onChanged={() => void refresh()} />
      </main>
    </>
  )
}
