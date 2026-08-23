import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// API target is configurable so dev/preview can point at any backend.
const apiTarget: string =
  (globalThis as { process?: { env?: Record<string, string> } }).process?.env?.GITCASTLE_API ??
  'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { '/api': apiTarget },
  },
  preview: {
    port: 4173,
    proxy: { '/api': apiTarget },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    exclude: ['e2e/**', 'node_modules/**', 'dist/**'],
  },
})
