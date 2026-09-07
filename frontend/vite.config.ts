import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发时将 /api 代理到 Go 后端，浏览器视角同源，无 CORS 问题。
export default defineConfig({
  plugins: [react()],
  server: {
    port: 8003,
    proxy: {
      '/api': 'http://127.0.0.1:7788',
    },
  },
})
