// Command registry behind the command palette. Global commands are
// registered once at startup; tools register contextual commands while
// they are the active tab.

import { untrack, type Component } from 'svelte'

export interface Command {
  id: string
  title: string
  group: string
  run: () => void
  shortcut?: string
  icon?: Component
  keywords?: string[]
}

class Commands {
  list = $state<Command[]>([])

  /** Add commands; returns a function that removes them again. Safe to
   *  call from an $effect: the list is read untracked, so registering
   *  doesn't make the effect depend on (and re-run for) its own write. */
  register(cmds: Command[]): () => void {
    untrack(() => this.list.push(...cmds))
    const ids = new Set(cmds.map((c) => c.id))
    return () => {
      this.list = untrack(() => this.list.filter((c) => !ids.has(c.id)))
    }
  }
}

export const commands = new Commands()
