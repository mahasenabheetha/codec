// What you reach for: tools pinned to the top of the rail and Home, the
// tools opened last, and the palette's last commands. Display
// preferences, persisted [19]: names of tools and commands only, never
// what was typed into them.
import { persisted } from './persist.svelte'

const pinned = persisted<string[]>('pinnedTools', [])
const recent = persisted<string[]>('recentTools', [])
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
  opened(key: string) {
    if (recent.value[0] === key) return
    recent.value = [key, ...recent.value.filter((k) => k !== key)].slice(0, RECENT_TOOLS)
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
