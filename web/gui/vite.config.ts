import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'
import { readFileSync } from 'node:fs'

// 编译期版本号来源:package.json。注入为全局常量 __APP_VERSION__,Header 徽标兜底显示。
const pkg = JSON.parse(readFileSync(path.resolve(__dirname, 'package.json'), 'utf-8'))

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      '/test': {
        target: 'ws://127.0.0.1:10888',
        ws: true,
      },
      '/version': 'http://127.0.0.1:10888',
      '/getSubscriptionLink': 'http://127.0.0.1:10888',
      '/getSubscription': 'http://127.0.0.1:10888',
      '/renderImage': 'http://127.0.0.1:10888',
    },
  },
})
