import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    // Dev proxy: the browser calls /api/... on :5173, Vite forwards it to the backend.
    // Same origin → no CORS problems in development.
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true }, // the API gateway
    },
  },
});
