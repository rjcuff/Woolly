import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Builds into the Go server package, which embeds it into the woolly binary.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/server/ui',
    emptyOutDir: true,
  },
  server: {
    // During development, send API calls to a running `woolly serve`.
    proxy: { '/api': 'http://127.0.0.1:11435' },
  },
});
