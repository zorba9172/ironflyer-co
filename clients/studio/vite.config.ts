import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: { port: 3000 },
  build: {
    target: 'es2022',
    rollupOptions: {
      output: {
        // Keep heavy libs out of the main bundle so screens load lean.
        manualChunks: {
          mui: ['@mui/material', '@emotion/react', '@emotion/styled'],
          query: ['@tanstack/react-query'],
          router: ['react-router-dom'],
        },
      },
    },
  },
});
