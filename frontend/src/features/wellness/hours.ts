// Active hours for break reminders (3.10.0): when the hours are, when
// the current stretch of them ends and the next begins. Pure and
// import-free, so `npm test` runs it in Node without a browser.
//
// A window starts at `from` on a day and ends at `to`; when `to` is not
// after `from` it ends the next day (22 to 6 is the night). With
// `weekdays`, only windows that start Monday to Friday count, so a night
// shift that starts on Friday runs into Saturday morning, and none
// starts on Saturday. `from` and `to` are hours, 9.5 for 09:30.

export interface Hours {
  on: boolean
  from: number
  to: number
  weekdays: boolean
}

export interface Window {
  start: number // ms since epoch
  end: number
}

/** The window that starts on the day of `day` (local time), or null when
 *  none starts that day (a weekend with weekdays only). */
function windowOn(h: Hours, day: Date): Window | null {
  const dow = day.getDay()
  if (h.weekdays && (dow === 0 || dow === 6)) return null
  const at = (d: Date, hour: number) => {
    const t = new Date(d)
    t.setHours(Math.floor(hour), Math.round((hour % 1) * 60), 0, 0)
    return t
  }
  const start = at(day, h.from)
  const endDay = new Date(day)
  if (h.to <= h.from) endDay.setDate(endDay.getDate() + 1) // across midnight (or a whole day when equal)
  return { start: start.getTime(), end: at(endDay, h.to).getTime() }
}

/** The window that holds `t`, or null outside the hours. With the hours
 *  off, every moment is in: null is never returned. */
export function windowAt(h: Hours, t: number): Window | null {
  if (!h.on) return { start: 0, end: Infinity }
  // A window holding t started today or, across midnight, yesterday.
  for (const back of [0, 1]) {
    const day = new Date(t)
    day.setDate(day.getDate() - back)
    const w = windowOn(h, day)
    if (w && w.start <= t && t < w.end) return w
  }
  return null
}

/** When the next window starts after `t` (the one after the current one
 *  when `t` is inside the hours). */
export function nextStart(h: Hours, t: number): number {
  for (let ahead = 0; ahead <= 8; ahead++) {
    const day = new Date(t)
    day.setDate(day.getDate() + ahead)
    const w = windowOn(h, day)
    if (w && w.start > t) return w.start
  }
  return t // no window at all can't happen: a week always has one
}

/** An hour as a clock reads it: 9.5 is 09:30. */
export function hourText(hour: number): string {
  const m = Math.round(hour * 60)
  return `${String(Math.floor(m / 60) % 24).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
}

/** How a window's start reads after "Breaks resume": "at 09:00" today,
 *  "tomorrow at 09:00", or the weekday ("Monday at 09:00"). */
export function startLabel(start: number, now: number, time: (t: number) => string): string {
  const d = new Date(start)
  const days = Math.round((new Date(d).setHours(0, 0, 0, 0) - new Date(now).setHours(0, 0, 0, 0)) / 86_400_000)
  if (days <= 0) return `at ${time(start)}`
  if (days === 1) return `tomorrow at ${time(start)}`
  return `${d.toLocaleDateString([], { weekday: 'long' })} at ${time(start)}`
}
