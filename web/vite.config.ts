import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: path.resolve(__dirname, '../internal/ui/dist'),
    emptyOutDir: true,
  },
  server: {
    port: 3000,
    proxy: {
      '/v2': 'http://localhost:5000',
      '/api': 'http://localhost:5000',
      '/metrics': 'http://localhost:5000',
    },
  },
});
