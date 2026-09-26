// App-shell UI state: rail, explorer, palette, dialogs, open tabs and
// the status line. Kept in one place so the shell, palette and tools
// agree.

import { persisted } from './persist.svelte'
import { router, fileRoute } from './router.svelte'

const railCollapsed = persisted('railCollapsed', false)
const explorerOpen = persisted('explorerOpen', true)
const explorerWidth = persisted('explorerWidth', 280)

// Tabs are routes (#/tools/json, #/file/a/b.yaml). Before workspaces,
// tabs were stored as bare tool ids; upgrade those in place.
const openTabs = persisted<string[]>('openTabs', [])
openTabs.value = openTabs.value.map((t) => (t.startsWith('/') ? t : '/tools/' + t))

export type StatusTone = 'neutral' | 'ok' | 'warn' | 'err'

export const EXPLORER_MIN = 180
export const EXPLORER_MAX = 560

class Layout {
  paletteOpen = $state(false)
  shortcutsOpen = $state(false)

  /** Status-bar message owned by the active tool. */
  status = $state<{ text: string; tone: StatusTone }>({ text: '', tone: 'neutral' })

  get railCollapsed() {
    return railCollapsed.value
  }
  set railCollapsed(v: boolean) {
    railCollapsed.value = v
  }
  toggleRail() {
    railCollapsed.value = !railCollapsed.value
  }

  get explorerOpen() {
    return explorerOpen.value
  }
  set explorerOpen(v: boolean) {
    explorerOpen.value = v
  }
  toggleExplorer() {
    explorerOpen.value = !explorerOpen.value
  }

  get explorerWidth() {
    return explorerWidth.value
  }
  set explorerWidth(px: number) {
    explorerWidth.value = Math.round(Math.min(EXPLORER_MAX, Math.max(EXPLORER_MIN, px)))
  }

  /** Open tabs, as routes. */
  get tabs(): string[] {
    return openTabs.value
  }

  /** Open (or focus) a tab for route and navigate to it. */
  open(route: string) {
    this.ensureTab(route)
    router.go(route)
  }

  openTool(id: string) {
    this.open('/tools/' + id)
  }

  openFile(path: string) {
    this.open(fileRoute(path))
  }

  /** Called when the URL points at a tab, e.g. from a bookmark. */
  ensureTab(route: string) {
    if (!openTabs.value.includes(route)) openTabs.value = [...openTabs.value, route]
  }

  close(route: string) {
    const tabs = openTabs.value
    const i = tabs.indexOf(route)
    if (i < 0) return
    const next = tabs.filter((t) => t !== route)
    openTabs.value = next
    if (router.path === route) {
      // Focus the neighbour, like an editor does; home if none left.
      router.go(next[Math.min(i, next.length - 1)] ?? '/')
    }
  }

  /** Close every tab whose route matches, e.g. all file tabs. */
  closeWhere(match: (route: string) => boolean) {
    const active = router.path
    openTabs.value = openTabs.value.filter((t) => !match(t))
    if (match(active)) router.go(openTabs.value[0] ?? '/')
  }

  setStatus(text: string, tone: StatusTone = 'neutral') {
    this.status = { text, tone }
  }
}

export const layout = new Layout()
