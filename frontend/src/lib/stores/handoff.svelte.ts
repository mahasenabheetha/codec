// Input one tool hands to another, such as smart paste opening a pasted
// cron expression in Time → Cron. In memory only, taken once.

import { layout } from './layout.svelte'

class Handoff {
  pending = $state<{ tool: string; tab: string; input: string } | null>(null)

  /** Open tool on tab with input in its box. */
  send(tool: string, tab: string, input: string) {
    this.pending = { tool, tab, input }
    layout.openTool(tool, tab)
  }

  /** The input handed to this tool and tab, once; null otherwise. */
  take(tool: string, tab: string): string | null {
    const p = this.pending
    if (!p || p.tool !== tool || p.tab !== tab) return null
    this.pending = null
    return p.input
  }
}

export const handoff = new Handoff()
