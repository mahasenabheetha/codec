// Platform helpers: shortcuts use "Mod" to mean ⌘ on macOS and Ctrl
// everywhere else, so one definition works on both.

export const isMac =
  typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)

const macSymbols: Record<string, string> = {
  mod: '⌘',
  ctrl: '⌃',
  alt: '⌥',
  shift: '⇧',
  enter: '↵',
  escape: 'Esc',
}

const otherNames: Record<string, string> = {
  mod: 'Ctrl',
  ctrl: 'Ctrl',
  alt: 'Alt',
  shift: 'Shift',
  enter: 'Enter',
  escape: 'Esc',
}

/** Split "Mod+Shift+K" into display keys for the current platform. */
export function shortcutKeys(combo: string): string[] {
  const names = isMac ? macSymbols : otherNames
  return combo.split('+').map((part) => {
    const k = part.trim().toLowerCase()
    return names[k] ?? (k.length === 1 ? k.toUpperCase() : part)
  })
}

/** "Mod+K" → "Ctrl+K" or "⌘K", for tooltips and titles. */
export function formatShortcut(combo: string): string {
  return shortcutKeys(combo).join(isMac ? '' : '+')
}
