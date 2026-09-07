import { useState } from 'react'
import { getTheme, nextTheme, setTheme, type Theme } from '../theme'

const ICONS: Record<Theme, React.ReactNode> = {
  // sync: 跟随系统
  sync: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="3" width="20" height="14" rx="2" />
      <line x1="8" y1="21" x2="16" y2="21" />
      <line x1="12" y1="17" x2="12" y2="21" />
    </svg>
  ),
  // light: 太阳
  light: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="4" />
      <line x1="12" y1="1" x2="12" y2="4" />
      <line x1="12" y1="20" x2="12" y2="23" />
      <line x1="4.2" y1="4.2" x2="6.3" y2="6.3" />
      <line x1="17.7" y1="17.7" x2="19.8" y2="19.8" />
      <line x1="1" y1="12" x2="4" y2="12" />
      <line x1="20" y1="12" x2="23" y2="12" />
      <line x1="4.2" y1="19.8" x2="6.3" y2="17.7" />
      <line x1="17.7" y1="6.3" x2="19.8" y2="4.2" />
    </svg>
  ),
  // dark: 月亮
  dark: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
    </svg>
  ),
}

const LABELS: Record<Theme, string> = {
  sync: '跟随系统',
  light: '浅色模式',
  dark: '深色模式',
}

export function ThemeToggle() {
  const [theme, setThemeState] = useState<Theme>(getTheme())

  function cycle() {
    const t = nextTheme(theme)
    setTheme(t)
    setThemeState(t)
  }

  return (
    <button
      className="theme-toggle"
      onClick={cycle}
      title={`主题：${LABELS[theme]}（点击切换）`}
      aria-label={LABELS[theme]}
    >
      {ICONS[theme]}
    </button>
  )
}
