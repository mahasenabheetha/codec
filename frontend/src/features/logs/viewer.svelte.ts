// The Log viewer's state: per tab, the log being read, the query and the rows
// fetched so far. The log itself lives on the server under an id; the
// sources (files, pasted text, folder paths) stay here in memory only
// (decision 19), so a log the server dropped (idle, or codec restarted)
// is sent again without asking. Display choices are preferences.

import { getContext, setContext } from 'svelte'
import { SvelteMap } from 'svelte/reactivity'
import { ApiError, isAbort } from '../../lib/api/client'
import {
  closeLog,
  compareLogs,
  logEntries,
  logFields,
  openLogs,
  type EntriesPage,
  type Histogram,
  type LogDiff,
  type LogOpened,
  type LogQuery,
  type LogRow,
  type LogSource,
} from '../../lib/api/logs'
import type { ArgoRun } from '../../lib/api/argo'
import { persisted } from '../../lib/stores/persist.svelte'
import { toast } from '../../lib/stores/toast.svelte'

export const PAGE = 200
const BUCKETS = 120

export type TimeMode = 'local' | 'utc' | 'relative'
export const prefs = {
  time: persisted<TimeMode>('logs.time', 'local'),
  /** Long cells wrapped (the default), or kept on one line with the list scrolling sideways. */
  lines: persisted<'wrap' | 'scroll'>('logs.lines', 'wrap'),
  /** Field columns, kept by name across logs; only the ones a log has show. */
  columns: persisted<string[]>('logs.columns', []),
  /** Entries shown around each match, and how repeated lines fold. */
  context: persisted<number>('logs.context', 0),
  repeats: persisted<RepeatMode>('logs.repeats', 'exact'),
  /** Over time: entries by level, or a lane per step or source. */
  chart: persisted<'levels' | 'lanes'>('logs.chart', 'levels'),
}

export type RepeatMode = 'off' | 'exact' | 'similar'

/** A pinned entry: what its row showed, and a note of the user's. */
export interface Pin {
  i: number
  row: LogRow
  note: string
}
const MARKERS = 300

/** This tab's log compared with another tab's (Compare with…); memory
 *  only. versions are the two logs' when it was made: if either has
 *  changed since, it is stale. */
export interface Comparison {
  other: LogViewer
  name: string // the other tab's, kept for when it closes
  diff: LogDiff | null
  loading: boolean
  error: string | null
  versions: [number, number]
}

/** A log that mixes structured entries with plain output (a database's
 *  lines inside an Argo run) starts with the output folded. */
function mixed(s: LogOpened['summary']): boolean {
  const f = s.formats
  return (f.text ?? 0) > 0 && (f.json ?? 0) + (f.ecs ?? 0) + (f.logfmt ?? 0) + (f.klog ?? 0) > 0
}

export const sourceName = (s: LogSource) => (s.kind === 'file' ? s.file.name : s.kind === 'text' ? s.name : s.path.split('/').pop()!)

let nextKey = 0

/** One open log: a tab of the Log viewer. */
export class LogViewer {
  readonly key = ++nextKey
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
  /** Each field's most common values, for the query's suggestions. */
  fieldValues = $state.raw<Map<string, string[]>>(new Map())
  /** The query last sent, so typing doesn't send it twice. */
  requested = ''
  selected = $state<number | null>(null) // entry index

