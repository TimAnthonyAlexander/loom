import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  clearScreen: false,
  server: {
    port: 1420,
    strictPort: true,
    hmr: {
      // Disable HMR WebSocket for Wails compatibility
      port: 1421,
    },
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      external: ['#minpath', '#minproc', '#minurl']
    },
  },
  optimizeDeps: {
    exclude: ['#minpath', '#minproc', '#minurl']
  },
  envPrefix: ['VITE_', 'TAURI_PLATFORM', 'TAURI_ARCH', 'TAURI_FAMILY', 'TAURI_PLATFORM_VERSION', 'TAURI_PLATFORM_TYPE'],
})