// The focus timer (decision 96): focus sessions and breaks in rounds,
// with a long break after the last round. One module-level store, so the
// timer keeps running whichever tab is in front. The lengths and choices
// persist; a running session persists too, so a reload carries on; the
// task label lives in session storage only (gone with the window).
import { persisted } from '../../lib/stores/persist.svelte'
import { Timer, type TimerSnapshot } from './clock.svelte'
import { today } from './today.svelte'
import { ring } from './wellness.svelte'
import { unlockAudio } from './sound'

export type Phase = 'focus' | 'short' | 'long'
export type PresetId = 'classic' | 'deep' | 'flow' | 'custom'

/** Lengths in minutes. */
export interface Lengths {
  focus: number
  short: number
  long: number
  /** Focus sessions before the long break. */
  rounds: number
}

export const presets: { id: Exclude<PresetId, 'custom'>; label: string; hint: string; lengths: Lengths }[] = [
  { id: 'classic', label: 'Classic', hint: '25 / 5', lengths: { focus: 25, short: 5, long: 15, rounds: 4 } },
  { id: 'deep', label: 'Deep work', hint: '50 / 10', lengths: { focus: 50, short: 10, long: 20, rounds: 3 } },
  { id: 'flow', label: 'Flow', hint: '90 / 20', lengths: { focus: 90, short: 20, long: 30, rounds: 2 } },
]

export const limits: Record<keyof Lengths, [number, number]> = {
  focus: [1, 180],
  short: [1, 60],
  long: [1, 90],
  rounds: [1, 12],
}

export const phaseLabel: Record<Phase, string> = { focus: 'Focus', short: 'Short break', long: 'Long break' }

interface Prefs {
  preset: PresetId
  custom: Lengths
  /** Start the next phase by itself when one ends. */
  autoStart: boolean
}

interface Run {
  phase: Phase
  round: number
  timer: TimerSnapshot
  /** When the current phase was first started, for Today. */
  startedAt: number
}

const defaults: Prefs = { preset: 'classic', custom: { focus: 30, short: 5, long: 15, rounds: 4 }, autoStart: false }
const prefsStore = persisted<Prefs>('wellness.focus', defaults)
prefsStore.value = { ...defaults, ...prefsStore.value }
const runStore = persisted<Run | null>('wellness.focus.run', null)

/** What just happened, for the card under the ring. */
export type Notice =
  | { kind: 'ended'; phase: Phase; next: Phase }
  | { kind: 'stale'; phase: Phase }
  | null

function readTask(): string {
  try {
    return sessionStorage.getItem('codec:wellness.task') ?? ''
  } catch {
    return ''
  }
}

class Focus {
  phase = $state<Phase>('focus')
  round = $state(1)
  notice = $state<Notice>(null)
  task = $state(readTask())
  #startedAt = 0
  timer = new Timer((late) => this.#ended(late))

  get prefs(): Prefs {
    return prefsStore.value
  }

  lengths = $derived<Lengths>(
    this.prefs.preset === 'custom' ? this.prefs.custom : presets.find((p) => p.id === this.prefs.preset)!.lengths,
  )

  /** Milliseconds in a phase. */
  length(phase: Phase): number {
    return this.lengths[phase] * 60_000
  }

  /** The phase after this one. */
  nextPhase(phase = this.phase, round = this.round): Phase {
    if (phase !== 'focus') return 'focus'
    return round >= this.lengths.rounds ? 'long' : 'short'
  }

  running = $derived(this.timer.state === 'running')
  /** Started and not finished: running or paused. */
  active = $derived(this.timer.state === 'running' || this.timer.state === 'paused')
  /** Time left, or the whole phase when it hasn't started. */
  left = $derived(this.active ? this.timer.left : this.length(this.phase))

  constructor() {
    const run = runStore.value
    if (run) {
      this.phase = run.phase
      this.round = run.round
      this.#startedAt = run.startedAt
      this.timer.restore(run.timer)
    }
    $effect.root(() => {
      // Persist the running session; drop it once nothing is under way.
      $effect(() => {
        const t = this.timer
        const live = t.state === 'running' || t.state === 'paused'
        runStore.value = live
          ? { phase: this.phase, round: this.round, startedAt: this.#startedAt, timer: { ...t.snapshot(), state: t.state } }
          : null
      })
      $effect(() => {
        try {
          if (this.task) sessionStorage.setItem('codec:wellness.task', this.task)
          else sessionStorage.removeItem('codec:wellness.task')
        } catch {
          // session storage blocked: the label just doesn't survive a reload
        }
      })
    })
  }

  /** Start the current phase from its full length. */
  start() {
    unlockAudio()
    this.notice = null
    this.#startedAt = Date.now()
    this.timer.start(this.length(this.phase))
  }

  toggle() {
    if (this.timer.state === 'running') this.timer.pause()
    else if (this.timer.state === 'paused') this.timer.resume()
    else this.start()
  }

  extend(minutes = 5) {
    if (this.active) this.timer.add(minutes * 60_000)
  }

  /** Move to the next phase now. A focus session cut short still counts
   *  its minutes in Today, but not as a session. */
  skip() {
    if (this.active && this.phase === 'focus') {
      today.addFocus(this.#startedAt, Date.now(), this.timer.duration - this.timer.left, false)
    }
    this.#advance()
    this.timer.reset()
    this.notice = null
    if (this.prefs.autoStart) this.start()
  }

  /** Back to the first round's focus, stopped. */
  reset() {
    this.timer.reset()
    this.phase = 'focus'
    this.round = 1
    this.notice = null
  }

  /** Choose a preset or custom lengths; a stopped timer takes them at once. */
  setPreset(id: PresetId) {
    this.prefs.preset = id
    if (!this.active) this.timer.reset()
  }

  #advance() {
    const next = this.nextPhase()
    if (this.phase === 'focus') {
      if (next === 'long') this.round = 1
    } else if (this.phase === 'short') {
      this.round = Math.min(this.round + 1, this.lengths.rounds)
    }
    this.phase = next
  }

  #ended(late: boolean) {
    const phase = this.phase
    if (late) {
      // Ran out unnoticed (a closed laptop): no alarm, no credit; offer
      // a fresh start of the same phase.
      this.timer.reset()
      this.notice = { kind: 'stale', phase }
      return
    }
    if (phase === 'focus') today.addFocus(this.#startedAt, Date.now(), this.timer.duration, true)
    else today.addBreak(this.#startedAt, Date.now())
    ring()
    this.#advance()
    this.timer.reset()
    this.notice = { kind: 'ended', phase, next: this.phase }
    if (this.prefs.autoStart) this.start()
  }
}

export const focus = new Focus()
