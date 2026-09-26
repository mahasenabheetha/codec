// Wiring every tool shares: keyboard shortcuts, palette commands and
// the status-bar line — all scoped to the tool's active tab. Call these
// during component init; cleanup is automatic.

import { untrack } from 'svelte'
import { shortcut } from '../../lib/stores/shortcuts.svelte'
import { commands, type Command } from '../../lib/stores/commands.svelte'
import { layout, type StatusTone } from '../../lib/stores/layout.svelte'

export interface ToolActions {
  run: () => void
  copy: () => void
  clear: () => void
  swap?: () => void
}

/** The v1 shortcuts, kept identical: Mod+Enter, Alt+C, Esc (+ Alt+S swap). */
export function toolShortcuts(isActive: () => boolean, a: ToolActions): void {
  $effect(() => {
    const offs = [
      shortcut('Mod+Enter', a.run, { when: isActive }),
      shortcut('Alt+C', a.copy, { when: isActive }),
      shortcut('Escape', a.clear, { when: isActive, inInputs: true }),
    ]
    if (a.swap) offs.push(shortcut('Alt+S', a.swap, { when: isActive }))
    return () => offs.forEach((off) => off())
  })
}

/** Palette commands that exist only while this tool is the active tab. */
export function toolCommands(isActive: () => boolean, build: () => Command[]): void {
  $effect(() => {
    if (!isActive()) return
    return commands.register(untrack(build))
  })
}

/** Keep the status bar in sync with this tool while it is active. */
export function toolStatus(isActive: () => boolean, status: () => { text: string; tone?: StatusTone }): void {
  $effect(() => {
    if (!isActive()) return
    const s = status()
    layout.setStatus(s.text, s.tone ?? 'neutral')
  })
}
