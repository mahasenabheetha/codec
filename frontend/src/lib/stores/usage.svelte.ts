// What you reach for: tools pinned to the top of the rail and Home, the
// tools opened last (and when, for Home), and the palette's last
// commands. Display preferences, persisted [19]: names of tools and
// commands and times only, never what was typed into them.
import { untrack } from 'svelte'
import { persisted } from './persist.svelte'

const pinned = persisted<string[]>('pinnedTools', [])
const recent = persisted<string[]>('recentTools', [])
const recentAt = persisted<Record<string, number>>('recentToolsAt', {})
const commands = persisted<string[]>('recentCommands', [])

const RECENT_TOOLS = 6
const RECENT_COMMANDS = 5

/** A tool entry's key: the tool id, or `tool.tab` for a sub-tab. */
export const usage = {
  get pinned(): string[] {
    return pinned.value
  },
  isPinned(key: string): boolean {
    return pinned.value.includes(key)
  },
  togglePin(key: string) {
    pinned.value = pinned.value.includes(key) ? pinned.value.filter((k) => k !== key) : [...pinned.value, key]
  },

  /** The tools opened last, newest first. */
  get recent(): string[] {
    return recent.value
  },
  /** When a recent tool was last opened (ms); 0 if before 3.11.0. */
  openedAt(key: string): number {
    return recentAt.value[key] ?? 0
  },
  opened(key: string) {
    // Untracked: it may run inside an effect, which must not come to
    // depend on the lists it rewrites.
    untrack(() => {
      if (recent.value[0] !== key) recent.value = [key, ...recent.value.filter((k) => k !== key)].slice(0, RECENT_TOOLS)
      const now = Date.now()
      recentAt.value = Object.fromEntries(recent.value.map((k) => [k, k === key ? now : (recentAt.value[k] ?? 0)]))
    })
  },

  /** Ids of the palette commands run last, newest first. */
  get commands(): string[] {
    return commands.value
  },
  ran(id: string) {
    if (commands.value[0] === id) return
    commands.value = [id, ...commands.value.filter((k) => k !== id)].slice(0, RECENT_COMMANDS)
  },
}
