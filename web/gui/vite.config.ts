import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
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
      '/getSubscriptionLink': 'http://127.0.0.1:10888',
      '/getSubscription': 'http://127.0.0.1:10888',
      '/renderImage': 'http://127.0.0.1:10888',
    },
  },
})
