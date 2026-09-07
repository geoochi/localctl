import { Dashboard } from './pages/Dashboard'
import { useEffect } from 'react'
import { initTheme } from './theme'

export default function App() {
  useEffect(() => {
    initTheme()
  }, [])
  return <Dashboard />
}
