// Query terms the viewer writes for you: field:value with the value
// quoted when it needs to be (see internal/logview/query.go).

const bare = /^[\w.\-/:@+]+$/

/** key:value, or -key:value to hide it. */
export function term(key: string, value: string, hide = false): string {
  const v = value !== '' && bare.test(value) && !value.startsWith('/') ? value : `"${value.replace(/["\\]/g, (c) => '\\' + c)}"`
  return `${hide ? '-' : ''}${key}:${v}`
}

/** Field names a term can't name (spaces, quotes, colons). */
export const nameable = (key: string) => /^[\w.@$\-/]+$/.test(key)

/** The words and phrases a query searches for anywhere (not field
 *  terms, regexes or what it hides), lower-cased, for highlighting. */
export function textTerms(q: string): string[] {
  const out: string[] = []
  for (const m of q.matchAll(/(-?)"((?:[^"\\]|\\.)*)"|(\S+)/g)) {
    if (m[2] !== undefined) {
      if (!m[1] && m[2]) out.push(m[2].replace(/\\(.)/g, '$1').toLowerCase())
      continue
    }
    const w = m[3].replace(/^\(+|\)+$/g, '')
    if (!w || /^(AND|OR|NOT)$/.test(w) || w.startsWith('-') || w.startsWith('/') || /[:<>*]/.test(w)) continue
    out.push(w.toLowerCase())
  }
  return out
}

/** Text split into the parts that match terms and the rest. */
export function marks(text: string, terms: string[]): { t: string; hit: boolean }[] {
  terms = terms.filter(Boolean)
  if (!terms.length || !text) return [{ t: text, hit: false }]
  const lower = text.toLowerCase()
  const out: { t: string; hit: boolean }[] = []
  let at = 0
  while (at < text.length) {
    let best = -1
    let len = 0
    for (const term of terms) {
      const i = lower.indexOf(term, at)
      if (i >= 0 && (best < 0 || i < best || (i === best && term.length > len))) {
        best = i
        len = term.length
      }
    }
    if (best < 0) break
    if (best > at) out.push({ t: text.slice(at, best), hit: false })
    out.push({ t: text.slice(best, best + len), hit: true })
    at = best + len
  }
  if (at < text.length) out.push({ t: text.slice(at), hit: false })
  return out
}

/** Fields that identify one request or trace, worth following. */
export const followable = (key: string) => /(^|[._-])(trace|span|request|req|correlation|transaction|x-request)[._-]?id$|^traceid$|^requestid$/i.test(key)
