import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// 本番はreverse proxyがWebとAPIを同一オリジンで公開する。
// dev serverでも同じ前提となるよう、`/api` をAPIへproxyする。
const apiProxyTarget = process.env.VITE_API_PROXY_TARGET ?? 'http://api:8081'
const hmrClientPort = Number(process.env.VITE_HMR_CLIENT_PORT ?? 5173)

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    // reverse proxy経由でHMR websocketを中継する。
    hmr: { clientPort: hmrClientPort },
    // reverse proxyのhost headerを許可する。
    allowedHosts: ['localhost', 'web', '127.0.0.1'],
    proxy: {
      '/api': { target: apiProxyTarget, changeOrigin: false },
      '/health': { target: apiProxyTarget, changeOrigin: false },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
})
