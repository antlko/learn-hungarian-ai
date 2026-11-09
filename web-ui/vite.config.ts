import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    rollupOptions: {
      output: {
        // One stable vendor chunk: the framework changes only on upgrades, so
        // an app release does not invalidate it in the browser cache.
        manualChunks(id) {
          if (/node_modules\/(vue|@vue|vue-router|pinia|vue-i18n|@intlify)\//.test(id)) return 'vendor'
        },
      },
    },
  },
  server: {
    // Proxy to the Go backend in development so cookies and requests work end
    // to end. In production the Go binary serves this build from an embedded FS.
    proxy: {
      '/api': 'http://localhost:8012',
      '/media': 'http://localhost:8012',
    },
  },
})
