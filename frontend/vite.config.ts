import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig, type Plugin } from 'vite'

// `npm run dev` talks to a running `codec serve` for the API. Override
// with CODEC_API when that server isn't on the default port (e.g. an
// installed codec already holds 8765).
const api = process.env.CODEC_API ?? 'http://127.0.0.1:8765'

// codec serve puts a per-run token in its index.html, and /api/v2 calls
// must send it back. The dev server has its own index.html, so copy the
// token from the running codec on every page load.
function codecToken(): Plugin {
  return {
    name: 'codec-token',
    apply: 'serve',
    async transformIndexHtml(html) {
      try {
        const page = await (await fetch(api + '/')).text()
        const meta = page.match(/<meta name="codec-token"[^>]*>/)?.[0]
        return meta ? html.replace('<head>', '<head>\n    ' + meta) : html
      } catch {
        console.warn(`codec-token: ${api} is not reachable; /api/v2 calls will fail`)
        return html
      }
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte(), codecToken()],

  build: {
    // Built straight into the folder the Go binary embeds. Only the
    // app/ subfolder is emptied, so the committed dist/.keep survives
    // and go:embed always has something to embed.
    outDir: '../internal/web/dist/app',
    emptyOutDir: true,
  },

  server: {
    proxy: { '/api': api },
  },
})
