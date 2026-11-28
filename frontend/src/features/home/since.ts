// How long ago something was opened, as Home's Opened lately says it:
// "now", "12 min", "2 h", "today", "yesterday", "3 days", then the date.

const MIN = 60_000
const HOUR = 60 * MIN

/** A short "how long ago" for a time in ms; '' when it isn't known (0). */
export function since(at: number, now: number): string {
  if (!at) return ''
  const d = now - at
  if (d < MIN) return 'now'
  if (d < HOUR) return `${Math.floor(d / MIN)} min`
  if (d < 3 * HOUR) return `${Math.floor(d / HOUR)} h`
  const days = dayNumber(now) - dayNumber(at)
  if (days <= 0) return 'today'
  if (days === 1) return 'yesterday'
  if (days < 7) return `${days} days`
  return new Date(at).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
}

// Days since the epoch in local time, so "today" ends at midnight; the
// rounding absorbs a daylight-saving hour.
function dayNumber(t: number): number {
  const d = new Date(t)
  return Math.round(new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime() / 86_400_000)
}