  /** How the matches are laid out (see internal/logview/view.go). */
  view = $state({ context: prefs.context.value, output: false, repeats: prefs.repeats.value })
  /** Folds opened, by their first entry. */
  expanded = $state.raw<Set<number>>(new Set())
  /** Up to four queries whose entries are marked, not filtered; memory only. */
  highlights = $state.raw<string[]>([])
  highlightErrors = $state.raw<string[]>([])
  /** A picture of the whole view, for beside the scrollbar. */
  markers = $state.raw<number[]>([])
  /** Per marker, the row a click on it goes to. */
  markerRows: number[] = []
  /** Rows in the view: matches, context, gaps and folds. */
  rowCount = $state(0)
  /** Entries pinned while investigating, in log order; memory only. */
  pins = $state.raw<Pin[]>([])
  /** Bumped when the tab's files change. */
  version = $state(0)
  /** An entry to show when the tab is next in front (from another tab). */
  want = $state<number | null>(null)
  compare = $state.raw<Comparison | null>(null)
  /** An Argo run's status, opened or pasted beside its log; memory only. */
  status = $state.raw<{ run: ArgoRun; name: string } | null>(null)

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
    // A tab with nothing read yet is named by what it is reading.
    if (!this.opened) this.sources = all
    try {
      const o = await openLogs(all, c.signal)
      if (c.signal.aborted) return
      const old = this.opened?.id
      this.sources = all
      this.opened = o
      this.version++
      this.fieldKeys = null
      this.selected = null
      this.expanded = new Set()
      // Pins point at entries by their place in the log, which changes
      // when files are added or removed.
      if (this.pins.length) {
        this.pins = []
        toast('Pins were cleared: the log changed')
      }
      if (!add) {
        this.resetQuery()
        this.view.output = mixed(o.summary)
      }
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

  /** Read the same sources again (the server no longer holds the log).
   *  Requests that find it gone together share one re-send, so a big log
   *  isn't uploaded and held once per page being fetched. */
  private reopening: Promise<void> | null = null
  private reopen(): Promise<void> {
    const sources = this.sources
    this.reopening ??= openLogs(sources)
      .then((o) => {
        if (this.sources === sources) this.opened = o // files changed meanwhile: keep the new log
        else closeLog(o.id).catch(() => {})
      })
      .finally(() => {
        this.reopening = null
      })
    return this.reopening
  }

  /** Run fn with the log's id, sending the log again once if it expired
   *  (a 410 that mine says is about this log, when fn uses two). */
  async withLog<T>(fn: (id: string) => Promise<T>, mine: (e: ApiError) => boolean = () => true): Promise<T> {
    const id = this.id
    if (!id) throw new Error('no log is open')
    try {
      return await fn(id)
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 410 && mine(e)) || !this.sources.length) throw e
      // Another request may have sent it again already.
      if (this.id === id) await this.reopen()
      return fn(this.id!)
    }
  }

  remove(i: number) {
    const rest = this.sources.filter((_, j) => j !== i)
    if (rest.length) this.open(rest)
    else logTabs.close(this)
  }

  /** The tab's name: its first source, and how many more. */
  get title(): string {
    if (!this.sources.length) return 'New log'
    const more = this.sources.length - 1
    return sourceName(this.sources[0]) + (more ? ` +${more}` : '')
  }

  /** Compare this run with another tab's: what is new, missing, more or
   *  less often, and slower. */
  async compareWith(other: LogViewer) {
    const versions: [number, number] = [this.version, other.version]
    const c: Comparison = { other, name: other.title, diff: null, loading: true, error: null, versions }
    this.compare = c
    const theirs = (e: ApiError) => !!e.body?.other
    try {
      const diff = await this.withLog(
        (id) => other.withLog((oid) => compareLogs(id, oid), theirs),
        (e) => !theirs(e),
      )
      if (this.compare === c) this.compare = { ...c, diff, loading: false }
    } catch (e) {
      if (this.compare === c) this.compare = { ...c, loading: false, error: e instanceof Error ? e.message : String(e) }
    }
  }

  /** The comparison no longer matches the logs (files added or removed). */
  get compareStale(): boolean {
    const c = this.compare
    return !!c && (c.versions[0] !== this.version || c.versions[1] !== c.other.version)
  }

  close() {
    this.ctrl?.abort()
    this.compare = null
    this.status = null
    if (this.id) closeLog(this.id).catch(() => {})
    this.sources = []
    this.opened = null
    this.error = null
    this.opening = false
    this.fieldKeys = null
    this.fieldValues = new Map()
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
    this.rowCount = 0
    this.markers = []
    this.markerRows = []
    this.histogram = null
  }

  /** What every page request carries besides where it starts. */
  private base(q: LogQuery = this.applied) {
    return {
      ...q,
      view: {
        context: this.view.context,
        output: this.view.output,
        repeats: this.view.repeats === 'off' ? ('' as const) : this.view.repeats,
        expanded: [...this.expanded],
      },
      columns: this.columns,
      highlights: this.highlights,
      pins: this.pins.map((p) => p.i),
    }
  }

  isPinned(i: number): boolean {
    return this.pins.some((p) => p.i === i)
  }

  /** Pin entry i (with what its row shows), or unpin it. */
  async togglePin(i: number, row?: LogRow) {
    if (this.isPinned(i)) {
      this.pins = this.pins.filter((p) => p.i !== i)
    } else {
      if (!row) row = (await this.withLog((id) => logEntries(id, { ...this.base(), at: i, limit: 1, view: {} }))).entries.find((r) => r.i === i)
      if (!row) return
      // A fold's row is pinned as its first entry.
      const r: LogRow = { ...row, kind: undefined, n: undefined, matches: undefined, lastI: undefined, lastTime: undefined, hl: undefined }
      this.pins = [...this.pins, { i, row: r, note: '' }].sort((a, b) => a.i - b.i)
    }
    this.refreshMarkers()
  }

  setNote(i: number, note: string) {
    this.pins = this.pins.map((p) => (p.i === i ? { ...p, note } : p))
  }

  clearPins() {
    this.pins = []
    this.refreshMarkers()
  }

  /** The pin before or after entry i, in log order (wrapping round). */
  pinNear(i: number | null, dir: 1 | -1): Pin | undefined {
    const ps = this.pins
    if (!ps.length) return undefined
    if (i === null) return dir > 0 ? ps[0] : ps[ps.length - 1]
    return dir > 0 ? (ps.find((p) => p.i > i) ?? ps[0]) : (ps.findLast((p) => p.i < i) ?? ps[ps.length - 1])
  }

  /** The marker strip again, after pins changed. */
  private refreshMarkers() {
    if (!this.id) return
    const gen = this.gen
    this.withLog((id) => logEntries(id, { ...this.base(), limit: 1, markers: MARKERS }))
      .then((p) => {
        if (gen === this.gen) {
          this.markers = p.markers ?? []
          this.markerRows = p.markerRows ?? []
        }
      })
      .catch(() => {})
  }

  /** Apply the query box (and the time range) and fetch the first page. */
  async run(q: LogQuery = { ...this.applied, q: this.input.trim() }) {
    if (!this.id) return
    const gen = ++this.gen
    this.requested = q.q
    this.loading = true
    try {
      const page = await this.withLog((id) => logEntries(id, { ...this.base(q), limit: PAGE, histogram: BUCKETS, markers: MARKERS }))
      if (gen !== this.gen) return
      this.applied = q
      this.queryError = null
      logTabs.remember(q.q)
      this.take(page, true)
      // The selected entry may have moved (a fold opened above it).
      if (this.selected !== null) {
        const sel = this.selected
        this.withLog((id) => logEntries(id, { ...this.base(), at: sel, limit: 1 }))
          .then((p) => {
            if (gen === this.gen && this.selected === sel) this.selectedPos = (p.atRow ?? -1) >= 0 ? p.atRow! : null
          })
          .catch(() => {})
      }
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
      this.rowCount = page.rows
      this.levelCounts = page.levels
      this.warnings = page.warnings ?? []
      this.markers = page.markers ?? []
      this.markerRows = page.markerRows ?? []
      this.highlightErrors = page.highlightErrors ?? []
      if (page.histogram) this.histogram = page.histogram
    }
    const p = Math.floor(page.offset / PAGE)
    this.asked.add(p)
    this.pages.set(p, page.entries)
  }

  /** The row at a position in the view, if its page has arrived. */
  rowAt(pos: number): LogRow | undefined {
    return this.pages.get(Math.floor(pos / PAGE))?.[pos % PAGE]
  }

  /** Fetch the pages rows first…last fall on (the list calls this as it scrolls). */
  ensure(first: number, last: number) {
    if (!this.id || this.rowCount === 0) return
    const gen = this.gen
    for (let p = Math.floor(first / PAGE); p <= Math.floor(Math.max(last - 1, 0) / PAGE); p++) {
      if (this.asked.has(p)) continue
      this.asked.add(p)
      this.withLog((id) => logEntries(id, { ...this.base(), offset: p * PAGE, limit: PAGE }))
        .then((page) => gen === this.gen && this.take(page, false))
        .catch((e) => {
          if (gen === this.gen && !isAbort(e)) {
            this.asked.delete(p)
            this.error = e instanceof Error ? e.message : String(e)
          }
        })
    }
  }

  /** The row that shows entry i (its own, or the fold holding it),
   *  fetching its page; when the query hides it (or a gap skips it), the
   *  query and range are cleared first. -1 if gone. */
  async locate(i: number): Promise<number> {
    const find = async () => {
      const page = await this.withLog((id) => logEntries(id, { ...this.base(), at: i, limit: PAGE }))
      const pos = page.atRow ?? -1
      const r = pos >= 0 ? page.entries[pos - page.offset] : undefined
      if (!r || r.kind === 'gap') return -1
      this.take(page, false)
      return pos
    }
    let pos = await find()
    if (pos < 0 && (this.applied.q || this.applied.from || this.applied.to)) {
      this.input = ''
      await this.run({ q: '' })
      pos = await find()
    }
    return pos
  }

  /** The view row that shows the selected entry, when known, and the
   *  last entry of that row (a fold holds several). */
  selectedPos = $state<number | null>(null)
  selectedLast: number | null = null

  select(i: number | null, pos: number | null = null, last?: number) {
    this.selected = i
    this.selectedPos = pos
    this.selectedLast = last ?? i
  }

  /** Change how the matches are laid out; the folds opened start over. */
  setView(v: Partial<{ context: number; output: boolean; repeats: RepeatMode }>) {
    Object.assign(this.view, v)
    if (v.context !== undefined) prefs.context.value = v.context
    if (v.repeats !== undefined) prefs.repeats.value = v.repeats
    this.expanded = new Set()
    this.run(this.applied)
  }

  /** Open a fold (show its entries) or fold them back. */
  toggleFold(i: number) {
    const e = new Set(this.expanded)
    if (e.has(i)) e.delete(i)
    else e.add(i)
    this.expanded = e
    this.run(this.applied)
  }

  setHighlights(rules: string[]) {
    this.highlights = rules.map((r) => r.trim()).filter(Boolean).slice(0, 4)
    this.run(this.applied)
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
    if (r || pos < 0 || pos >= this.rowCount) return r
    const p = Math.floor(pos / PAGE)
    const page = await this.withLog((id) => logEntries(id, { ...this.base(), offset: p * PAGE, limit: PAGE }))
    this.take(page, false)
    return this.rowAt(pos)
  }

  /** The position of the next (or previous) error or worse among the
   *  matches, from the selected entry; null when there is none. */
  async nextError(dir: 1 | -1): Promise<number | null> {
    // From the end of the selected row going down: a fold of errors is passed whole.
    const from = (dir > 0 ? this.selectedLast : this.selected) ?? (dir > 0 ? -1 : this.opened?.summary.entries ?? 0)
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
    const r = await this.withLog((id) => logFields(id, { q: '', top: 20 }))
    this.fieldKeys = r.fields.map((f) => f.key).filter((k) => k !== 'source')
    this.fieldValues = new Map(r.fields.filter((f) => !f.long).map((f) => [f.key, (f.top ?? []).map((t) => t.value)]))
  }

  toggleColumn(key: string) {
    const cols = prefs.columns.value
    prefs.columns.value = cols.includes(key) ? cols.filter((c) => c !== key) : [...cols, key]
    this.refetch()
  }
}

