// Break reminders (decision 96): eye breaks, stretches, water, posture
// and your own, each on its own interval, independent of the focus
// timer but aware of it. One scheduler for all of them, driven by the
// Wellness clock; it is loaded with the app shell (BreakLayer), so
// reminders run whichever tool is open.
import { persisted } from '../../lib/stores/persist.svelte'
import { toasts } from '../../lib/stores/toast.svelte'
import { hold, Timer } from './clock.svelte'
import { focus } from './focus.svelte'
import { type Chime, unlockAudio } from './sound'
import { ring } from './wellness.svelte'
import { notify } from './notify'

/** How a reminder shows: the whole window, a card in the corner, or a
 *  toast (a nudge with no countdown). */
export type Style = 'window' | 'card' | 'toast'
/** What a break shows while it counts down. */
export type Guide = 'breathe' | 'look' | 'stretch' | 'water' | 'posture' | 'none'
export type Rhythm = 'calm' | 'box' | '478'
export type Kind = 'eye' | 'stretch' | 'water' | 'posture' | 'custom'

export interface Reminder {
  id: string
  kind: Kind
  name: string
  text: string
  enabled: boolean
  /** Minutes between reminders. */
  every: number
  /** Seconds the break lasts; 0 for a nudge. */
  length: number
  style: Style
  guide: Guide
  /** 'default' is the chime chosen under Sound. */
  chime: Chime | 'default' | 'none'
}

export const builtIns: Reminder[] = [
  {
    id: 'eye',
    kind: 'eye',
    name: 'Eye break',
    text: 'Look at something about 6 metres (20 feet) away and let your eyes relax.',
    enabled: false,
    every: 20,
    length: 20,
    style: 'card',
    guide: 'look',
    chime: 'default',
  },
  {
    id: 'stretch',
    kind: 'stretch',
    name: 'Stretch',
    text: 'Stand up, loosen up, and follow the moves.',
    enabled: false,
    every: 50,
    length: 180,
    style: 'window',
    guide: 'stretch',
    chime: 'default',
  },
  {
    id: 'water',
    kind: 'water',
    name: 'Water',
    text: 'Time for a glass of water.',
    enabled: false,
    every: 60,
    length: 0,
    style: 'toast',
    guide: 'water',
    chime: 'soft',
  },
  {
    id: 'posture',
    kind: 'posture',
    name: 'Posture',
    text: 'Feet flat, shoulders down, screen at eye level.',
    enabled: false,
    every: 30,
    length: 0,
    style: 'toast',
    guide: 'posture',
    chime: 'soft',
  },
]

/** Seconds a reminder lasts when it changes from a nudge (a toast) to a
 *  break on screen, so it doesn't stay at 0 and silently remain a toast. */
export const screenLength: Record<Kind, number> = { eye: 20, stretch: 180, water: 20, posture: 20, custom: 30 }

export const rhythms: { value: Rhythm; label: string; steps: { label: string; s: number; scale: number }[] }[] = [
  { value: 'calm', label: 'Calm 4-6', steps: [{ label: 'Breathe in', s: 4, scale: 1 }, { label: 'Breathe out', s: 6, scale: 0.55 }] },
  {
    value: 'box',
    label: 'Box 4-4-4-4',
    steps: [
      { label: 'Breathe in', s: 4, scale: 1 },
      { label: 'Hold', s: 4, scale: 1 },
      { label: 'Breathe out', s: 4, scale: 0.55 },
      { label: 'Hold', s: 4, scale: 0.55 },
    ],
  },
  {
    value: '478',
    label: '4-7-8',
    steps: [
      { label: 'Breathe in', s: 4, scale: 1 },
      { label: 'Hold', s: 7, scale: 1 },
      { label: 'Breathe out', s: 8, scale: 0.55 },
    ],
  },
]

interface Prefs {
  list: Reminder[]
  /** Wait for a focus session to end before reminding. */
  holdDuringFocus: boolean
  rhythm: Rhythm
  hours: { on: boolean; from: number; to: number; weekdays: boolean }
}

