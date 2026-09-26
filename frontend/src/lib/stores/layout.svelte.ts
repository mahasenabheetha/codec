// App-shell UI state: rail, palette, dialogs, open tabs and the status
// line. Kept in one place so the shell, palette and tools agree.

import { persisted } from './persist.svelte'
import { router } from './router.svelte'

const railCollapsed = persisted('railCollapsed', false)
const openTabs = persisted<string[]>('openTabs', [])

export type StatusTone = 'neutral' | 'ok' | 'warn' | 'err'

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

  get tabs(): string[] {
    return openTabs.value
  }

  /** Open (or focus) a tool's tab and navigate to it. */
  openTool(id: string) {
    if (!openTabs.value.includes(id)) openTabs.value = [...openTabs.value, id]
    router.go('/tools/' + id)
  }

  /** Called when the route points at a tool, e.g. from a bookmark. */
  ensureTab(id: string) {
    if (!openTabs.value.includes(id)) openTabs.value = [...openTabs.value, id]
  }

  closeTool(id: string) {
    const tabs = openTabs.value
    const i = tabs.indexOf(id)
    if (i < 0) return
    const next = tabs.filter((t) => t !== id)
    openTabs.value = next
    if (router.toolId === id) {
      // Focus the neighbour, like an editor does; home if none left.
      const neighbour = next[Math.min(i, next.length - 1)]
      router.go(neighbour ? '/tools/' + neighbour : '/')
    }
  }

  setStatus(text: string, tone: StatusTone = 'neutral') {
    this.status = { text, tone }
  }
}

export const layout = new Layout()
