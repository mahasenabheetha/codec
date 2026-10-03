// App-shell UI state: rail, explorer, palette, dialogs, open tabs and
// the status line. Kept in one place so the shell, palette and tools
// agree.

import { persisted } from './persist.svelte'
import { router, fileRoute, helmRoute } from './router.svelte'

const railCollapsed = persisted('railCollapsed', false)
const explorerOpen = persisted('explorerOpen', true)
// What the sidebar panel beside the rail shows.
const sidebarView = persisted<'explorer' | 'search'>('sidebarView', 'explorer')
const explorerWidth = persisted('explorerWidth', 280)
// The last sub-tab of each tool that has them (Time → Cron).
const toolTabs = persisted<Record<string, string>>('toolTabs', {})

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
  get sidebarView() {
    return sidebarView.value
  }
  /** Show the Explorer; hide the panel if it is already showing. */
  toggleExplorer() {
    explorerOpen.value = !(explorerOpen.value && sidebarView.value === 'explorer')
    sidebarView.value = 'explorer'
  }
  /** Show Find in Files in the panel (the search box takes focus). */
  showSearch() {
    explorerOpen.value = true
    sidebarView.value = 'search'
  }
  /** Show Find in Files; hide the panel if it is already showing. */
  toggleSearch() {
    if (explorerOpen.value && sidebarView.value === 'search') explorerOpen.value = false
    else this.showSearch()
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

  /** Open a tool, on one of its sub-tabs when tab is given. */
  openTool(id: string, tab?: string) {
    if (tab) this.setToolTab(id, tab)
    this.open('/tools/' + id)
  }

  /** The sub-tab a tool last showed, or undefined. */
  toolTab(id: string): string | undefined {
    return toolTabs.value[id]
  }
  setToolTab(id: string, tab: string) {
    toolTabs.value = { ...toolTabs.value, [id]: tab }
  }

  openFile(path: string) {
    this.open(fileRoute(path))
  }

  openHelm(chart: string) {
    this.open(helmRoute(chart))
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
