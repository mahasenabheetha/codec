// Presentation helpers for JWT time claims (seconds since the epoch).

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
  ['second', 1],
]

/** "in 3 hours" / "2 days ago" relative to now. */
export function relative(epochSeconds: number, now = Date.now()): string {
  const diff = epochSeconds - now / 1000
  for (const [unit, secs] of units) {
    if (Math.abs(diff) >= secs || unit === 'second') {
      return rtf.format(Math.round(diff / secs), unit)
    }
  }
  return ''
}

/** Local date-time, e.g. "26 Sep 2026, 14:05:09". */
export function absolute(epochSeconds: number): string {
  return new Date(epochSeconds * 1000).toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  })
}

export type Validity = 'valid' | 'expired' | 'not-yet' | 'no-expiry'

/** Where "now" falls relative to the exp / nbf claims. */
export function validity(exp: unknown, nbf: unknown, now = Date.now() / 1000): Validity {
  if (typeof nbf === 'number' && now < nbf) return 'not-yet'
  if (typeof exp !== 'number') return 'no-expiry'
  return now >= exp ? 'expired' : 'valid'
}
