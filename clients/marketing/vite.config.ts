import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  resolve: { dedupe: ['react', 'react-dom', 'react-router', 'react-router-dom', '@mui/material', '@emotion/react'] },
  ssr: {
    noExternal: [/@mui\//, /@emotion\//, '@ironflyer/ui-web', '@ironflyer/design-tokens', '@ironflyer/core'],
  },
  build: { target: 'es2022' },
});
