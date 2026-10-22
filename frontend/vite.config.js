import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

// Main build: customer app (index.html), studio app (studio.html), console on app port (console.html).
// Output is embedded into the Go binary.
export default defineConfig({
  plugins: [vue()],
  base: '/',
  define: { __CONSOLE_MODE__: JSON.stringify('server') },
  build: {
    outDir: '../backend/web',
    emptyOutDir: true,
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      input: {
        index: resolve(__dirname, 'index.html'),
        studio: resolve(__dirname, 'studio.html'),
        console: resolve(__dirname, 'console.html'),
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8686',
      '/img': 'http://127.0.0.1:8686',
      '/dl': 'http://127.0.0.1:8686',
      '/branding': 'http://127.0.0.1:8686',
      '/manifest.webmanifest': 'http://127.0.0.1:8686',
      '/ws': { target: 'ws://127.0.0.1:8686', ws: true },
    },
  },
})
