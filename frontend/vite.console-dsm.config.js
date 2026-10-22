import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

// Console build served by DSM itself from the package ui/ folder (opened from the DSM main menu).
export default defineConfig({
  plugins: [vue()],
  base: '/webman/3rdparty/StudioAlbum/console/',
  publicDir: false,
  define: { __CONSOLE_MODE__: JSON.stringify('dsm') },
  build: {
    outDir: '../package/ui/console',
    emptyOutDir: true,
    rollupOptions: { input: { index: resolve(__dirname, 'console-dsm.html') } },
  },
})
