import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // `npm run dev` proxies API calls to a local `sqlforge ui` so the browser
    // stays same-origin (the Go server sends no CORS headers).
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