const defaults: Prefs = {
  list: builtIns,
  holdDuringFocus: true,
  rhythm: 'calm',
  hours: { on: false, from: 9, to: 18, weekdays: true },
}

const prefsStore = persisted<Prefs>('wellness.reminders', defaults, { sync: true })
{
  // Merge over the defaults, and keep every built-in even if an older
  // version stored fewer (a later one may add some).
  const stored = { ...defaults, ...prefsStore.value }
  const list = [...stored.list]
  for (const b of builtIns) if (!list.some((r) => r.id === b.id)) list.push(b)
  // Water and posture had no picture of their own before 3.6.0 was out;
  // one stored without a guide takes its own.
  const own: Partial<Record<Kind, Guide>> = { water: 'water', posture: 'posture' }
  prefsStore.value = {
    ...stored,
    list: list.map((r) => {
      const merged = { ...(builtIns.find((b) => b.id === r.id) ?? {}), ...r }
      return own[merged.kind] && merged.guide === 'none' ? { ...merged, guide: own[merged.kind]! } : merged
    }),
  }
}
/** When each reminder is next due (ms since epoch). */
const nextStore = persisted<Record<string, number>>('wellness.reminders.next', {}, { sync: true })
/** Reminders paused until then (0: not paused). */
const pauseStore = persisted<number>('wellness.reminders.pause', 0, { sync: true })

/** A reminder due this long ago (the laptop slept) is skipped, not shown
 *  late: the time away was a break. */
const LATE_MS = 10 * 60_000

export interface Active {
  reminder: Reminder
  timer: Timer
  /** A test from the settings: shown, but the schedule doesn't move. */
  test: boolean
}

class Reminders {
  current = $state<Active | null>(null)
  /** This tab runs the schedule. With several codec tabs open, one holds
   *  a Web Lock and reminds; the others follow its settings and take
   *  over when it closes. Without Web Locks every tab is on its own. */
  leader = $state(typeof navigator === 'undefined' || !navigator.locks)
  #release: (() => void) | undefined

  get prefs(): Prefs {
    return prefsStore.value
  }
  get next(): Record<string, number> {
    return nextStore.value
  }
  get pausedUntil(): number {
    return pauseStore.value
  }

  enabled = $derived(this.prefs.list.filter((r) => r.enabled))

  /** The reminder due soonest, and when. */
  upcoming = $derived.by(() => {
    let best: { reminder: Reminder; at: number } | null = null
    for (const r of this.enabled) {
      const at = this.next[r.id]
      if (at && (!best || at < best.at)) best = { reminder: r, at }
    }
    return best
  })

  constructor() {
    navigator.locks?.request('codec-wellness-reminders', () => {
      this.leader = true
      return new Promise<void>(() => {}) // held while this tab lives
    })
    $effect.root(() => {
      // Schedule what was switched on, forget what was switched off, and
      // keep the clock ticking while anything is on (the other tabs tick
      // too, for their countdowns). Only the tab holding the lock writes
      // the schedule: two tabs that briefly disagreed would otherwise
      // rewrite it back and forth through the storage events.
      $effect(() => {
        const on = this.enabled
        if (this.leader) {
          const next = { ...nextStore.value }
          let changed = false
          for (const r of on) {
            if (!next[r.id]) (next[r.id] = Date.now() + r.every * 60_000), (changed = true)
          }
          for (const id of Object.keys(next)) {
            if (!on.some((r) => r.id === id)) delete next[id], (changed = true)
          }
          if (changed) nextStore.value = next
        }
        if (on.length && !this.#release) this.#release = hold(() => this.#tick())
        if (!on.length && this.#release) this.#release(), (this.#release = undefined)
      })
    })
  }

  /** True when reminders may show now (not paused, within active hours). */
  allowed(now = Date.now()): boolean {
    if (this.pausedUntil > now) return false
    const h = this.prefs.hours
    if (!h.on) return true
    const d = new Date(now)
    if (h.weekdays && (d.getDay() === 0 || d.getDay() === 6)) return false
    const hour = d.getHours() + d.getMinutes() / 60
    return hour >= h.from && hour < h.to
  }

  #schedule(r: Reminder, fromNow = r.every * 60_000) {
    nextStore.value = { ...nextStore.value, [r.id]: Date.now() + fromNow }
  }

  #tick() {
    if (!this.leader) return
    const now = Date.now()
    for (const r of this.enabled) {
      const at = this.next[r.id]
      if (!at || at > now) continue
      if (now - at > LATE_MS || !this.allowed(now)) {
        this.#schedule(r) // asleep or outside hours: start the interval over
        continue
      }
      if (focus.active && focus.phase !== 'focus') {
        this.#schedule(r) // a focus break is already a break
        continue
      }
      if (this.prefs.holdDuringFocus && focus.active && focus.phase === 'focus') continue // after the session
      if (this.current) continue // after the break on screen
      this.show(r)
      return // one at a time
    }
  }

