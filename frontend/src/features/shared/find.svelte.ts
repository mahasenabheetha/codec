// Ctrl+F for a pane with a find bar (the input and output panes): it
// opens the bar while the pointer is over the pane or focus is inside
// it; elsewhere Ctrl+F stays the browser's. It runs before the code
// editor sees the key, so the editor's own search panel never opens in
// its place.

interface Target {
  /** The pane's element. */
  readonly pane: HTMLElement | undefined
  /** The pointer is over the pane. */
  readonly over: boolean
  /** There is something to search. */
  readonly enabled: boolean
}

/** Call during component init; `open` shows the find bar. */
export function findShortcut(target: Target, open: () => void): void {
  $effect(() => {
    const onkey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== 'f' || !target.enabled) return
      if (!target.over && !target.pane?.contains(document.activeElement)) return
      e.preventDefault()
      e.stopPropagation()
      open()
    }
    window.addEventListener('keydown', onkey, true)
    return () => window.removeEventListener('keydown', onkey, true)
  })
}
