import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'
import { readFileSync } from 'node:fs'

// 编译期版本号来源:package.json。注入为全局常量 __APP_VERSION__,Header 徽标兜底显示。
const pkg = JSON.parse(readFileSync(path.resolve(__dirname, 'package.json'), 'utf-8'))

// https://vite.dev/config/
export default defineConfig({
  // 产物内所有资源引用走相对路径(./assets/...),而不是绝对的 /assets/...。
  // 这样同一份 dist 既能挂在域名根目录,也能挂在二级路径(如 /speedtest/)下,
  // 无需按部署路径重新编译。运行时的接口地址同样相对解析,见 src/lib/api.ts。
  base: './',
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