/** The open logs, one tab each, and the queries run in any of them. */
class LogTabs {
  list = $state.raw<LogViewer[]>([])
  current = $state.raw<LogViewer | null>(null)
  history = $state.raw<string[]>([]) // newest first; memory only (decision 19)

  /** Read sources in a tab of their own (the current tab when it holds
   *  nothing yet), or add them to the current log, merged by time. A
   *  folder file already open on its own just gets its tab back. */
  open(sources: LogSource[], add = false) {
    if (!sources.length) return
    const cur = this.current
    if (add && cur?.opened) {
      cur.open(sources, true)
      return
    }
    const one = sources.length === 1 && sources[0].kind === 'path' ? sources[0].path : null
    const same = one && this.list.find((v) => v.sources.length === 1 && v.sources[0].kind === 'path' && v.sources[0].path === one)
    if (same) {
      this.current = same
      return
    }
    let v = cur && !cur.opened && !cur.opening ? cur : null
    if (!v) {
      v = new LogViewer()
      this.list = [...this.list, v]
    }
    this.current = v
    v.open(sources)
  }

  select(v: LogViewer) {
    this.current = v
  }

  /** Close a tab and let the server go of its log. */
  close(v: LogViewer) {
    const i = this.list.indexOf(v)
    v.close()
    this.list = this.list.filter((x) => x !== v)
    if (this.current === v) this.current = this.list[Math.min(i, this.list.length - 1)] ?? null
  }

  remember(q: string) {
    if (q && this.history[0] !== q) this.history = [q, ...this.history.filter((h) => h !== q)].slice(0, 20)
  }
}

export const logTabs = new LogTabs()

// Each tab's components reach its LogViewer through Svelte's context, so
// the same components serve every tab.
const KEY = Symbol('log viewer')
export function provideViewer(v: LogViewer) {
  setContext(KEY, v)
}
export function useViewer(): LogViewer {
  return getContext<LogViewer>(KEY)
}
