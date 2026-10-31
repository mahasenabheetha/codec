// Today's focus sessions and breaks (decision 96), kept in the browser's
// storage until the day changes, then dropped: no history beyond today.
import { persisted } from '../../lib/stores/persist.svelte'

export interface Block {
  kind: 'focus' | 'break'
  start: number
  end: number
  /** A focus session that ran its full length (skipped ones don't). */
  complete?: boolean
}

interface Day {
  date: string // local yyyy-mm-dd
  blocks: Block[]
}

function dateOf(t: number): string {
  const d = new Date(t)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const store = persisted<Day>('wellness.today', { date: dateOf(Date.now()), blocks: [] })

/** Today's record for writing, starting a new one when the date has
 *  moved on. */
function day(): Day {
  const d = dateOf(Date.now())
  if (store.value.date !== d) store.value = { date: d, blocks: [] }
  return store.value
}

class Today {
  /** Today's blocks; none when the stored day is an earlier one. Reading
   *  never writes, so it is safe inside $derived. */
  blocks = $derived(store.value.date === dateOf(Date.now()) ? store.value.blocks : [])

  /** Focus sessions that ran their full length. */
  sessions = $derived(this.blocks.filter((b) => b.kind === 'focus' && b.complete).length)
  /** Minutes focused, counting sessions cut short. */
  focusMinutes = $derived(Math.round(this.blocks.filter((b) => b.kind === 'focus').reduce((n, b) => n + (b.end - b.start), 0) / 60_000))
  breaks = $derived(this.blocks.filter((b) => b.kind === 'break').length)

  /** A focus session; `worked` ms of it were focus (pauses excluded). */
  addFocus(start: number, end: number, worked: number, complete: boolean) {
    if (worked < 30_000) return // a mis-click, not a session
    day().blocks.push({ kind: 'focus', start: Math.max(start, end - worked), end, complete })
  }

  addBreak(start: number, end: number) {
    day().blocks.push({ kind: 'break', start, end })
  }

  clear() {
    store.value = { date: dateOf(Date.now()), blocks: [] }
  }
}

export const today = new Today()
