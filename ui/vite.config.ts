import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// The engine serves the page under /account/. The Dockerfile copies dist
// into internal/webui before it builds the engine.
export default defineConfig({
  base: '/account/',
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
