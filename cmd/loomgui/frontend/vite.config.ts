import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': '/src',
    },
  },
  server: {
    port: 3000,
    // Enable detailed error overlay
    hmr: {
      overlay: true,
    },
  },
  build: {
    // Enable source maps for better debugging
    sourcemap: true,
    // Show build errors in console
    rollupOptions: {
      onwarn(warning, warn) {
        // Log all warnings to console
        console.warn('Vite build warning:', warning);
        warn(warning);
      },
    },
  },
  // Enable detailed error logging
  logLevel: 'info',
  // Show error overlay in development
  clearScreen: false,
})