  /** Show a reminder now. A test leaves the schedule alone. */
  show(r: Reminder, test = false) {
    if (r.chime !== 'none') ring(test ? `${r.name} (Try it)` : `${r.name} reminder`, r.chime === 'default' ? undefined : r.chime)
    notify(r.name, r.text, 'codec-break-' + r.id)
    if (r.style === 'toast' || r.length === 0) {
      toasts.show(`${r.name}: ${r.text}`, 'neutral', 9000, {
        label: 'Snooze 5 min',
        run: () => this.#schedule(r, 5 * 60_000),
      })
      if (!test) this.#schedule(r)
      return
    }
    const timer = new Timer((late) => this.finish(!late))
    this.current = { reminder: r, timer, test }
    timer.start(r.length * 1000)
  }

  /** Close the break on screen. A break taken (or skipped) covers any
   *  shorter one that has come due meanwhile. */
  finish(taken = true) {
    const c = this.current
    if (!c) return
    c.timer.reset()
    this.current = null
    if (c.test) return
    this.#schedule(c.reminder)
    const now = Date.now()
    for (const r of this.enabled) {
      if (r.id !== c.reminder.id && (this.next[r.id] ?? Infinity) <= now && r.length <= c.reminder.length) this.#schedule(r)
    }
    if (taken && c.reminder.chime !== 'none') ring(`${c.reminder.name} break done`, 'soft') // a gentle "done"
  }

  snooze(minutes = 5) {
    const c = this.current
    if (!c) return
    c.timer.reset()
    this.current = null
    if (!c.test) this.#schedule(c.reminder, minutes * 60_000)
  }

  /** Take the longest enabled break now (or the stretch if none is on). */
  breakNow() {
    unlockAudio()
    const withLength = this.enabled.filter((r) => r.length > 0).sort((a, b) => b.length - a.length)
    this.show(withLength[0] ?? this.prefs.list.find((r) => r.id === 'stretch')!)
  }

  /** Pause every reminder for `ms`, or until tomorrow morning. */
  pause(ms: number | 'tomorrow') {
    if (ms === 'tomorrow') {
      const d = new Date()
      d.setDate(d.getDate() + 1)
      d.setHours(this.prefs.hours.on ? this.prefs.hours.from : 6, 0, 0, 0)
      pauseStore.value = d.getTime()
    } else {
      pauseStore.value = Date.now() + ms
    }
  }

  resume() {
    pauseStore.value = 0
    // Start the intervals over, so everything doesn't fire at once.
    for (const r of this.enabled) this.#schedule(r)
  }

  /** Restart one reminder's interval (after its settings change). */
  restart(r: Reminder) {
    if (r.enabled) this.#schedule(r)
  }

  add(): Reminder {
    const r: Reminder = {
      id: 'custom-' + Date.now().toString(36),
      kind: 'custom',
      name: 'My reminder',
      text: 'Take a moment.',
      enabled: true,
      every: 45,
      length: 60,
      style: 'card',
      guide: 'breathe',
      chime: 'default',
    }
    unlockAudio()
    this.prefs.list.push(r)
    return r
  }

  remove(id: string) {
    this.prefs.list = this.prefs.list.filter((r) => r.id !== id)
  }
}

export const reminders = new Reminders()

