import { useCallback, useEffect, useState } from 'react'
import { fetchCron, importCronEntry } from '../api'
import type { CronEntry } from '../types'

function CronBadge({ entry }: { entry: CronEntry }) {
  if (entry.imported) return <span className="badge enabled">已导入</span>
  if (!entry.importable) return <span className="badge failed">无法导入</span>
  if (entry.approximate) return <span className="badge disabled">近似导入</span>
  return <span className="badge running">可导入</span>
}

export function CronPanel({ onChanged }: { onChanged: () => void }) {
  const [entries, setEntries] = useState<CronEntry[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<number | null>(null)
  const [rowError, setRowError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchCron()
      setEntries(data.entries)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  async function doImport(entry: CronEntry) {
    const note = entry.approximate
      ? `\n注意：${entry.reason}`
      : ''
    if (!window.confirm(`将创建 LaunchAgent 并从 crontab 移除原条目：\n${entry.raw}${note}\n\n确定导入？`)) {
      return
    }
    setBusy(entry.index)
    setRowError(null)
    try {
      await importCronEntry(entry.index)
      await refresh()
      onChanged()
    } catch (e) {
      setRowError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(null)
    }
  }

  return (
    <section className="cron-panel">
      <h2>Cron 计划任务（crontab）</h2>
      <div className="hint">
        可导入的条目会转成 LaunchAgent（StartCalendarInterval；分钟步进用 StartInterval 近似）。
        导入 = 创建 plist 并注册，同时从 crontab 移除原条目，避免双重执行。
      </div>
      {error && <div className="detail-err">读取 crontab 失败：{error}</div>}
      {rowError && <div className="detail-err">{rowError}</div>}
      {entries?.length === 0 && <div className="hint">当前 crontab 为空。</div>}
      {entries?.map((entry) => (
        <div className="svc cron-row" key={`${entry.index}-${entry.label}`}>
          <div className="svc-grid">
            <div className="svc-col">
              <span className="badge idle">{entry.schedule}</span>
              <CronBadge entry={entry} />
            </div>
            <div className="svc-col">
              <span className="program">{entry.command}</span>
              {entry.reason && <span className="cron-reason">{entry.reason}</span>}
            </div>
            <div className="svc-col svc-actions">
              <button
                className="btn"
                disabled={!entry.importable || entry.imported || busy !== null}
                onClick={() => doImport(entry)}
              >
                {busy === entry.index ? '导入中…' : '导入'}
              </button>
            </div>
          </div>
        </div>
      ))}
    </section>
  )
}
