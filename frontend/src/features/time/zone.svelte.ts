// The time zone both Time tabs show times in (a UI preference), the zone
// list for picking one, and formatting in a zone.

import { persisted } from '../../lib/stores/persist.svelte'

/** This browser's zone, e.g. "Europe/Stockholm". */
export const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

const stored = persisted<{ timestamp: string; cron: string }>('timeZones', { timestamp: browserZone, cron: 'UTC' })

/** Per tab: timestamps default to this machine's zone, cron to UTC (as
 *  GitHub Actions and Azure Pipelines run schedules). */
export const zones = {
  get timestamp() {
    return stored.value.timestamp
  },
  set timestamp(z: string) {
    stored.value = { ...stored.value, timestamp: z }
  },
  get cron() {
    return stored.value.cron
  },
  set cron(z: string) {
    stored.value = { ...stored.value, cron: z }
  },
}

/** Every zone the browser knows, UTC and this machine's first. */
export const zoneNames: string[] = (() => {
  let all: string[] = []
  try {
    all = Intl.supportedValuesOf('timeZone')
  } catch {
    /* older engines: typing a name still works */
  }
  return ['UTC', browserZone, ...all.filter((z) => z !== 'UTC' && z !== browserZone)]
})()

export function validZone(z: string): boolean {
  try {
    new Intl.DateTimeFormat('en', { timeZone: z })
    return true
  } catch {
    return false
  }
}

/** "Mon 5 Oct 2026, 02:00" in zone. */
export function formatRun(iso: string, zone: string): { day: string; time: string } {
  const d = new Date(iso)
  const day = new Intl.DateTimeFormat('en-GB', { timeZone: zone, weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' }).format(d)
  const time = new Intl.DateTimeFormat('en-GB', { timeZone: zone, hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZoneName: 'short' }).format(d)
  return { day, time }
}

/** "in 3 hours 5 minutes", "2 days ago": two units at most. */
export function relative(iso: string, now = Date.now()): string {
  let ms = new Date(iso).getTime() - now
  const future = ms > 0
  ms = Math.abs(ms)
  if (ms < 60_000) return future ? 'in under a minute' : 'just now'
  const units: [number, string][] = [
    [86_400_000, 'day'],
    [3_600_000, 'hour'],
    [60_000, 'minute'],
  ]
  const parts: string[] = []
  for (const [size, name] of units) {
    if (ms >= size) {
      const n = Math.floor(ms / size)
      ms -= n * size
      parts.push(`${n} ${name}${n === 1 ? '' : 's'}`)
      if (parts.length === 2) break
    } else if (parts.length === 1) break
  }
  const s = parts.join(' ')
  return future ? `in ${s}` : `${s} ago`
}
