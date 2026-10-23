// How the Log viewer writes times, spans and counts.

import type { TimeMode } from './viewer.svelte'

const pad = (n: number, w = 2) => String(n).padStart(w, '0')

/** A row's time: clock time (with the date when the log spans days),
 *  in local time or UTC, or the time since the log's first entry. */
export function formatTime(iso: string | undefined, mode: TimeMode, start: number, days: boolean): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (mode === 'relative') return '+' + formatSpan(d.getTime() - start, true)
  const u = mode === 'utc'
  const date = days ? `${pad((u ? d.getUTCMonth() : d.getMonth()) + 1)}-${pad(u ? d.getUTCDate() : d.getDate())} ` : ''
  const h = u ? d.getUTCHours() : d.getHours()
  const m = u ? d.getUTCMinutes() : d.getMinutes()
  const s = u ? d.getUTCSeconds() : d.getSeconds()
  return `${date}${pad(h)}:${pad(m)}:${pad(s)}.${pad(d.getMilliseconds(), 3)}`
}

/** A duration: 850 ms, 12.3 s, 4 m 05 s, 2 h 10 m, 3 d 4 h. */
export function formatSpan(ms: number, compact = false): string {
  const neg = ms < 0
  ms = Math.abs(ms)
  const sign = neg ? '-' : ''
  const sp = compact ? '' : ' '
  if (ms < 1000) return `${sign}${Math.round(ms)}${sp}ms`
  if (ms < 60_000) return `${sign}${(ms / 1000).toFixed(ms < 10_000 ? 3 : 1)}${sp}s`
  const s = Math.floor(ms / 1000)
  if (s < 3600) return `${sign}${Math.floor(s / 60)}${sp}m${sp}${pad(s % 60)}${sp}s`
  if (s < 86_400) return `${sign}${Math.floor(s / 3600)}${sp}h${sp}${pad(Math.floor(s / 60) % 60)}${sp}m`
  return `${sign}${Math.floor(s / 86_400)}${sp}d${sp}${Math.floor(s / 3600) % 24}${sp}h`
}

export const count = (n: number) => n.toLocaleString('en-US')

/** A date and time for summaries, to the second. */
export function formatWhen(iso: string, mode: TimeMode): string {
  const d = new Date(iso)
  if (mode === 'utc') return d.toISOString().slice(0, 19).replace('T', ' ') + ' UTC'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** A full time to the millisecond, for a report: local or UTC (said so). */
export function formatStamp(iso: string | undefined, mode: TimeMode): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (mode === 'utc') return d.toISOString().replace('T', ' ').replace('Z', ' UTC')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`
}

/** The colours of the four highlight rules, in order. */
export const ruleTones = ['var(--syn-anchor)', 'var(--syn-bool)', 'var(--syn-number)', 'var(--syn-expr-rt)']

/** The first highlight rule a row matches (its bits), or -1. */
export const firstRule = (hl = 0) => (hl ? Math.log2(hl & -hl) : -1)

/** Level → the colour token it is drawn with. */
export const levelTone: Record<string, string> = {
  fatal: 'var(--err)',
  error: 'var(--err)',
  warn: 'var(--warn)',
  info: 'var(--info)',
  debug: 'var(--fg-2)',
  trace: 'var(--fg-2)',
  '': 'var(--border-strong)',
}
