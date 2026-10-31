// The Wellness clock (decision 96): one ticker for every timer. A timer
// is an end time, not a count of ticks, so a throttled background tab or
// a sleeping laptop can't make it drift: the time left is always worked
// out from the wall clock. The ticker only runs while a timer does.

const TICK_MS = 250

/** Noticed this long after its end (the laptop slept), a timer doesn't
 *  ring: it is offered as a fresh start instead of a stale alarm. */
export const STALE_MS = 5 * 60_000

let now = $state(Date.now())
const listeners = new Set<() => void>()
let interval: ReturnType<typeof setInterval> | undefined

function tick() {
  now = Date.now()
  for (const l of listeners) l()
}

/** Keep the clock ticking while `onTick` listens; returns the release. */
function hold(onTick: () => void): () => void {
  listeners.add(onTick)
  if (!interval) {
    interval = setInterval(tick, TICK_MS)
    // A tab coming back to the front catches up at once.
    document.addEventListener('visibilitychange', tick)
  }
  return () => {
    listeners.delete(onTick)
    if (listeners.size || !interval) return
    clearInterval(interval)
    interval = undefined
    document.removeEventListener('visibilitychange', tick)
  }
}

export const clock = {
  /** The time of the last tick; reactive. */
  get now(): number {
    return now
  },
}

export type TimerState = 'idle' | 'running' | 'paused' | 'done'

/** What a timer needs to carry on after a reload. */
export interface TimerSnapshot {
  state: TimerState
  duration: number
  endsAt: number
  pausedLeft: number
}

/** A countdown. `onEnd` runs once when it reaches zero; `late` is true
 *  when that was noticed only after STALE_MS (no alarm is wanted then). */
export class Timer {
  state = $state<TimerState>('idle')
  /** The whole length, including time added with add(). */
  duration = $state(0)
  /** True when the timer ran out unnoticed, e.g. while the laptop slept. */
  stale = $state(false)
  #endsAt = $state(0)
  #pausedLeft = $state(0)
  #release: (() => void) | undefined
  #onEnd: (late: boolean) => void

  constructor(onEnd: (late: boolean) => void = () => {}) {
    this.#onEnd = onEnd
  }

  /** Milliseconds left; reactive while running. */
  left = $derived.by(() => {
    if (this.state === 'running') return Math.max(0, this.#endsAt - clock.now)
    if (this.state === 'paused') return this.#pausedLeft
    if (this.state === 'done') return 0
    return this.duration
  })

  /** 0 at the start, 1 at the end. */
  progress = $derived(this.duration > 0 ? Math.min(1, 1 - this.left / this.duration) : 0)

  start(ms: number) {
    this.duration = ms
    this.#endsAt = Date.now() + ms
    this.stale = false
    this.state = 'running'
    this.#run()
  }

  pause() {
    if (this.state !== 'running') return
    this.#pausedLeft = Math.max(0, this.#endsAt - Date.now())
    this.state = 'paused'
    this.#stop()
  }

  resume() {
    if (this.state !== 'paused') return
    this.#endsAt = Date.now() + this.#pausedLeft
    this.state = 'running'
    this.#run()
  }

  /** More time (or less, with a negative ms) on a running or paused timer. */
  add(ms: number) {
    if (this.state === 'running') {
      this.#endsAt += ms
      this.duration = Math.max(1, this.duration + ms)
      this.#check()
    } else if (this.state === 'paused') {
      this.#pausedLeft = Math.max(0, this.#pausedLeft + ms)
      this.duration = Math.max(1, this.duration + ms)
    }
  }

  reset() {
    this.#stop()
    this.state = 'idle'
    this.stale = false
  }

  snapshot(): TimerSnapshot {
    return { state: this.state, duration: this.duration, endsAt: this.#endsAt, pausedLeft: this.#pausedLeft }
  }

  /** Carry on from a snapshot. A timer that ran out while codec was closed
   *  ends as stale, without an alarm. */
  restore(s: TimerSnapshot) {
    this.duration = s.duration
    this.#endsAt = s.endsAt
    this.#pausedLeft = s.pausedLeft
    this.state = s.state
    if (s.state === 'running') this.#run()
  }

  #run() {
    this.#release ??= hold(() => this.#check())
    this.#check()
  }

  #stop() {
    this.#release?.()
    this.#release = undefined
  }

  #check() {
    if (this.state !== 'running') return
    const over = Date.now() - this.#endsAt
    if (over < 0) return
    this.#stop()
    this.state = 'done'
    this.stale = over > STALE_MS
    this.#onEnd(this.stale)
  }
}

/** 25:00, 4:05, 1:30:00: whole seconds, rounded up, so 0:00 means done. */
export function formatClock(ms: number): string {
  const total = Math.ceil(Math.max(0, ms) / 1000)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const ss = String(s).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}
