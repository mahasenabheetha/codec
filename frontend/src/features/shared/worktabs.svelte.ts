// Work tabs (decision 97): several pastes, comparisons or logs open at
// once in one tool, each tab with its own state. The Log viewer, Smart
// paste, Base64 and Compare share this store and the strip that shows
// it (WorkTabStrip.svelte). Tabs live for the window only: nothing
// pasted is stored (19).
import { shortcut } from '../../lib/stores/shortcuts.svelte'
import { toasts } from '../../lib/stores/toast.svelte'

let nextKey = 0

/** A key for a new tab, unique for the window. */
export const newTabKey = () => ++nextKey

export class WorkTabs<T extends { readonly key: number }> {
  list = $state.raw<T[]>([])
  current = $state.raw<T | null>(null)
  #make: (() => T) | undefined
  #keepOne: boolean
  #undoable: ((t: T) => boolean) | undefined

  /** `make` builds a fresh tab for "+"; with `keepOne` there is always at
   *  least one (closing the last opens a fresh one). `undoable` says a
   *  tab holds something that exists nowhere else (pasted text, 19):
   *  closing it offers Undo for a few seconds. */
  constructor(make?: () => T, opts: { keepOne?: boolean; undoable?: (t: T) => boolean } = {}) {
    this.#make = make
    this.#keepOne = opts.keepOne ?? false
    this.#undoable = opts.undoable
    if (this.#keepOne && make) this.add()
  }

  /** Add a tab (a fresh one by default) and show it. */
  add(t?: T): T {
    const tab = t ?? this.#make!()
    this.list = [...this.list, tab]
    this.current = tab
    return tab
  }

  select(t: T) {
    this.current = t
  }

  /** The next (1) or previous (-1) tab, wrapping round. */
  step(dir: 1 | -1) {
    if (!this.list.length) return
    const i = this.current ? this.list.indexOf(this.current) : -1
    this.current = this.list[(i + dir + this.list.length) % this.list.length]
  }

  /** Close a tab; the one beside it is shown. */
  close(t: T) {
    const i = this.list.indexOf(t)
    if (i < 0) return
    const undo = this.#undoable?.(t) ?? false
    this.list = this.list.filter((x) => x !== t)
    if (this.current === t) this.current = this.list[Math.min(i, this.list.length - 1)] ?? null
    if (!undo) this.closed(t)
    if (this.#keepOne && !this.list.length && this.#make) this.add()
    if (undo) {
      toasts.show('Tab closed', 'neutral', 8000, {
        label: 'Undo',
        run: () => {
          if (this.list.includes(t)) return
          const list = [...this.list]
          list.splice(Math.min(i, list.length), 0, t)
          this.list = list
          this.current = t
        },
      })
    }
  }

  /** After a tab is closed: let go of what it holds. */
  protected closed(_t: T) {}
}

/** Alt+T opens a tab and Alt+W closes the current one while `isActive`.
 *  (Ctrl+T and Ctrl+W belong to the browser.) Call during component init. */
export function workTabShortcuts<T extends { readonly key: number }>(isActive: () => boolean, tabs: WorkTabs<T>, add: () => void) {
  $effect(() => {
    const offs = [
      shortcut('Alt+T', add, { when: isActive }),
      shortcut(
        'Alt+W',
        () => {
          if (tabs.current) tabs.close(tabs.current)
        },
        { when: isActive },
      ),
    ]
    return () => offs.forEach((off) => off())
  })
}
