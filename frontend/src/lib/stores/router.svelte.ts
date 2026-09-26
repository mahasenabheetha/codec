// Hash router: routes look like #/tools/base64 or #/file/charts/app/values.yaml.
// Hash routing needs no server support, so the Go binary just serves
// index.html.

import { persisted } from './persist.svelte'

const lastRoute = persisted('lastRoute', '/')

function readHash(): string {
  const h = location.hash.replace(/^#/, '')
  return h.startsWith('/') ? h : '/'
}

/** Route of a workspace file. Each segment is encoded, slashes kept. */
export function fileRoute(path: string): string {
  return '/file/' + path.split('/').map(encodeURIComponent).join('/')
}

/** The workspace file a route points at, or null. */
export function routeFile(route: string): string | null {
  const m = route.match(/^\/file\/(.+)$/)
  if (!m) return null
  try {
    return m[1].split('/').map(decodeURIComponent).join('/')
  } catch {
    return null // malformed escape in a hand-edited URL
  }
}

/** Route of a Helm view for a chart directory ("." = workspace root). */
export function helmRoute(chart: string): string {
  return '/helm/' + chart.split('/').map(encodeURIComponent).join('/')
}

/** The chart a Helm route points at, or null. */
export function routeHelm(route: string): string | null {
  const m = route.match(/^\/helm\/(.+)$/)
  if (!m) return null
  try {
    return m[1].split('/').map(decodeURIComponent).join('/')
  } catch {
    return null
  }
}

/** The tool id a route points at, or null. */
export function routeTool(route: string): string | null {
  const m = route.match(/^\/tools\/([\w-]+)/)
  return m ? m[1] : null
}

class Router {
  path = $state('/')

  constructor() {
    // No hash on first load: reopen where the user left off.
    if (!location.hash && lastRoute.value !== '/') {
      history.replaceState(null, '', '#' + lastRoute.value)
    }
    this.path = readHash()
    window.addEventListener('hashchange', () => {
      this.path = readHash()
      lastRoute.value = this.path
    })
  }

  go(path: string) {
    if (path !== this.path) location.hash = path
  }

  /** The tool id for /tools/<id>, or null on other routes. */
  get toolId(): string | null {
    return routeTool(this.path)
  }

  /** The workspace file for /file/<path>, or null on other routes. */
  get filePath(): string | null {
    return routeFile(this.path)
  }
}

export const router = new Router()
