// Find in Files: the query, its options and the last result. Kept in
// memory only; a new folder starts empty.

import { isAbort } from '../../lib/api/client'
import { searchFiles, type SearchMatch, type SearchResult } from '../../lib/api/search'

export interface FileHits {
  path: string
  matches: SearchMatch[]
}

class Search {
  query = $state('')
  matchCase = $state(false)
  wholeWord = $state(false)
  regex = $state(false)
  globs = $state('') // comma-separated, as typed

  result = $state.raw<SearchResult | null>(null)
  error = $state<string | null>(null)
  running = $state(false)
  collapsed = $state(new Set<string>())
  focusTick = $state(0) // bumped to focus the search box
  focusedTick = 0 // the last bump the panel acted on
  root: string | undefined // the folder the results belong to

  private inflight: AbortController | null = null
  private timer: ReturnType<typeof setTimeout> | undefined

  /** Matches grouped by file, in path order. */
  byFile = $derived.by(() => {
    const out: FileHits[] = []
    for (const m of this.result?.matches ?? []) {
      const last = out[out.length - 1]
      if (last?.path === m.path) last.matches.push(m)
      else out.push({ path: m.path, matches: [m] })
    }
    return out
  })

  /** Search after typing settles; at once when `now`. */
  schedule(now = false) {
    clearTimeout(this.timer)
    this.timer = setTimeout(() => this.run(), now ? 0 : 250)
  }

  async run() {
    clearTimeout(this.timer)
    this.inflight?.abort()
    if (!this.query.trim()) {
      this.result = null
      this.error = null
      this.running = false
      return
    }
    const ctrl = new AbortController()
    this.inflight = ctrl
    this.running = true
    try {
      const globs = this.globs.split(',').map((g) => g.trim()).filter(Boolean)
      const r = await searchFiles(
        { query: this.query, regex: this.regex, matchCase: this.matchCase, wholeWord: this.wholeWord, globs },
        ctrl.signal,
      )
      if (ctrl.signal.aborted) return
      this.result = r
      this.error = null
      this.collapsed = new Set()
    } catch (e) {
      if (isAbort(e)) return
      this.error = e instanceof Error ? e.message : String(e)
      this.result = null
    } finally {
      if (this.inflight === ctrl) this.running = false
    }
  }

  toggleFile(path: string) {
    const next = new Set(this.collapsed)
    if (next.has(path)) next.delete(path)
    else next.add(path)
    this.collapsed = next
  }

  reset() {
    this.inflight?.abort()
    this.query = ''
    this.result = null
    this.error = null
  }
}

export const finder = new Search()
