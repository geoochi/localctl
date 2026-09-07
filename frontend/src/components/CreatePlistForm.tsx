import { useState } from 'react'
import { createPlist } from '../api'
import type { PlistCreateType } from '../types'

export function CreatePlistForm({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState('')
  const [command, setCommand] = useState('')
  const [type, setType] = useState<PlistCreateType>('runatload')
  const [keepAlive, setKeepAlive] = useState(false)
  const [intervalValue, setIntervalValue] = useState(5)
  const [intervalUnit, setIntervalUnit] = useState<'s' | 'm' | 'h'>('m')
  const [time, setTime] = useState('09:00')
  const [weekdayMode, setWeekdayMode] = useState<'daily' | 'workday'>('daily')
  const [workingDir, setWorkingDir] = useState('')
  const [stdOut, setStdOut] = useState('')
  const [stdErr, setStdErr] = useState('')
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function reset() {
    setLabel('')
    setCommand('')
    setType('runatload')
    setKeepAlive(false)
    setShowAdvanced(false)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    const req: Parameters<typeof createPlist>[0] = {
      label: label.trim(),
      command: command.trim(),
      type,
      working_dir: workingDir.trim() || undefined,
      std_out_path: stdOut.trim() || undefined,
      std_err_path: stdErr.trim() || undefined,
    }

    if (type === 'interval') {
      const mult = intervalUnit === 's' ? 1 : intervalUnit === 'm' ? 60 : 3600
      req.interval_seconds = Math.max(1, Math.round(intervalValue * mult))
    } else if (type === 'calendar') {
      const [h, m] = time.split(':').map(Number)
      req.hour = h
      req.minute = m
      req.weekdays = weekdayMode === 'daily' ? [] : [1, 2, 3, 4, 5]
    }

    setBusy(true)
    try {
      await createPlist(req)
      reset()
      setOpen(false)
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  if (!open) {
    return (
      <div className="create-bar">
        <button className="btn primary" onClick={() => setOpen(true)}>+ 新建 LaunchAgent</button>
      </div>
    )
  }

  return (
    <form className="svc create-form" onSubmit={handleSubmit}>
      <div className="create-title">新建 LaunchAgent（保存到 ~/Library/LaunchAgents 并立即注册）</div>

      <label className="field">
        <span>Label</span>
        <input
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          placeholder="com.geoochi.myjob"
          required
        />
      </label>

      <label className="field">
        <span>命令（shell）</span>
        <input
          value={command}
          onChange={(e) => setCommand(e.target.value)}
          placeholder="例如：/Users/me/scripts/backup.sh"
          required
        />
      </label>

      <div className="field">
        <span>运行方式</span>
        <div className="type-options">
          <label>
            <input
              type="radio"
              checked={type === 'runatload'}
              onChange={() => setType('runatload')}
            />
            登录时启动
          </label>
          <label>
            <input
              type="radio"
              checked={type === 'interval'}
              onChange={() => setType('interval')}
            />
            固定间隔
          </label>
          <label>
            <input
              type="radio"
              checked={type === 'calendar'}
              onChange={() => setType('calendar')}
            />
            定时运行
          </label>
        </div>
      </div>

      {type === 'runatload' && (
        <label className="field inline">
          <input
            type="checkbox"
            checked={keepAlive}
            onChange={(e) => setKeepAlive(e.target.checked)}
          />
          <span>退出后自动重启（KeepAlive）</span>
        </label>
      )}

      {type === 'interval' && (
        <div className="field inline-group">
          <span>间隔</span>
          <input
            type="number"
            min={1}
            value={intervalValue}
            onChange={(e) => setIntervalValue(Number(e.target.value))}
          />
          <select value={intervalUnit} onChange={(e) => setIntervalUnit(e.target.value as 's' | 'm' | 'h')}>
            <option value="s">秒</option>
            <option value="m">分钟</option>
            <option value="h">小时</option>
          </select>
        </div>
      )}

      {type === 'calendar' && (
        <div className="field inline-group">
          <span>时间</span>
          <input type="time" value={time} onChange={(e) => setTime(e.target.value)} required />
          <select value={weekdayMode} onChange={(e) => setWeekdayMode(e.target.value as 'daily' | 'workday')}>
            <option value="daily">每天</option>
            <option value="workday">工作日（周一至周五）</option>
          </select>
        </div>
      )}

      <button type="button" className="btn detail adv-toggle" onClick={() => setShowAdvanced((v) => !v)}>
        {showAdvanced ? '收起高级选项' : '高级选项'}
      </button>
      {showAdvanced && (
        <div className="advanced">
          <label className="field"><span>工作目录</span>
            <input value={workingDir} onChange={(e) => setWorkingDir(e.target.value)} placeholder="可选" />
          </label>
          <label className="field"><span>标准输出日志</span>
            <input value={stdOut} onChange={(e) => setStdOut(e.target.value)} placeholder="可选，如 /tmp/myjob.out" />
          </label>
          <label className="field"><span>错误日志</span>
            <input value={stdErr} onChange={(e) => setStdErr(e.target.value)} placeholder="可选，如 /tmp/myjob.err" />
          </label>
        </div>
      )}

      {error && <div className="detail-err">{error}</div>}

      <div className="create-actions">
        <button type="submit" className="btn primary" disabled={busy}>
          {busy ? '创建中…' : '创建并注册'}
        </button>
        <button type="button" className="btn" onClick={() => setOpen(false)}>取消</button>
      </div>
    </form>
  )
}
