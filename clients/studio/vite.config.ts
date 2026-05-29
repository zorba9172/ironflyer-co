import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    // The runtime service (workspace File API, PTY WebSocket, embedded IDE) is
    // a separate REST service. The data layer calls it at the relative base
    // `/api/runtime` (see packages/data/src/runtime.ts); proxy that to the
    // local runtime on :8090, stripping the prefix since the runtime mounts at
    // root (`/workspaces/{id}/ide`). Override the target with RUNTIME_PROXY.
    proxy: {
      '/api/runtime': {
        target: process.env.RUNTIME_PROXY ?? 'http://localhost:8090',
        changeOrigin: true,
        ws: true,
        rewrite: (p) => p.replace(/^\/api\/runtime/, ''),
      },
    },
  },
  build: {
    target: 'es2022',
    rollupOptions: {
      output: {
        // Keep heavy libs out of page chunks so route-level lazy imports stay lean.
        manualChunks(id: string) {
          if (id.includes('/node_modules/@mui/material/') || id.includes('/node_modules/@emotion/')) return 'mui';
          if (id.includes('/node_modules/@tanstack/react-query/')) return 'query';
          if (id.includes('/node_modules/react-router') || id.includes('/node_modules/react-router-dom/')) return 'router';
          if (id.includes('/node_modules/echarts/') || id.includes('/node_modules/zrender/')) return 'charts';
          if (id.includes('/node_modules/ag-grid-community/') || id.includes('/node_modules/ag-grid-react/')) return 'data-grid';
        },
      },
    },
  },
});
