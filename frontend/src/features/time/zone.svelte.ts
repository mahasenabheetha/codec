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
  // A Set: this machine's zone may be UTC itself.
  return [...new Set(['UTC', browserZone, ...all])]
})()

/** The zone name the server will know, or null: Intl accepts any case
 *  ("utc") and offsets ("+01:00"); Go wants a database name as listed. */
export function canonicalZone(z: string): string | null {
  try {
    const name = new Intl.DateTimeFormat('en', { timeZone: z.trim() }).resolvedOptions().timeZone
    if (name === 'UTC' || name === 'Etc/UTC') return 'UTC'
    if (/^[+-]\d/.test(name)) return null
    return zoneNames.length <= 2 || zoneNames.includes(name) ? name : null
  } catch {
    return null
  }
}

/** "Mon 5 Oct 2026, 02:00" in zone; in UTC if the browser doesn't know the zone. */
export function formatRun(iso: string, zone: string): { day: string; time: string } {
  const d = new Date(iso)
  const fmt = (o: Intl.DateTimeFormatOptions) => {
    try {
      return new Intl.DateTimeFormat('en-GB', { timeZone: zone, ...o }).format(d)
    } catch {
      return new Intl.DateTimeFormat('en-GB', { timeZone: 'UTC', ...o }).format(d)
    }
  }
  return {
    day: fmt({ weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' }),
    time: fmt({ hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZoneName: 'short' }),
  }
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

// A zone saved before names were checked ("utc") is repaired once.
{
  const { timestamp, cron } = stored.value
  const fixed = { timestamp: canonicalZone(timestamp) ?? browserZone, cron: canonicalZone(cron) ?? 'UTC' }
  if (fixed.timestamp !== timestamp || fixed.cron !== cron) stored.value = fixed
}
