// The Log viewer's state: the log being read, the query and the rows
// fetched so far. The log itself lives on the server under an id; the
// sources (files, pasted text, folder paths) stay here in memory only
// (decision 19), so a log the server dropped (idle, or codec restarted)
// is sent again without asking. Display choices are preferences.

import { SvelteMap } from 'svelte/reactivity'
import { ApiError, isAbort } from '../../lib/api/client'
import {
  closeLog,
  logEntries,
  logFields,
  openLogs,
  type EntriesPage,
  type Histogram,
  type LogOpened,
  type LogQuery,
  type LogRow,
  type LogSource,
} from '../../lib/api/logs'
import { persisted } from '../../lib/stores/persist.svelte'

export const PAGE = 200
const BUCKETS = 120

export type TimeMode = 'local' | 'utc' | 'relative'
export const prefs = {
  time: persisted<TimeMode>('logs.time', 'local'),
  wrap: persisted('logs.wrap', false),
  /** Field columns, kept by name across logs; only the ones a log has show. */
  columns: persisted<string[]>('logs.columns', []),
}

export const sourceName = (s: LogSource) => (s.kind === 'file' ? s.file.name : s.kind === 'text' ? s.name : s.path.split('/').pop()!)

class LogViewer {
  sources = $state.raw<LogSource[]>([])
  opened = $state.raw<LogOpened | null>(null)
  opening = $state(false)
  error = $state<string | null>(null)

  /** The query box, and the query the rows are for. */
  input = $state('')
  applied = $state<LogQuery>({ q: '' })
  queryError = $state<{ message: string; pos: number } | null>(null)
  warnings = $state.raw<string[]>([])
  total = $state(0)
  levelCounts = $state.raw<number[]>([])
  histogram = $state.raw<Histogram | null>(null)
  loading = $state(false)
  /** Rows by page; a page not here yet is drawn as a placeholder. */
  readonly pages = new SvelteMap<number, LogRow[]>()
  /** Field keys of the log, for the column picker (asked for once). */
  fieldKeys = $state.raw<string[] | null>(null)
  selected = $state<number | null>(null) // entry index
  history = $state.raw<string[]>([]) // queries run, newest first; memory only

  private gen = 0 // bumped per query, so late pages of an old one are dropped
  private asked = new Set<number>()
  private ctrl: AbortController | null = null

  get id() {
    return this.opened?.id
  }
  /** Columns to show: the chosen ones this log has. */
  get columns(): string[] {
    const keys = this.fieldKeys
    return keys ? prefs.columns.value.filter((c) => keys.includes(c)) : prefs.columns.value
  }

  /** Read sources: replacing what is open, or added to it (merged by time). */
  async open(sources: LogSource[], add = false) {
    if (!sources.length) return
    const all = add ? [...this.sources, ...sources] : sources
    this.ctrl?.abort()
    const c = new AbortController()
    this.ctrl = c
    this.opening = true
    this.error = null
    try {
      const o = await openLogs(all, c.signal)
      if (c.signal.aborted) return
      const old = this.opened?.id
      this.sources = all
      this.opened = o
      this.fieldKeys = null
      this.selected = null
      if (!add) this.resetQuery()
      if (old) closeLog(old).catch(() => {})
      this.loadFieldKeys().catch(() => {})
      await this.run()
    } catch (e) {
      if (isAbort(e) || c.signal.aborted) return
      this.error = e instanceof Error ? e.message : String(e)
    } finally {
      if (this.ctrl === c) this.opening = false
    }
  }

  /** Read the same sources again (the server no longer holds the log). */
  private async reopen() {
    const o = await openLogs(this.sources)
    this.opened = o
  }

