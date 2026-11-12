// Wiring every tool shares: keyboard shortcuts, palette commands and
// the status-bar line — all scoped to the tool's active tab. Call these
// during component init; cleanup is automatic.

import { untrack } from 'svelte'
import { shortcut } from '../../lib/stores/shortcuts.svelte'
import { commands, type Command } from '../../lib/stores/commands.svelte'
import { layout, type StatusTone } from '../../lib/stores/layout.svelte'
import { toasts } from '../../lib/stores/toast.svelte'

export interface ToolActions {
  run: () => void
  copy: () => void
  clear: () => void
  swap?: () => void
}

/** Escape clears a tool from its main input (the editor, or a field
 *  marked `data-clears`) and from anywhere that isn't a field. Typed in
 *  any other field (an option, a key, a filter) it is that field's own:
 *  it used to wipe the whole tool from there (3.10.2). */
function inOtherField(e: KeyboardEvent): boolean {
  const t = e.target as HTMLElement | null
  if (!t || t.closest('[data-clears]')) return false
  return t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT'
}

/** "Cleared · Undo": what a clear took away comes back for a few
 *  seconds. The text is held in memory only, until the toast goes. */
export function cleared(undo: () => void, message = 'Cleared'): void {
  toasts.show(message, 'neutral', 7000, { label: 'Undo', run: undo })
}

/** The v1 shortcuts: Mod+Enter, Alt+C, Esc (+ Alt+S swap). */
export function toolShortcuts(isActive: () => boolean, a: ToolActions): void {
  $effect(() => {
    const offs = [
      shortcut('Mod+Enter', a.run, { when: isActive }),
      shortcut('Alt+C', a.copy, { when: isActive }),
      shortcut('Escape', a.clear, { when: isActive, inInputs: true, skip: inOtherField }),
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
