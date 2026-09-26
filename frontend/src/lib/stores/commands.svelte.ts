// Command registry behind the command palette. Global commands are
// registered once at startup; tools register contextual commands while
// they are the active tab.

import type { Component } from 'svelte'

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

  /** Add commands; returns a function that removes them again. */
  register(cmds: Command[]): () => void {
    this.list.push(...cmds)
    const ids = new Set(cmds.map((c) => c.id))
    return () => {
      this.list = this.list.filter((c) => !ids.has(c.id))
    }
  }
}

export const commands = new Commands()
