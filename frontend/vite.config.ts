import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],

  build: {
    // Built straight into the folder the Go binary embeds. Only the
    // app/ subfolder is emptied, so the committed dist/.keep survives
    // and go:embed always has something to embed.
    outDir: '../internal/web/dist/app',
    emptyOutDir: true,
  },

  server: {
    // `npm run dev` talks to a running `codec serve` for the API.
    // Override with CODEC_API when that server isn't on the default
    // port (e.g. an installed codec already holds 8765).
    proxy: {
      '/api': process.env.CODEC_API ?? 'http://127.0.0.1:8765',
    },
  },
})
