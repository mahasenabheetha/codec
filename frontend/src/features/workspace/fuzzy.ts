// Fuzzy file matching for Go to file (Ctrl+P). Query characters must
// appear in order; matches in the file name, at word starts and in
// runs score higher, so "valdev" finds values-dev.yaml first.

export interface Match {
  path: string
  score: number
  positions: number[] // matched indexes into path, for highlighting
}

function isBoundary(s: string, i: number): boolean {
  if (i === 0) return true
  const prev = s[i - 1]
  if (prev === '/' || prev === '-' || prev === '_' || prev === '.' || prev === ' ') return true
  // camelCase: a capital after a lowercase letter
  return s[i] !== s[i].toLowerCase() && prev === prev.toLowerCase()
}

// In-order match of q (lowercased) inside s from `from`. With jump,
// each character prefers its next word-start occurrence (unless it
// extends a run), which is usually what the user meant.
function matchFrom(s: string, lower: string, q: string, from: number, jump = true): number[] | null {
  const pos: number[] = []
  let i = from
  for (const ch of q) {
    let hit = lower.indexOf(ch, i)
    if (hit < 0) return null
    // If the previous match was consecutive, keep the run going.
    if (jump && !(pos.length && hit === pos[pos.length - 1] + 1)) {
      for (let j = hit; j >= 0 && j < lower.length; j = lower.indexOf(ch, j + 1)) {
        if (isBoundary(s, j)) {
          hit = j
          break
        }
      }
    }
    pos.push(hit)
    i = hit + 1
  }
  return pos
}

// Boundary jumps can overshoot what later characters need; fall back
// to the plain leftmost match then.
function find(s: string, lower: string, q: string, from: number): number[] | null {
  return matchFrom(s, lower, q, from) ?? matchFrom(s, lower, q, from, false)
}

function score(s: string, pos: number[], nameStart: number): number {
  let sc = 0
  for (let k = 0; k < pos.length; k++) {
    sc += 1
    if (isBoundary(s, pos[k])) sc += 6
    if (k > 0 && pos[k] === pos[k - 1] + 1) sc += 4
    if (pos[k] >= nameStart) sc += 2
  }
  return sc - s.length * 0.02
}

export function fuzzy(query: string, path: string): Match | null {
  const q = query.toLowerCase().replace(/\s+/g, '')
  if (!q) return { path, score: 0, positions: [] }
  const lower = path.toLowerCase()
  const nameStart = path.lastIndexOf('/') + 1

  // Try the file name alone first (what people usually type), then the
  // whole path (for queries like "prod/values").
  const inName = q.includes('/') ? null : find(path, lower, q, nameStart)
  const pos = inName ?? find(path, lower, q, 0)
  if (!pos) return null
  return { path, score: score(path, pos, nameStart) + (inName ? 20 : 0), positions: pos }
}

/** The best `limit` matches, best first. */
export function rank(query: string, paths: string[], limit = 60): Match[] {
  const out: Match[] = []
  for (const p of paths) {
    const m = fuzzy(query, p)
    if (m) out.push(m)
  }
  out.sort((a, b) => b.score - a.score || a.path.length - b.path.length)
  return out.slice(0, limit)
}
