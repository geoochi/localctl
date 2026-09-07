import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 仅前端开发时使用（生产模式：pnpm build 后由 Go 二进制内嵌托管，见 embed.go）。
// 开发时后端需另跑在 7788：LOCALCTL_ADDR=127.0.0.1:7788 go run ./cmd/localctl
export default defineConfig({
  plugins: [react()],
  server: {
    port: 8003,
    proxy: {
      '/api': 'http://127.0.0.1:7788',
    },
  },
})
