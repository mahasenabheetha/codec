// Hash router: routes look like #/tools/base64. Hash routing needs no
// server support, so the Go binary just serves index.html.

import { persisted } from './persist.svelte'

const lastRoute = persisted('lastRoute', '/')

function readHash(): string {
  const h = location.hash.replace(/^#/, '')
  return h.startsWith('/') ? h : '/'
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
    const m = this.path.match(/^\/tools\/([\w-]+)/)
    return m ? m[1] : null
  }
}

export const router = new Router()
