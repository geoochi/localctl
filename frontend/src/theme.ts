export type Theme = 'sync' | 'light' | 'dark'

const THEME_KEY = 'localctl_theme'

export function getTheme(): Theme {
  const t = localStorage.getItem(THEME_KEY)
  return t === 'light' || t === 'dark' ? t : 'sync'
}

function apply(theme: Theme) {
  const dark =
    theme === 'dark' ||
    (theme === 'sync' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
}

export function setTheme(theme: Theme) {
  localStorage.setItem(THEME_KEY, theme)
  apply(theme)
}

// 初始化主题并跟随系统变化（仅 sync 模式下）。
export function initTheme() {
  apply(getTheme())
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (getTheme() === 'sync') apply('sync')
  })
}

export function nextTheme(theme: Theme): Theme {
  return theme === 'sync' ? 'light' : theme === 'light' ? 'dark' : 'sync'
}