  /** Run fn with the log's id, sending the log again once if it expired. */
  async withLog<T>(fn: (id: string) => Promise<T>): Promise<T> {
    if (!this.id) throw new Error('no log is open')
    try {
      return await fn(this.id)
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 410) || !this.sources.length) throw e
      await this.reopen()
      return fn(this.id!)
    }
  }

  remove(i: number) {
    const rest = this.sources.filter((_, j) => j !== i)
    if (rest.length) this.open(rest)
    else this.close()
  }

  close() {
    this.ctrl?.abort()
    if (this.id) closeLog(this.id).catch(() => {})
    this.sources = []
    this.opened = null
    this.error = null
    this.opening = false
    this.fieldKeys = null
    this.selected = null
    this.resetQuery()
  }

  private resetQuery() {
    this.input = ''
    this.applied = { q: '' }
    this.queryError = null
    this.warnings = []
    this.pages.clear()
    this.asked.clear()
    this.total = 0
    this.histogram = null
  }

  /** Apply the query box (and the time range) and fetch the first page. */
  async run(q: LogQuery = { ...this.applied, q: this.input.trim() }) {
    if (!this.id) return
    const gen = ++this.gen
    this.loading = true
    try {
      const page = await this.withLog((id) => logEntries(id, { ...q, limit: PAGE, columns: this.columns, histogram: BUCKETS }))
      if (gen !== this.gen) return
      this.applied = q
      this.queryError = null
      if (q.q && this.history[0] !== q.q) this.history = [q.q, ...this.history.filter((h) => h !== q.q)].slice(0, 20)
      this.take(page, true)
    } catch (e) {
      if (gen !== this.gen) return
      if (e instanceof ApiError && e.status === 422) {
        this.queryError = { message: e.message, pos: Number(e.body?.pos ?? -1) }
        return
      }
      this.error = e instanceof Error ? e.message : String(e)
    } finally {
      if (gen === this.gen) this.loading = false
    }
  }

  private take(page: EntriesPage, fresh: boolean) {
    if (fresh) {
      this.pages.clear()
      this.asked.clear()
      this.total = page.total
      this.levelCounts = page.levels
      this.warnings = page.warnings ?? []
      if (page.histogram) this.histogram = page.histogram
    }
    const p = Math.floor(page.offset / PAGE)
    this.asked.add(p)
    this.pages.set(p, page.entries)
  }

  /** The row at a position in the matches, if its page has arrived. */
  rowAt(pos: number): LogRow | undefined {
    return this.pages.get(Math.floor(pos / PAGE))?.[pos % PAGE]
  }

  /** Fetch the pages rows first…last fall on (the list calls this as it scrolls). */
  ensure(first: number, last: number) {
    if (!this.id || this.total === 0) return
    const gen = this.gen
    for (let p = Math.floor(first / PAGE); p <= Math.floor(Math.max(last - 1, 0) / PAGE); p++) {
      if (this.asked.has(p)) continue
      this.asked.add(p)
      this.withLog((id) => logEntries(id, { ...this.applied, offset: p * PAGE, limit: PAGE, columns: this.columns }))
        .then((page) => gen === this.gen && this.take(page, false))
        .catch((e) => {
          if (gen === this.gen && !isAbort(e)) {
            this.asked.delete(p)
            this.error = e instanceof Error ? e.message : String(e)
          }
        })
    }
  }

  /** Where entry i is among the matches, fetching its page; when the
   *  query hides it, the query and range are cleared first. -1 if gone. */
  async locate(i: number): Promise<number> {
    const find = async () => {
      const page = await this.withLog((id) => logEntries(id, { ...this.applied, at: i, limit: PAGE, columns: this.columns }))
      const j = page.entries.findIndex((r) => r.i === i)
      if (j >= 0) this.take(page, false)
      return j < 0 ? -1 : page.offset + j
    }
    let pos = await find()
    if (pos < 0 && (this.applied.q || this.applied.from || this.applied.to)) {
      this.input = ''
      await this.run({ q: '' })
      pos = await find()
    }
    return pos
  }

  /** Where the selected entry is among the matches, when known. */
  selectedPos = $state<number | null>(null)

  select(i: number | null, pos: number | null = null) {
    this.selected = i
    this.selectedPos = pos
  }

  /** Add a term to the query and run it ("only this", "hide this"). */
  addTerm(term: string) {
    this.input = this.input.trim() ? `${this.input.trim()} ${term}` : term
    this.run({ ...this.applied, q: this.input })
  }

  /** Show only entries with this value, whatever else was filtered:
   *  what happened to one request or trace. */
  follow(term: string) {
    this.input = term
    this.run({ q: term })
  }

  /** The row at a position, fetching its page if needed. */
  async rowAsync(pos: number): Promise<LogRow | undefined> {
    const r = this.rowAt(pos)
    if (r || pos < 0 || pos >= this.total) return r
    const p = Math.floor(pos / PAGE)
    const page = await this.withLog((id) => logEntries(id, { ...this.applied, offset: p * PAGE, limit: PAGE, columns: this.columns }))
    this.take(page, false)
    return this.rowAt(pos)
  }

  /** The position of the next (or previous) error or worse among the
   *  matches, from the selected entry; null when there is none. */
  async nextError(dir: 1 | -1): Promise<number | null> {
    const from = this.selected ?? (dir > 0 ? -1 : this.opened?.summary.entries ?? 0)
    const q = this.applied.q ? `(${this.applied.q}) level>=error` : 'level>=error'
    const req = { ...this.applied, q, limit: 1 }
    const hit = await this.withLog(async (id) => {
      if (dir > 0) return (await logEntries(id, { ...req, at: from + 1 })).entries[0]
      const k = (await logEntries(id, { ...req, at: from })).offset
      return k > 0 ? (await logEntries(id, { ...req, offset: k - 1 })).entries[0] : undefined
    })
    if (!hit || (dir > 0 ? hit.i <= from : hit.i >= from)) return null
    const pos = await this.locate(hit.i)
    return pos < 0 ? null : pos
  }

  /** Rows again for the same query (columns changed). */
  refetch() {
    this.run(this.applied)
  }

  /** Limit to a time range (from the chart), or clear it. */
  setRange(from?: string, to?: string) {
    this.run({ q: this.applied.q, from, to })
  }

  /** Set the query's minimum level, replacing any level term in it. */
  setLevel(min: string) {
    const rest = this.input.replace(/(^|\s)level(:|>=|>|<=|<)\S+/g, ' ').trim()
    this.input = min ? `level>=${min}${rest ? ' ' + rest : ''}` : rest
    this.run()
  }

  /** The level the query asks for at least, when it says so simply. */
  get minLevel(): string {
    return /(?:^|\s)level>=(\w+)/.exec(this.applied.q)?.[1] ?? ''
  }

  async loadFieldKeys() {
    if (this.fieldKeys || !this.id) return
    const r = await this.withLog((id) => logFields(id, { q: '', top: 1 }))
    this.fieldKeys = r.fields.map((f) => f.key).filter((k) => k !== 'source')
  }

  toggleColumn(key: string) {
    const cols = prefs.columns.value
    prefs.columns.value = cols.includes(key) ? cols.filter((c) => c !== key) : [...cols, key]
    this.refetch()
  }
}

export const viewer = new LogViewer()
