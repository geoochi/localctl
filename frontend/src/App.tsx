import { useEffect, useState } from 'react'
import { getToken } from './api'
import { Login } from './pages/Login'
import { Dashboard } from './pages/Dashboard'

export default function App() {
  const [authed, setAuthed] = useState(() => getToken() !== null)

  // 任意 API 调用遇到 401（token 失效）时回到登录页。
  useEffect(() => {
    const handler = () => setAuthed(false)
    window.addEventListener('localctl:unauthorized', handler)
    return () => window.removeEventListener('localctl:unauthorized', handler)
  }, [])

  if (!authed) {
    return <Login onLoggedIn={() => setAuthed(true)} />
  }
  return <Dashboard />
}
