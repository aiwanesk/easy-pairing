import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// En développement, l'API Go tourne sur :8080 et Vite la proxifie.
export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: false } } },
  build: { chunkSizeWarningLimit: 900 },
})
