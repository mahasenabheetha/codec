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

function prefixed(prefix: string) {
  return {
    route: (p: string) => prefix + p.split("/").map(encodeURIComponent).join("/"),
    parse: (route: string): string | null => {
      if (!route.startsWith(prefix)) return null
      try {
        return route.slice(prefix.length).split("/").map(decodeURIComponent).join("/") || "."
      } catch {
        return null
      }
    },
  }
}
const k8s = prefixed("/k8s/")
const kustomize = prefixed("/kustomize/")
const argo = prefixed("/argo/")
const ciPrefix = prefixed("/ci/")
const composePrefix = prefixed("/compose/")
const ansiblePrefix = prefixed("/ansible/")
const clonePrefix = prefixed("/clone/")

/** Route of the Kubernetes resources view of a folder ("." = root). */
export const k8sRoute = k8s.route
/** The folder a Kubernetes route points at, or null. */
export const routeK8s = k8s.parse
/** Route of a Kustomize build of a directory. */
export const kustomizeRoute = kustomize.route
/** The kustomization directory a route points at, or null. */
export const routeKustomize = kustomize.parse
/** Route of the Argo view of a file. */
export const argoRoute = argo.route
/** The file an Argo route points at, or null. */
export const routeArgo = argo.parse
/** Route of the pipeline view of a CI file. */
export const ciRoute = ciPrefix.route
/** The file a pipeline route points at, or null. */
export const routeCI = ciPrefix.parse
/** Route of the Compose view of a file. */
export const composeRoute = composePrefix.route
/** The file a Compose route points at, or null. */
export const routeCompose = composePrefix.parse
/** Route of the Ansible view of a playbook, task file or inventory. */
export const ansibleRoute = ansiblePrefix.route
/** The file an Ansible route points at, or null. */
export const routeAnsible = ansiblePrefix.parse
/** Route of the clone-with-rename view of a file. */
export const cloneRoute = clonePrefix.route
/** The file a clone route points at, or null. */
export const routeClone = clonePrefix.parse

/** Fixed app views that open as tabs. */
export const problemsRoute = '/problems'
export const settingsRoute = '/settings'
export const compareRoute = '/compare'
export const queryRoute = '/query'
export const newRoute = '/new'

/** Is route one of the fixed views? */
export function isView(route: string): boolean {
  return [problemsRoute, settingsRoute, compareRoute, queryRoute, newRoute].includes(route) || routeK8s(route) !== null || routeKustomize(route) !== null || routeArgo(route) !== null || routeCI(route) !== null || routeCompose(route) !== null || routeAnsible(route) !== null || routeClone(route) !== null
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
