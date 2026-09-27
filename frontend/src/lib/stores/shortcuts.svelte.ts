// Global keyboard shortcut registry. One keydown listener dispatches to
// registered combos, so features never attach their own window
// listeners and conflicts are easy to spot in one place.
//
// Combo syntax: "Mod+K", "Alt+C", "Mod+Enter", "Escape", "?".
// "Mod" is ⌘ on macOS and Ctrl elsewhere.

import { isMac } from '../utils/platform'

export interface ShortcutOptions {
  /** Only fire while this returns true (e.g. the owning tool is active). */
  when?: () => boolean
  /** Fire even while typing in an input or editor. Defaults to true
   *  for combos with a modifier and false for bare keys like "?". */
  inInputs?: boolean
}

interface Entry {
  combo: Parsed
  handler: (e: KeyboardEvent) => void
  opts: ShortcutOptions
}

interface Parsed {
  mod: boolean
  alt: boolean
  shift: boolean
  key: string
}

function parse(combo: string): Parsed {
  const parts = combo.split('+').map((p) => p.trim().toLowerCase())
  const key = parts[parts.length - 1]
  return {
    mod: parts.includes('mod'),
    alt: parts.includes('alt'),
    shift: parts.includes('shift'),
    key,
  }
}

// Letters are matched by physical key (e.code) so Alt+C still works on
// macOS, where Alt changes e.key to "ç".
function eventKey(e: KeyboardEvent): string {
  if (e.code.startsWith('Key')) return e.code.slice(3).toLowerCase()
  return e.key.toLowerCase()
}

function matches(p: Parsed, e: KeyboardEvent): boolean {
  const mod = isMac ? e.metaKey : e.ctrlKey
  if (p.key === '?') return e.key === '?' && !mod && !e.altKey
  return (
    mod === p.mod &&
    e.altKey === p.alt &&
    e.shiftKey === p.shift &&
    eventKey(e) === p.key
  )
}

function isTyping(e: KeyboardEvent): boolean {
  const t = e.target as HTMLElement | null
  if (!t) return false
  return (
    t.isContentEditable ||
    t.tagName === 'INPUT' ||
    t.tagName === 'TEXTAREA' ||
    t.tagName === 'SELECT'
  )
}

/** True while a modal dialog or the command palette is open. */
function modalOpen(): boolean {
  return document.querySelector('[role="dialog"][data-state="open"]') !== null
}

const entries: Entry[] = []

window.addEventListener('keydown', (e) => {
  // Something closer to the focus (an editor command, an open dropdown
  // handling Escape) already consumed this key.
  if (e.defaultPrevented) return
  // Newest registration wins, so a focused feature can shadow a global.
  for (let i = entries.length - 1; i >= 0; i--) {
    const { combo, handler, opts } = entries[i]
    if (!matches(combo, e)) continue
    if (opts.when && !opts.when()) continue
    const inInputs = opts.inInputs ?? (combo.mod || combo.alt)
    if (!inInputs && isTyping(e)) continue
    // Bare keys inside CodeMirror's search panel belong to the panel
    // (e.g. Escape closes it) rather than to the app.
    if (!combo.mod && !combo.alt && (e.target as Element | null)?.closest?.('.cm-panels')) continue
    // Dialogs own Escape and plain keys while open.
    if (modalOpen() && !combo.mod) continue
    e.preventDefault()
    handler(e)
    return
  }
})

/** Register a shortcut; returns a function that removes it. Call it
 *  from a component's $effect so cleanup happens automatically. */
export function shortcut(
  combo: string,
  handler: (e: KeyboardEvent) => void,
  opts: ShortcutOptions = {},
): () => void {
  const entry: Entry = { combo: parse(combo), handler, opts }
  entries.push(entry)
  return () => {
    const i = entries.indexOf(entry)
    if (i >= 0) entries.splice(i, 1)
  }
}
